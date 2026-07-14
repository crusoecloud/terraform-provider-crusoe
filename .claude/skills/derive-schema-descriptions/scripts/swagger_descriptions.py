#!/usr/bin/env python3
"""Extract field descriptions from the pinned client-go swagger spec.

The Crusoe client-go module is generated (by Swagger Codegen) from the Crusoe
Cloud API's Swagger 2.0 spec and ships that spec at ``swagger/v1/swagger.json``
inside the module. Each property's ``description`` is the authoritative source
for Terraform schema descriptions (see CCX-2836). This script resolves the
swagger spec for whatever client-go version go.mod currently pins, then reports
the description for each property of a swagger definition -- flagging any
property that has no description so a human can write one rather than have the
model invent it.

Run from the repository root so ``go`` resolves the pinned version.

Modes
-----
  --type  <GoTypeName>   Resolve the swagger definition for a client-go Go type
                         (e.g. DiskV1, VpcNetwork), read the property names from
                         its generated model_*.go json tags, and print the
                         swagger description for each -- the primary mode.
  --def   <DefName>      Same, but address the swagger definition directly by its
                         spec key (e.g. VPCNetwork). Use when --type can't resolve.
  --all   [SUBSTR]       Dump descriptions for every provider resource's read
                         model (plus nested/request defs) listed in resources.json,
                         grouped by package. Optionally filter packages by SUBSTR.
                         Use this to refresh ALL resources in one run.
  --coverage [SUBSTR]    List definitions and their described/total counts.
                         Optionally filter definition names by SUBSTR.
  --list  [SUBSTR]       List definition names (optionally filtered by SUBSTR).

Add --json for machine-readable output (default is human-readable text).
"""

import argparse
import json
import os
import re
import subprocess
import sys
from pathlib import Path

CLIENT_MODULE = "github.com/crusoecloud/client-go"


def die(msg: str) -> "None":
    print(f"error: {msg}", file=sys.stderr)
    sys.exit(1)


def run(cmd: list) -> str:
    try:
        out = subprocess.run(cmd, capture_output=True, text=True, check=True)
    except (subprocess.CalledProcessError, FileNotFoundError) as exc:
        detail = getattr(exc, "stderr", "") or str(exc)
        die(f"command failed: {' '.join(cmd)}\n{detail}")
    return out.stdout.strip()


def locate_spec() -> Path:
    """Find swagger.json for the client-go version pinned in go.mod."""
    version = run(["go", "list", "-m", "-f", "{{.Version}}", CLIENT_MODULE])
    gomodcache = run(["go", "env", "GOMODCACHE"])
    spec = Path(gomodcache) / f"{CLIENT_MODULE}@{version}" / "swagger" / "v1" / "swagger.json"
    if not spec.is_file():
        # Module may not be downloaded yet; fetch it, then retry.
        run(["go", "mod", "download", CLIENT_MODULE])
    if not spec.is_file():
        die(f"swagger spec not found for {CLIENT_MODULE}@{version}: {spec}")
    print(f"# client-go {version}", file=sys.stderr)
    print(f"# spec: {spec}", file=sys.stderr)
    return spec


def model_dir(spec: Path) -> Path:
    return spec.parent  # swagger/v1 holds both swagger.json and model_*.go


def load_defs(spec: Path) -> dict:
    with spec.open() as fh:
        return json.load(fh).get("definitions", {})


def normalize(name: str) -> str:
    """Case/punctuation-insensitive key: Go type names lowercase acronyms
    (VpcNetwork) while swagger definition keys preserve them (VPCNetwork)."""
    return re.sub(r"[^a-z0-9]", "", name.lower())


def resolve_def_name(go_type: str, defs: dict) -> str:
    if go_type in defs:
        return go_type
    target = normalize(go_type)
    matches = [k for k in defs if normalize(k) == target]
    if len(matches) == 1:
        return matches[0]
    if not matches:
        near = sorted(k for k in defs if target in normalize(k) or normalize(k) in target)
        hint = f" Did you mean: {', '.join(near[:10])}?" if near else ""
        die(f"no swagger definition matches Go type '{go_type}'.{hint}")
    die(f"'{go_type}' is ambiguous across definitions: {', '.join(matches)}. Use --def.")


def find_model_file(go_type: str, models: Path) -> "Path | None":
    """Locate the generated model_*.go declaring `type <go_type> struct`."""
    decl = re.compile(rf"^type {re.escape(go_type)} struct \{{", re.MULTILINE)
    for path in sorted(models.glob("model_*.go")):
        if decl.search(path.read_text()):
            return path
    return None


# Field lines look like:  SubnetId   string   `json:"subnet_id,omitempty"`
# Capture the property name up to the first comma/quote, then tolerate any
# tag options (e.g. `,omitempty`) before the closing quote.
GO_FIELD = re.compile(r'^\s*(\w+)\s+[^\s`]+\s+`[^`]*json:"([^",]+)[^"]*"')


def parse_go_fields(model_file: Path, go_type: str) -> "list[tuple[str, str]]":
    """Return [(GoField, jsonProperty), ...] for the struct, in declaration order."""
    text = model_file.read_text()
    start = re.search(rf"^type {re.escape(go_type)} struct \{{", text, re.MULTILINE)
    if not start:
        return []
    body = text[start.end():]
    end = body.find("\n}")
    body = body[:end] if end != -1 else body
    fields = []
    for line in body.splitlines():
        m = GO_FIELD.match(line)
        if m and m.group(2) != "-":
            fields.append((m.group(1), m.group(2)))
    return fields


def describe(defs: dict, def_name: str, properties: "list[str]") -> "list[dict]":
    props = defs.get(def_name, {}).get("properties", {})
    rows = []
    for prop in properties:
        desc = props.get(prop, {}).get("description")
        rows.append({"property": prop, "description": desc, "missing": not desc})
    return rows


def emit(rows: "list[dict]", def_name: str, go_type: "str | None", as_json: bool) -> None:
    described = sum(1 for r in rows if not r["missing"])
    if as_json:
        print(json.dumps({
            "definition": def_name,
            "go_type": go_type,
            "described": described,
            "total": len(rows),
            "properties": rows,
        }, indent=2))
        return
    header = f"definition {def_name}"
    if go_type and go_type != def_name:
        header += f" (Go type {go_type})"
    print(f"{header} — {described}/{len(rows)} described\n")
    for r in rows:
        if r["missing"]:
            print(f"  ⚠ MISSING  {r['property']}")
        else:
            print(f"  ✓ {r['property']}: {r['description']}")
    missing = [r["property"] for r in rows if r["missing"]]
    if missing:
        print(f"\n  {len(missing)} property(ies) have NO swagger description — do NOT invent; "
              f"flag for a human: {', '.join(missing)}")


def cmd_type(args, defs, spec) -> None:
    def_name = resolve_def_name(args.name, defs)
    model_file = find_model_file(args.name, model_dir(spec))
    if model_file:
        properties = [prop for _, prop in parse_go_fields(model_file, args.name)]
        print(f"# model: {model_file.name}", file=sys.stderr)
    else:
        # Fall back to the spec's own property list for the definition.
        properties = list(defs.get(def_name, {}).get("properties", {}).keys())
        print(f"# no model_*.go for '{args.name}'; using spec property list", file=sys.stderr)
    emit(describe(defs, def_name, properties), def_name, args.name, args.json)


def cmd_def(args, defs, _spec) -> None:
    if args.name not in defs:
        die(f"no swagger definition named '{args.name}'. Try --list.")
    properties = list(defs[args.name].get("properties", {}).keys())
    emit(describe(defs, args.name, properties), args.name, None, args.json)


def try_resolve_def_name(go_type: str, defs: dict) -> "str | None":
    """Like resolve_def_name but returns None instead of exiting, for --all."""
    if go_type in defs:
        return go_type
    matches = [k for k in defs if normalize(k) == normalize(go_type)]
    return matches[0] if len(matches) == 1 else None


def load_manifest() -> dict:
    """Load the resource->read-model manifest that lives beside this script."""
    path = Path(__file__).with_name("resources.json")
    if not path.is_file():
        die(f"resource manifest not found: {path}")
    with path.open() as fh:
        return json.load(fh).get("resources", {})


def cmd_all(args, defs, spec) -> None:
    """Dump descriptions for every read model (and extra defs) in resources.json."""
    manifest = load_manifest()
    models = model_dir(spec)
    results = []
    for pkg in sorted(manifest):
        if args.name and args.name not in pkg:
            continue
        entry = manifest[pkg]
        for go_type in [entry["read_model"], *entry.get("extra", [])]:
            def_name = try_resolve_def_name(go_type, defs)
            if def_name is None:
                print(f"# WARN {pkg}: no swagger definition for Go type '{go_type}' "
                      f"(stale manifest?)", file=sys.stderr)
                continue
            model_file = find_model_file(go_type, models)
            if model_file:
                properties = [prop for _, prop in parse_go_fields(model_file, go_type)]
            else:
                properties = list(defs.get(def_name, {}).get("properties", {}).keys())
            results.append({"package": pkg, "go_type": go_type, "definition": def_name,
                            "rows": describe(defs, def_name, properties)})

    if args.json:
        print(json.dumps([{
            "package": r["package"], "definition": r["definition"], "go_type": r["go_type"],
            "described": sum(1 for x in r["rows"] if not x["missing"]),
            "total": len(r["rows"]), "properties": r["rows"],
        } for r in results], indent=2))
        return

    current_pkg = None
    for r in results:
        if r["package"] != current_pkg:
            current_pkg = r["package"]
            print(f"\n=== {current_pkg} ===")
        emit(r["rows"], r["definition"], r["go_type"], as_json=False)

    print("\n=== summary (described/total per definition) ===")
    for r in results:
        described = sum(1 for x in r["rows"] if not x["missing"])
        total = len(r["rows"])
        flag = "" if described == total else "  (partial)" if described else "  (NONE)"
        print(f"  {r['package']}: {r['definition']} {described}/{total}{flag}")


def cmd_coverage(args, defs, _spec) -> None:
    rows = []
    for name in sorted(defs):
        props = defs[name].get("properties", {})
        if args.name and args.name.lower() not in name.lower():
            continue
        total = len(props)
        described = sum(1 for p in props.values() if isinstance(p, dict) and p.get("description"))
        rows.append({"definition": name, "described": described, "total": total})
    if args.json:
        print(json.dumps(rows, indent=2))
        return
    for r in rows:
        flag = "" if r["described"] == r["total"] else "  (partial)" if r["described"] else "  (NONE)"
        print(f"  {r['definition']}: {r['described']}/{r['total']}{flag}")


def cmd_list(args, defs, _spec) -> None:
    names = sorted(k for k in defs if not args.name or args.name.lower() in k.lower())
    if args.json:
        print(json.dumps(names, indent=2))
        return
    for n in names:
        print(n)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("--type", dest="mode_type", metavar="GoType", help="client-go Go type name")
    group.add_argument("--def", dest="mode_def", metavar="DefName", help="swagger definition key")
    group.add_argument("--all", dest="mode_all", nargs="?", const="", metavar="SUBSTR",
                       help="dump every resource read model in resources.json (optionally filter packages by SUBSTR)")
    group.add_argument("--coverage", dest="mode_cov", nargs="?", const="", metavar="SUBSTR",
                       help="described/total counts per definition")
    group.add_argument("--list", dest="mode_list", nargs="?", const="", metavar="SUBSTR",
                       help="list definition names")
    parser.add_argument("--json", action="store_true", help="machine-readable output")
    args = parser.parse_args()

    spec = locate_spec()
    defs = load_defs(spec)

    if args.mode_type is not None:
        args.name = args.mode_type
        cmd_type(args, defs, spec)
    elif args.mode_def is not None:
        args.name = args.mode_def
        cmd_def(args, defs, spec)
    elif args.mode_all is not None:
        args.name = args.mode_all
        cmd_all(args, defs, spec)
    elif args.mode_cov is not None:
        args.name = args.mode_cov
        cmd_coverage(args, defs, spec)
    else:
        args.name = args.mode_list
        cmd_list(args, defs, spec)


if __name__ == "__main__":
    main()

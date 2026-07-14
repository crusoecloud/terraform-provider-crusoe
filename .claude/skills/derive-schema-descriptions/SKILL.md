---
name: derive-schema-descriptions
description: Derive Terraform schema attribute descriptions from the pinned client-go swagger spec. Use after bumping github.com/crusoecloud/client-go in go.mod, or when asked to sync/update/refresh a resource's schema descriptions from the go-client / swagger / OpenAPI docs. Pulls verbatim descriptions from the spec and flags any attribute the spec does not describe instead of inventing text.
---

# Derive schema descriptions from client-go swagger

The Crusoe `client-go` module is generated (by Swagger Codegen) from the Crusoe
Cloud API's Swagger 2.0 spec and ships that spec at `swagger/v1/swagger.json`
inside the module, plus per-field description comments in the generated
`model_*.go` files. **This is the authoritative source for Terraform schema
attribute descriptions** (CCX-2836).

Use this skill to (re)derive a resource's `MarkdownDescription`/`Description`
strings from the spec — typically after `github.com/crusoecloud/client-go` is
bumped in `go.mod`, since a bump can add descriptions that were previously absent.

## Hard rules

1. **Never invent a description.** Every description written by this skill must
   trace to spec text for the corresponding property. Only mechanical style
   normalization (below) is permitted on top of the spec text.
2. **Flag, don't fill.** If the spec has no description for a mapped property (or
   the attribute has no spec property at all), do not write an `apiDesc*` string
   for it: keep any existing provider-authored text as a `providerDesc*` constant
   (or leave it undescribed) and list the attribute in the report as needing a
   human. This skill only authors `apiDesc*` (spec-derived) text.
3. **Flag in the report, never as a code comment.** The flag from rule 2 lives in
   the report to the user (see [Report](#report)) — **never** as an inline comment
   in the schema or `util.go`. Do not add comments like
   `// public_ipv4 intentionally left without a description: the spec has none` or
   `// ib_partition_id has no swagger description; left undescribed`. A missing
   `MarkdownDescription`/`Description` (or provider-authored `providerDesc*` text)
   is self-explanatory; such comments merely restate the visible absence, drift out
   of date, and get removed.
4. The spec version is whatever `go.mod` pins — never hardcode it. The helper
   script resolves it dynamically.

## Tooling

`scripts/swagger_descriptions.py` resolves the spec for the pinned client-go
version and reports descriptions. **Run from the repository root.**

```bash
python3 .claude/skills/derive-schema-descriptions/scripts/swagger_descriptions.py --type <GoType>   # by client-go Go type (primary)
python3 .claude/skills/derive-schema-descriptions/scripts/swagger_descriptions.py --def  <DefName>   # by swagger definition key (fallback)
python3 .claude/skills/derive-schema-descriptions/scripts/swagger_descriptions.py --all [SUBSTR]      # EVERY resource's read model at once (from resources.json)
python3 .claude/skills/derive-schema-descriptions/scripts/swagger_descriptions.py --coverage [SUBSTR] # described/total per definition
python3 .claude/skills/derive-schema-descriptions/scripts/swagger_descriptions.py --list [SUBSTR]     # list definition names
```

Add `--json` for machine-readable output. `--type` handles the Go↔spec name
casing gap automatically (Go `VpcNetwork` ↔ spec `VPCNetwork`, `IBNetwork`, etc.)
and marks properties the spec does not describe as `⚠ MISSING`.

## Workflow

**To refresh ALL resources at once** (the usual case, e.g. after a client-go bump):
run `--all`. It dumps the spec descriptions for every resource's read model — and
the nested/request defs its descriptions draw from — listed in
`scripts/resources.json`, grouped by package, with a described/total summary.
Diff that against the `apiDesc*` constants in each package and update the ones
whose spec text changed. Add a line to `resources.json` when a new resource
package is introduced (that is the one manual step keeping `--all` complete).

**To work a single package:** if the user names one (e.g. "disk"), start there.
Otherwise run `--all` (or `--coverage`) first, present which packages changed, and
confirm scope before editing.

### 1. Identify the API model the resource maps from

Each package maps state from one primary swagger read model. Find it:

```bash
grep -rhoE "swagger\.[A-Z][A-Za-z0-9_]+" internal/<pkg>/*.go | sort -u
```

Pick the **read** model (the type returned by `Get*`/`List*` and consumed by the
`*ToTerraform*Model` function), e.g. `DiskV1`, `VpcNetwork`, `InstanceTemplate` —
**not** the `*PostRequest`/`*PatchRequest` bodies. Some attributes are only
described on a request body; consult those with `--def <Body>` when the read
model leaves a field `⚠ MISSING`.

### 2. Pull descriptions

```bash
python3 .claude/skills/derive-schema-descriptions/scripts/swagger_descriptions.py --type <ReadModel>
```

### 3. Map Terraform attributes → swagger properties

The Terraform attribute name is **not** always the spec property name. Establish
each mapping from the code, not by guessing:

- The model struct's `tfsdk:"..."` tag is the Terraform attribute name.
- The `*ToTerraform*Model` mapping function connects a model field to a swagger Go
  field, e.g. `model.Subnet = template.SubnetId` → TF `subnet` ↔ Go `SubnetId`.
- The Go field's `json:"..."` tag (in `model_*.go`) is the spec property name,
  e.g. `SubnetId` → `subnet_id`. The script already reports by spec property name.

Known-style mismatches you will encounter: `subnet`↔`subnet_id`, `image`↔`image_name`,
`ssh_key`↔`ssh_public_key`, `ib_partition`↔`ib_partition_id`. Always verify against
the actual mapping function — do not rely on this list.

Some Terraform attributes are **provider-side only** (no spec property): e.g.
`project_id` inference behavior, deprecated compatibility fields. These have no
spec description — they belong in `providerDesc*` constants (see step 4), not
`apiDesc*`. Note them in the report (they are not failures).

### 4. Update the description source (`apiDesc*` / `providerDesc*` convention)

Descriptions live in each package's `util.go`, split into two origin-denoting
constant blocks that **both** the resource and data source schemas reference.
Every attribute's description is a named, origin-prefixed constant — no inline
strings. Reference implementation: `internal/disk`.

- **`apiDesc*`** — derived verbatim from the spec (this skill's territory; only
  mechanical style normalization on top of the spec text):

  ```go
  // apiDesc* — schema descriptions derived from the client-go swagger spec (<ReadModel>).
  const (
      apiDescID   = "ID of the disk."
      apiDescName = "Name of the disk."
  )
  ```

- **`providerDesc*`** — provider-specific text with no spec basis (Terraform
  behavior, deprecation notes, `project_id` inference, `common.DevelopmentMessage`,
  etc.). **This skill does not author these** — it preserves/relocates existing
  ones and flags gaps:

  ```go
  // providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
  const (
      providerDescProjectID = "ID of the project the disk belongs to. " + project.ProviderDescProjectIDFallback
  )
  ```

Wiring the schema:

- Pure spec attribute → `MarkdownDescription: apiDescX`.
- **Mixed origin** (spec base + a provider-specific addition) → compose in the
  schema, keeping the two constants separate so each fragment's origin stays
  explicit: `MarkdownDescription: apiDescX + " " + providerDescX`.
- **`project_id`** is provider-side: `providerDescProjectID = "<spec text if the
  read model has a `project_id` property, else the house phrase \"ID of the project
  the <resource> belongs to.\"> " + project.ProviderDescProjectIDFallback`. The
  shared inference suffix lives once as `project.ProviderDescProjectIDFallback` in
  `internal/project` — reference it, don't duplicate it.

Only `apiDesc*` constants are this skill's output — update them from the spec.
Never rewrite `providerDesc*` text from the spec. If a spec description newly
becomes available for an attribute currently described only by provider text, add
the spec text as the `apiDesc*` base and compose (`apiDescX + " " + providerDescX`)
rather than overwriting the provider remainder.

### 5. Style normalization (mechanical only)

Spec wording does not follow the repo's house style. Apply only these transforms,
each of which preserves meaning without adding information:

- Collapse embedded newlines / repeated whitespace to single spaces.
- Strip a leading article: `The name of…` → `Name of…`.
- Ensure exactly one trailing period.
- Wrap literal values, examples, and identifiers in backticks.
- Convert an inline value list already present in the text to the house pattern,
  reusing **only** the literals the spec states:
  `Type of the disk: persistent-ssd or shared-volume.`
  → `Type of the disk. Possible values: `persistent-ssd`, `shared-volume`.`

If a transform would require adding or changing a fact, keep the spec text verbatim
and note it. Never introduce constraints, defaults, or values not in the spec.

### 6. Verify

```bash
make build        # compiles; catches renamed/removed constants
make test         # unit tests, incl. any schema/description consistency tests
make docs         # regenerate docs/ from the updated schema descriptions
```

`make docs` fails if generated docs drift — commit the regenerated `docs/*`.

## Report

Flags are reported here, to the user — not written into the code as comments (see
[Hard rules](#hard-rules) 2–3). After editing, summarize for the user:

- **Updated (`apiDesc*`)** — attribute: old description → new spec-derived description.
- **Unchanged** — already matched the spec.
- **Flagged — no spec description** — the spec property exists but has no
  `description`; left undescribed or kept as `providerDesc*` text. Needs a human.
- **Flagged — provider-specific** — description carries meaning beyond the spec
  (deprecation, inference, Terraform behavior); kept as `providerDesc*`, not
  overwritten from the spec.
- **Flagged — differs greatly** — provider wording diverges substantially from the
  spec (not just style); surfaced for a human decision rather than auto-synced.
- **Provider-side only** — Terraform attributes with no spec property (expected).

State the resolved client-go version and definition(s) used.

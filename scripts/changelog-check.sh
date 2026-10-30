#!/usr/bin/env bash
# Checks that a merge request carries a valid changelog fragment in .changelog/.
#
# Usage: scripts/changelog-check.sh [file]
#   In a merge request pipeline the file defaults to .changelog/<MR IID>.txt.
#   Exits 0 without checking when the MR has the label "skip-changelog" or
#   "dependencies", or comes from a renovate/* branch.
#
# Besides the block structure and the type, the script checks the product tag
# for Crusoe Updates, "<!-- site: <category>/<subcategory> -->" (see
# .changelog/README.md). Entries of the types in SITE_TYPES must carry one, or
# "<!-- site: none -->". Tag values are checked against the site's live
# changelogConfig.json in the docs repo. When that file cannot be read the value
# is reported as unchecked and the script still passes: the docs side validates
# again before anything is published, so this check is a convenience for the
# author, not the gate. An entry with a product tag must also carry the title
# of its site entry, "<!-- title: ... -->", at most TITLE_MAX characters.
#
#   scripts/changelog-check.sh --tags   prints the valid product tags and exits
set -euo pipefail

TYPES="feature|enhancement|bug|security|deprecation|breaking-change|note"
SITE_TYPES="feature|security|breaking-change"
TITLE_MAX=70
SKIP_LABELS="skip-changelog|dependencies"
# A merge request that only touches these needs neither a fragment nor a
# label: no customer can notice them. Keep the list short; anything else
# needs an explicit decision.
SAFE_PATHS=("docs/*" "examples/*" "templates/*" "*.md" ".gitlab-ci.yml" ".gitlab/*" ".github/*" ".claude/*" ".gitignore" ".golangci.yml" "mise.toml" "versions.env" "*_test.go" "*.tftest.hcl" "testdata/*" "*/testdata/*")
DOCS_PROJECT="crusoeenergy/island/external/docs"
DOCS_CONFIG="src/data/changelogConfig.json"

# The valid values are the keys of platforms.<category>.subcategories in the
# docs repo's changelogConfig.json. Read live, never copied into this repo.
site_values() {
  local path="projects/${DOCS_PROJECT//\//%2F}/repository/files/${DOCS_CONFIG//\//%2F}/raw?ref=main"
  local json
  if [[ -n "${CI_JOB_TOKEN:-}" && -n "${CI_API_V4_URL:-}" ]]; then
    json=$(curl -fsS --max-time 20 -H "JOB-TOKEN: ${CI_JOB_TOKEN}" "${CI_API_V4_URL}/${path}") || return 1
  elif command -v glab >/dev/null 2>&1; then
    json=$(glab api "$path" 2>/dev/null) || return 1
  else
    return 1
  fi
  command -v jq >/dev/null 2>&1 || return 1
  jq -r '.platforms | to_entries[] | .key as $c | .value.subcategories | keys_unsorted[] | "\($c)/\(.)"' <<<"$json"
}

file="${1:-}"
explicit_file="$file"
list_tags=false
[[ "$file" == "--tags" ]] && { list_tags=true; file=""; explicit_file=""; }
if $list_tags; then
  if values=$(site_values) && [[ -n "$values" ]]; then
    printf '%s\n' "$values"
    exit 0
  fi
  echo "changelog: could not read ${DOCS_CONFIG} from ${DOCS_PROJECT} (glab or CI_JOB_TOKEN needed)" >&2
  exit 1
fi
if [[ -z "$file" ]]; then
  if [[ -z "${CI_MERGE_REQUEST_IID:-}" ]]; then
    echo "usage: $0 .changelog/<MR>.txt   (or run inside a merge request pipeline)" >&2
    exit 2
  fi
  file=".changelog/${CI_MERGE_REQUEST_IID}.txt"
fi

if [[ ",${CI_MERGE_REQUEST_LABELS:-}," =~ ,(${SKIP_LABELS}), ]]; then
  echo "changelog: skipped, merge request is labelled ${BASH_REMATCH[1]}"
  exit 0
fi
case "${CI_MERGE_REQUEST_SOURCE_BRANCH_NAME:-}" in
  renovate/*)
    echo "changelog: skipped, Renovate branch"
    exit 0
    ;;
esac

# Files the merge request changes, from its merge base. Fails when git cannot
# tell (no base SHA, base not fetchable), and the fragment is then required.
changed_files() {
  local base="${CI_MERGE_REQUEST_DIFF_BASE_SHA:-}"
  [[ -n "$base" ]] || return 1
  git cat-file -e "${base}^{commit}" 2>/dev/null || git fetch -q --depth=1 origin "$base" 2>/dev/null || return 1
  git diff --name-only "$base" "${CI_COMMIT_SHA:-HEAD}"
}

is_safe() {
  local f="$1" p
  for p in "${SAFE_PATHS[@]}"; do
    # shellcheck disable=SC2053  # the right-hand side is a glob on purpose
    [[ "$f" == $p ]] && return 0
  done
  return 1
}

if [[ -z "$explicit_file" && -n "${CI_MERGE_REQUEST_IID:-}" ]]; then
  if files=$(changed_files) && [[ -n "$files" ]]; then
    unsafe=""
    while read -r f; do
      [[ -z "$f" ]] && continue
      is_safe "$f" || unsafe+="${f}"$'\n'
    done <<<"$files"
    if [[ -z "$unsafe" ]]; then
      echo "changelog: skipped, the merge request only changes files no customer can notice:"
      awk '{ print "  " $0 }' <<<"$files"
      exit 0
    fi
  else
    echo "changelog: could not list the merge request's files; a fragment or the skip-changelog label is required" >&2
  fi
fi

if [[ ! -f "$file" ]]; then
  cat >&2 <<MSG
changelog: ${file} is missing

Every merge request needs a changelog fragment, or the label "skip-changelog"
when nothing is customer-visible. Example ${file}:

\`\`\`release-note:enhancement
\`crusoe_vpc_firewall_rule\` takes \`source_port_ranges\` and \`destination_port_ranges\`, lists of single ports, port ranges or \`*\`.
<!-- site: infrastructure-cloud/networking -->
<!-- title: Set port ranges on firewall rules in Terraform -->
\`\`\`

Types: ${TYPES//|/, }. See .changelog/README.md. In Claude Code, /changelog
drafts the fragment from the branch diff.
MSG
  exit 1
fi

# Structure and types. Prints one "<type><TAB><site tag><TAB><title>" line per entry.
entries=$(awk -v types="$TYPES" '
  BEGIN {
    n = split(types, list, "|")
    for (i = 1; i <= n; i++) valid[list[i]] = 1
    inblock = 0; blocks = 0; bad = 0
  }
  /^```release-note:/ {
    if (inblock) { print "changelog: nested release-note block at line " NR > "/dev/stderr"; bad = 1 }
    typ = substr($0, 17)
    gsub(/[ \t\r]+$/, "", typ)
    if (!(typ in valid)) {
      print "changelog: unknown type \"" typ "\" at line " NR " (valid: " types ")" > "/dev/stderr"
      bad = 1
    }
    inblock = 1; body = 0; site = ""; tags = 0; title = ""; titles = 0; blocks++
    next
  }
  /^```/ && inblock {
    if (!body) { print "changelog: empty entry ending at line " NR > "/dev/stderr"; bad = 1 }
    if (tags > 1) { print "changelog: more than one site tag in the entry ending at line " NR > "/dev/stderr"; bad = 1 }
    if (titles > 1) { print "changelog: more than one title in the entry ending at line " NR > "/dev/stderr"; bad = 1 }
    print typ "\t" site "\t" title
    inblock = 0
    next
  }
  inblock && /^<!-- *site: *[^ ].* *-->[ \t\r]*$/ {
    tags++
    site = $0
    sub(/^<!-- *site: */, "", site)
    sub(/ *-->[ \t\r]*$/, "", site)
    next
  }
  inblock && /^<!-- *title: *[^ ].* *-->[ \t\r]*$/ {
    titles++
    title = $0
    sub(/^<!-- *title: */, "", title)
    sub(/ *-->[ \t\r]*$/, "", title)
    next
  }
  inblock && NF > 0 { body = 1 }
  END {
    if (inblock) { print "changelog: release-note block never closed" > "/dev/stderr"; bad = 1 }
    if (blocks == 0) { print "changelog: no ```release-note:<type> block found" > "/dev/stderr"; bad = 1 }
    exit bad
  }
' "$file") || exit 1

# Product tags. Presence means "draft a Crusoe Updates entry"; SITE_TYPES must
# decide explicitly, the other types opt in by adding a tag. A tagged entry
# also needs the title the site will show.
shape='^[a-z0-9-]+/[a-z0-9-]+$'
bad=0
lookup=""
while IFS=$'\t' read -r typ site title; do
  [[ -z "$typ" ]] && continue
  if [[ -n "$title" && ${#title} -gt $TITLE_MAX ]]; then
    echo "changelog: title is ${#title} characters, the site shows at most ${TITLE_MAX}: \"${title}\"" >&2
    bad=1
  fi
  if [[ -z "$site" ]]; then
    if [[ "$typ" =~ ^(${SITE_TYPES})$ ]]; then
      cat >&2 <<MSG
changelog: a "${typ}" entry needs a product tag for Crusoe Updates, or an explicit opt-out:

  <!-- site: infrastructure-cloud/networking -->
  <!-- site: none -->

See .changelog/README.md, "Product tag for Crusoe Updates".
MSG
      bad=1
    fi
    continue
  fi
  [[ "$site" == "none" ]] && continue
  if [[ -z "$title" ]]; then
    cat >&2 <<MSG
changelog: an entry with a product tag needs the title of its Crusoe Updates entry:

  <!-- title: Manage external load balancers with the Terraform provider -->

One line, at most ${TITLE_MAX} characters, no trailing period. See .changelog/README.md, "Product tag and title for Crusoe Updates", or run /changelog in Claude Code.
MSG
    bad=1
  fi
  if [[ ! "$site" =~ $shape ]]; then
    echo "changelog: product tag \"${site}\" is not of the form <category>/<subcategory>" >&2
    bad=1
    continue
  fi
  lookup+="${site}"$'\n'
done <<<"$entries"
(( bad )) && exit 1


if [[ -n "$lookup" ]]; then
  if values=$(site_values) && [[ -n "$values" ]]; then
    while read -r site; do
      [[ -z "$site" ]] && continue
      if ! grep -qxF "$site" <<<"$values"; then
        echo "changelog: unknown product tag \"${site}\". Valid values, from ${DOCS_PROJECT}:${DOCS_CONFIG}:" >&2
        awk '{ print "  " $0 }' <<<"$values" >&2
        bad=1
      fi
    done <<<"$lookup"
    (( bad )) && exit 1
  else
    echo "changelog: warning, could not read ${DOCS_CONFIG} from ${DOCS_PROJECT}; product tag not validated: $(awk 'NF' <<<"$lookup" | paste -sd ' ' -)" >&2
  fi
fi

summary=$(awk -F'\t' '$2 != "" { print $2 }' <<<"$entries" | paste -sd, -)
titles=$(awk -F'\t' '$3 != "" { print "\"" $3 "\"" }' <<<"$entries" | paste -sd, -)
echo "changelog: ok, ${file}${summary:+ (product tag: ${summary}${titles:+; title: ${titles}})}"

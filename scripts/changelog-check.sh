#!/usr/bin/env bash
# Checks that a merge request carries a valid changelog fragment in .changelog/.
#
# Usage: scripts/changelog-check.sh [file]
#   In a merge request pipeline the file defaults to .changelog/<MR IID>.txt.
#   Exits 0 without checking when the MR has the label "dependencies" or comes
#   from a renovate/* branch, or has the label "skip-changelog" and says why in
#   its description (a line "No changelog: <why>").
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

# The merge request description, into MR_DESCRIPTION. CI truncates the
# variable at 2700 characters; the full text then comes from the API (job
# token), and MR_DESCRIPTION_PARTIAL says when that did not happen.
MR_DESCRIPTION=""
MR_DESCRIPTION_PARTIAL=false
read_mr_description() {
  local full
  MR_DESCRIPTION=$(printf '%s\n' "${CI_MERGE_REQUEST_DESCRIPTION:-}" | tr -d '\r')
  [[ "${CI_MERGE_REQUEST_DESCRIPTION_IS_TRUNCATED:-false}" == true ]] || return 0
  MR_DESCRIPTION_PARTIAL=true
  [[ -n "${CI_JOB_TOKEN:-}" && -n "${CI_API_V4_URL:-}" ]] || return 0
  command -v jq >/dev/null 2>&1 || return 0
  if full=$(curl -fsS --max-time 20 -H "JOB-TOKEN: ${CI_JOB_TOKEN}" \
      "${CI_API_V4_URL}/projects/${CI_MERGE_REQUEST_PROJECT_ID:-${CI_PROJECT_ID:-}}/merge_requests/${CI_MERGE_REQUEST_IID:-}" \
      | jq -r '.description // ""'); then
    MR_DESCRIPTION=$(printf '%s\n' "$full" | tr -d '\r')
    MR_DESCRIPTION_PARTIAL=false
  fi
}

# The skip-changelog label says the check is skipped; the description says
# why, for the reviewer: a line "No changelog: <why>" anywhere, or a
# "## No changelog" heading with text under it. Like the portal's
# check_mr_description.sh, the check fails without the reason.
# has_skip_reason tells whether the description says why (and keeps what to
# print in SKIP_REASON); require_skip_reason adds the error when it does not.
SKIP_REASON=""
REASON_LINE='^[[:space:]]*([>*+-][[:space:]]*)*(\*\*|__)?No changelog(\*\*|__)?:[[:space:]*_]*[^[:space:]*_]'
has_skip_reason() {
  local reason
  read_mr_description
  if reason=$(grep -Ei -m 1 "$REASON_LINE" <<<"$MR_DESCRIPTION"); then
    SKIP_REASON="as the description says: $(sed -E 's/^[[:space:]>*+-]+//; s/\*\*//g' <<<"$reason")"
    return 0
  fi
  if awk 'tolower($0) ~ /^##+[[:space:]]*no changelog[[:space:]]*:?[[:space:]]*$/ { s = 1; next } s && /^#/ { exit } s && /[^[:space:]]/ { found = 1 } END { exit !found }' <<<"$MR_DESCRIPTION"; then
    SKIP_REASON="as the \"No changelog\" section of the description says"
    return 0
  fi
  return 1
}
require_skip_reason() {
  if has_skip_reason; then
    echo "changelog: skipped, ${SKIP_REASON}"
    return 0
  fi
  cat >&2 <<MSG
changelog: the merge request is labelled skip-changelog, but its description does not say why.

Add a line that starts with "No changelog:" and says why no customer can notice
the change, anywhere in the description. For example:

  No changelog: CI and release scripts only
  No changelog: refactor, same behaviour
  No changelog: limited availability, announced at general availability

Then retry this job: GitLab does not start a pipeline when a description is edited.
MSG
  if $MR_DESCRIPTION_PARTIAL; then
    echo "changelog: the description is longer than 2700 characters and could not be fetched in full, so only its start was checked; put the line near the top" >&2
  fi
  return 1
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

# Exempt outright: dependency bumps and Renovate. The skip-changelog label
# is honoured after the safe-path check below, and needs the reason.
if [[ ",${CI_MERGE_REQUEST_LABELS:-}," == *,dependencies,* ]]; then
  echo "changelog: skipped, merge request is labelled dependencies"
  exit 0
fi
case "${CI_MERGE_REQUEST_SOURCE_BRANCH_NAME:-}" in
  renovate/*)
    echo "changelog: skipped, Renovate branch"
    exit 0
    ;;
esac
skip_label=false
[[ ",${CI_MERGE_REQUEST_LABELS:-}," == *,skip-changelog,* ]] && skip_label=true

# Files the merge request changes: its merge base against its own head. In a
# merged results or merge train pipeline CI_COMMIT_SHA is a merge with the
# target branch, which also holds what landed there after the merge base, so
# the source branch head is used when GitLab sets it. Fails when git cannot
# tell (no base SHA, a commit not fetchable), and the fragment is then required.
changed_files() {
  local base="${CI_MERGE_REQUEST_DIFF_BASE_SHA:-}"
  local head="${CI_MERGE_REQUEST_SOURCE_BRANCH_SHA:-${CI_COMMIT_SHA:-HEAD}}"
  local sha
  [[ -n "$base" ]] || return 1
  for sha in "$base" "$head"; do
    git cat-file -e "${sha}^{commit}" 2>/dev/null || git fetch -q --depth=1 origin "$sha" 2>/dev/null || return 1
  done
  git diff --name-only "$base" "$head"
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

if $skip_label; then
  require_skip_reason || exit 1
  exit 0
fi

# The line without the label is the likely slip: name it instead of asking
# for a fragment.
if [[ ! -f "$file" ]] && has_skip_reason; then
  cat >&2 <<MSG
changelog: the description says "No changelog", but the merge request has no skip-changelog label.

The line counts only with the label. Add the label "skip-changelog" to the
merge request and retry this job: GitLab does not start a pipeline when a
label changes.
MSG
  exit 1
fi

if [[ ! -f "$file" ]]; then
  cat >&2 <<MSG
changelog: ${file} is missing

Every merge request needs a changelog fragment, or the label "skip-changelog"
with a line "No changelog: <why>" in its description when nothing is
customer-visible. Example ${file}:

\`\`\`release-note:enhancement
You can now set \`source_port_ranges\` and \`destination_port_ranges\` on \`crusoe_vpc_firewall_rule\`, as lists of single ports, port ranges or \`*\`.
<!-- site: infrastructure-cloud/networking -->
<!-- title: Set port ranges on firewall rules in Terraform -->
\`\`\`

Types: ${TYPES//|/, }. See .changelog/README.md. In Claude Code, /changelog
drafts the fragment from the branch diff.
MSG
  exit 1
fi

# Structure and types. Prints one "<type><TAB><site tag><TAB><title>" line per entry.
# Fields are joined with the unit separator: unlike a tab it is not IFS
# whitespace, so an empty site or title survives `read` and `awk -F`.
US=$'\037'
entries=$(awk -v types="$TYPES" -v US="$US" '
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
    print typ US site US title
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
while IFS="$US" read -r typ site title; do
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

summary=$(awk -F "$US" '$2 != "" { print $2 }' <<<"$entries" | paste -sd, -)
titles=$(awk -F "$US" '$3 != "" { print "\"" $3 "\"" }' <<<"$entries" | paste -sd, -)
echo "changelog: ok, ${file}${summary:+ (product tag: ${summary}${titles:+; title: ${titles}})}"

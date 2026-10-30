#!/usr/bin/env bash
# Builds release-manifest.json for the release being cut: the top section of
# CHANGELOG.md turned back into structured entries (type, product tag, title,
# text, merge request), dated the day the release is cut, for the Crusoe
# Updates translator in the docs repo. The release workflow attaches it to the
# GitHub release.
#
# Usage: scripts/release-manifest.sh [--check]
#   Prints the manifest to stdout. --check validates it and prints a status
#   line instead. When RELEASE_VERSION is set (scripts/tag_semver.sh writes it
#   in the release workflow) the section's version must match it, same rule as
#   release-notes.sh.
#   RELEASE_DATE=YYYY-MM-DD sets the manifest date; the default is today (UTC).
set -euo pipefail

CHANGELOG="CHANGELOG.md"
ENTRIES_DIR=".changelog"
ARTIFACT="terraform"
GITHUB_REPO="crusoecloud/terraform-provider-crusoe"
MR_BASE="${CI_PROJECT_URL:-https://gitlab.com/crusoeenergy/island/external/terraform-provider-crusoe}/-/merge_requests"

cd "$(git rev-parse --show-toplevel)"
command -v jq >/dev/null 2>&1 || { echo "release-manifest: jq is required" >&2; exit 2; }

top=$(grep -n -m 1 '^## v[0-9]' "$CHANGELOG" | cut -d: -f1 || true)
if [[ -z "$top" ]]; then
  echo "release-manifest: no '## vX.Y.Z' section in ${CHANGELOG}; run 'make changelog' and commit" >&2
  exit 1
fi
version=$(sed -n "${top}p" "$CHANGELOG" | awk '{ print $2 }')

# The entries are dated the day the release is cut, not with the date in the
# CHANGELOG.md heading: `make changelog` writes that one on the release-prep
# branch, which can merge days before the release. The release workflow runs
# minutes before GoReleaser publishes, so today in UTC is the release day.
date="${RELEASE_DATE:-$(date -u +%Y-%m-%d)}"
if [[ ! "$date" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
  echo "release-manifest: RELEASE_DATE \"${date}\" is not YYYY-MM-DD" >&2
  exit 1
fi

expected="${RELEASE_VERSION:-}"
expected="${expected%%-[0-9]*}"   # merge request pipelines append -<pipeline id>
if [[ -n "$expected" && "$version" != "$expected" ]]; then
  echo "release-manifest: top section of ${CHANGELOG} is ${version} but this release is ${expected}; run 'make changelog' and commit" >&2
  exit 1
fi

# The entries of this release are the fragments of the merge requests the
# section cites, "(!123)" per line (note.tmpl), so no git history is needed.
section=$(awk -v start="$top" 'NR <= start { next } /^## / { exit } { print }' "$CHANGELOG")
mrs=$(grep -o -E '\(![0-9]+\)' <<<"$section" | tr -d '(!)' | sort -un || true)
if [[ -z "$mrs" ]]; then
  echo "release-manifest: the ${version} section cites no merge request; nothing to put in the manifest" >&2
  exit 1
fi

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
while read -r mr; do
  [[ -z "$mr" ]] && continue
  file="${ENTRIES_DIR}/${mr}.txt"
  if [[ ! -f "$file" ]]; then
    echo "release-manifest: the ${version} section cites !${mr} but ${file} does not exist" >&2
    exit 1
  fi
  # One JSON object per release-note block. Body lines are joined with "\n";
  # the site comment becomes the "site" field (null when absent or "none") and
  # the title comment the "title" field (null when absent).
  awk -v mr="$mr" -v url="${MR_BASE}/${mr}" '
    function esc(s) { gsub(/\\/, "\\\\", s); gsub(/"/, "\\\"", s); gsub(/\t/, "\\t", s); return s }
    /^```release-note:/ {
      typ = substr($0, 17); gsub(/[ \t\r]+$/, "", typ)
      inblock = 1; site = ""; title = ""; text = ""
      next
    }
    /^```/ && inblock {
      sub(/(\\n)+$/, "", text)
      s = (site == "" || site == "none") ? "null" : "\"" esc(site) "\""
      t = (title == "") ? "null" : "\"" esc(title) "\""
      printf "{\"type\":\"%s\",\"site\":%s,\"title\":%s,\"text\":\"%s\",\"mr\":\"%s\",\"mr_iid\":%s}\n", esc(typ), s, t, text, esc(url), mr
      inblock = 0
      next
    }
    inblock && /^<!-- *site: *[^ ].* *-->[ \t\r]*$/ {
      site = $0; sub(/^<!-- *site: */, "", site); sub(/ *-->[ \t\r]*$/, "", site)
      next
    }
    inblock && /^<!-- *title: *[^ ].* *-->[ \t\r]*$/ {
      title = $0; sub(/^<!-- *title: */, "", title); sub(/ *-->[ \t\r]*$/, "", title)
      next
    }
    inblock {
      line = $0; sub(/\r$/, "", line)
      if (text == "" && line == "") next
      text = (text == "") ? esc(line) : text "\\n" esc(line)
    }
  ' "$file" >> "$tmp"
done <<<"$mrs"

manifest=$(jq -n \
  --arg artifact "$ARTIFACT" \
  --arg version "$version" \
  --arg date "$date" \
  --arg release_url "https://github.com/${GITHUB_REPO}/releases/tag/${version}" \
  --slurpfile entries "$tmp" \
  '{artifact: $artifact, version: $version, date: $date, release_url: $release_url, entries: $entries}')

count=$(jq '.entries | length' <<<"$manifest")
if [[ "$count" -eq 0 ]]; then
  echo "release-manifest: no release-note entries found in the fragments of ${version}" >&2
  exit 1
fi

if [[ "${1:-}" == "--check" ]]; then
  echo "release-manifest: ${version} (${date}), ${count} entries from $(wc -l <<<"$mrs" | tr -d ' ') merge requests${expected:+, matches RELEASE_VERSION}"
  exit 0
fi
printf '%s\n' "$manifest"

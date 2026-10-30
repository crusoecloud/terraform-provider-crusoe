#!/usr/bin/env bash
# Renders the .changelog/ fragments added since the last tag into a new
# "## vX.Y.Z (date)" section at the top of CHANGELOG.md, for the release-prep
# merge request: `make changelog` on a branch off `main` (see README.md,
# "Contributing"). Running it again before the version is tagged replaces the
# section. HEAD must be on a branch: go-changelog reads the repository through
# an in-memory clone that only follows branches and tags, so a detached HEAD
# (a CI checkout) fails with "object not found".
#
# Usage: scripts/changelog-build.sh [--dry-run]
#   --dry-run        print the section instead of editing CHANGELOG.md
#   VERSION=v1.2.3   override the version (default: next patch of
#                    MAJOR_VERSION.MINOR_VERSION from versions.env, the same
#                    rule scripts/tag_semver.sh applies in the GitHub release
#                    workflow)
set -euo pipefail

# github.com/hashicorp/go-changelog has no tagged releases; pin a commit.
# a47758c56a59 (2026-04-20) is the last one that builds with Go 1.24; later
# commits require Go 1.25. GOTOOLCHAIN=auto lets an older local Go fetch 1.24.
GO_CHANGELOG_VERSION="a47758c56a59"
ENTRIES_DIR=".changelog"
MARKER="<!-- new releases go below this line -->"

dry_run=false
[[ "${1:-}" == "--dry-run" ]] && dry_run=true

cd "$(git rev-parse --show-toplevel)"
. scripts/release-common.sh   # CHANGELOG, fetch_release_tags, release_tags, next_version

fetch_release_tags || echo "changelog: warning, could not fetch the release tags from GitHub; using local tags" >&2
last=$(release_tags | head -n 1)
if [[ -z "$last" ]]; then
  echo "changelog: no v* tag found (they are minted on GitHub); cannot tell which fragments are new" >&2
  exit 1
fi
# go-changelog reads the entries dir at both refs; "-" means "no previous
# release", which is the case while the last tag predates .changelog/.
last_ref="$last"
if ! git cat-file -e "${last}:${ENTRIES_DIR}" 2>/dev/null; then
  last_ref="-"
fi

VERSION="${VERSION:-$(next_version)}"

# Not -local-fs: that mode checks refs out in this working tree and wipes
# uncommitted work. The default mode clones the repo into memory instead.
section=$(GOTOOLCHAIN=auto go run "github.com/hashicorp/go-changelog/cmd/changelog-build@${GO_CHANGELOG_VERSION}" \
  -last-release "$last_ref" \
  -this-release "${THIS_RELEASE:-HEAD}" \
  -git-dir . \
  -entries-dir "$ENTRIES_DIR" \
  -changelog-template "${ENTRIES_DIR}/changelog.tmpl" \
  -note-template "${ENTRIES_DIR}/note.tmpl")

if [[ -z "${section//[[:space:]]/}" ]]; then
  echo "changelog: no fragments in ${ENTRIES_DIR} since ${last}; nothing to release" >&2
  echo "changelog: a release needs at least one entry; add a fragment for it first" >&2
  exit 1
fi
# go-changelog appends the merge request reference after the whole fragment
# body. When the body ends with comment lines (the product tag, the title),
# "(!123)" lands under the last comment and GitHub renders it on a line of its
# own. Move it back onto the last text line and keep the comments after it.
section=$(printf '%s\n' "$section" | awk '
  function flush() { if (has) print held; has = 0; for (i = 1; i <= n; i++) print pend[i]; n = 0 }
  /^<!--.*--> *\(![0-9]+\)$/ {
    ref = $0; sub(/.*--> */, "", ref)
    comment = $0; sub(/ *\(![0-9]+\)$/, "", comment)
    if (has) { print held " " ref; has = 0 } else { print ref }
    for (i = 1; i <= n; i++) print pend[i]; n = 0
    print comment; next
  }
  /^<!--.*-->[ \t]*$/ { pend[++n] = $0; next }
  { flush(); held = $0; has = 1 }
  END { flush() }
')
section=$(printf '%s\n' "$section" | sed '/./,$!d')   # drop leading blank lines

heading="## ${VERSION} ($(date -u +%Y-%m-%d))"

if $dry_run; then
  printf '%s\n\n%s\n' "$heading" "$section"
  exit 0
fi

if ! grep -qxF "$MARKER" "$CHANGELOG"; then
  echo "changelog: marker \"${MARKER}\" not found in ${CHANGELOG}" >&2
  exit 1
fi

tmp=$(mktemp)
stripped=$(mktemp)
trap 'rm -f "$tmp" "$stripped"' EXIT
src="$CHANGELOG"
if grep -q "^## ${VERSION} " "$CHANGELOG"; then
  if git rev-parse -q --verify "refs/tags/${VERSION}" >/dev/null; then
    echo "changelog: ${VERSION} is already tagged; its section cannot be rewritten" >&2
    exit 1
  fi
  # Not released yet: drop the existing section so it is rendered again with
  # the fragments merged since. CHANGELOG.md itself is written once, below.
  awk -v heading="## ${VERSION} " '
    index($0, heading) == 1 { skip = 1; next }
    skip && /^## / { skip = 0 }
    !skip { print }
  ' "$CHANGELOG" > "$stripped"
  src="$stripped"
  echo "changelog: replacing the unreleased ${VERSION} section"
fi

# The marker is matched as a literal line, not as a regex.
{
  awk -v marker="$MARKER" '{ print } $0 == marker { exit }' "$src"
  printf '\n%s\n\n%s\n' "$heading" "$section"
  awk -v marker="$MARKER" 'found { print } $0 == marker { found = 1 }' "$src"
} > "$tmp"
mv "$tmp" "$CHANGELOG"

echo "changelog: added \"${heading}\" to ${CHANGELOG} (fragments since ${last})"
printf '\n%s\n' "$section"

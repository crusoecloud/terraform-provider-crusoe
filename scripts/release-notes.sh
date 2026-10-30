#!/usr/bin/env bash
# Prints the top section of CHANGELOG.md, the release being cut, for GoReleaser
# (--release-notes). When RELEASE_VERSION is set (the release workflow sets it
# from the tag) the section's version must match it, so a release cannot ship
# with stale notes.
#
# Usage: scripts/release-notes.sh [--check]
#   --check   verify only, print nothing but a status line
set -euo pipefail

CHANGELOG="CHANGELOG.md"
cd "$(git rev-parse --show-toplevel)"

top=$(grep -n -m 1 '^## v[0-9]' "$CHANGELOG" | cut -d: -f1 || true)
if [[ -z "$top" ]]; then
  echo "release-notes: no '## vX.Y.Z' section in ${CHANGELOG}; run 'make changelog' on the release candidate and commit" >&2
  exit 1
fi
version=$(sed -n "${top}p" "$CHANGELOG" | awk '{ print $2 }')

expected="${RELEASE_VERSION:-}"
expected="${expected%%-[0-9]*}"   # merge request pipelines append -<pipeline id>; release tags carry no suffix
if [[ -n "$expected" && "$version" != "$expected" ]]; then
  echo "release-notes: top section of ${CHANGELOG} is ${version} but this release is ${expected}; run 'make changelog' and commit" >&2
  exit 1
fi

if [[ "${1:-}" == "--check" ]]; then
  if [[ -n "$expected" ]]; then
    echo "release-notes: ${CHANGELOG} top section is ${version}, matches RELEASE_VERSION"
  else
    echo "release-notes: ${CHANGELOG} top section is ${version}; RELEASE_VERSION is not set, so the version was not compared"
  fi
  exit 0
fi

# Body of the top section: everything after its heading up to the next one.
# Blank lines before the first and after the last line of text are dropped,
# so the notes look the same whether or not an older section follows.
awk -v start="$top" '
  NR <= start { next }
  /^## / { exit }
  /^[ \t]*$/ { pending = pending "\n"; next }
  { if (started) printf "%s", pending; pending = ""; started = 1; print }
' "$CHANGELOG"

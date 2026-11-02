#!/usr/bin/env bash
# Verifies that the top section of CHANGELOG.md is the version the release
# workflow will tag: the next patch of versions.env, counted from the tags on
# GitHub. CI runs it on the release branch and in merge requests into it
# (changelog_release_check), so a stale changelog fails before the merge; the
# GitHub workflow runs the same two checks again before tagging. It also
# refuses a go.mod with a replace directive, which lint lets through.
#
# Usage: scripts/release-check.sh
#   RELEASE_VERSION=vX.Y.Z   the version to compare with (default: the next
#                            patch, or the tag already on HEAD after a release)
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
. scripts/release-common.sh
require_tools git jq
check_go_mod_replace

fetch_release_tags || echo "release-check: warning, could not fetch the release tags from GitHub; using local tags" >&2

# On the release branch this job runs after the merge, when the GitHub
# workflow may already have tagged the commit; the next patch is then one past
# the top section. A tag on HEAD named by the top section is that release
# done, so it is the version to compare with.
if [[ -z "${RELEASE_VERSION:-}" ]]; then
  top=$(top_section_version)
  if [[ -n "$top" && "$(git rev-parse -q --verify "refs/tags/${top}^{commit}" || true)" == "$(git rev-parse HEAD)" ]]; then
    RELEASE_VERSION="$top"
  fi
fi
RELEASE_VERSION="${RELEASE_VERSION:-$(next_version)}"
export RELEASE_VERSION
scripts/release-notes.sh --check
scripts/release-manifest.sh --check

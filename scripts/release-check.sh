#!/usr/bin/env bash
# Verifies that the top section of CHANGELOG.md is the version the release
# workflow will tag: the next patch of versions.env, counted from the tags on
# GitHub. CI runs it on the release branch and in merge requests into it
# (changelog_release_check), so a stale changelog fails before the merge; the
# GitHub workflow runs the same two checks again before tagging.
#
# Usage: scripts/release-check.sh
#   RELEASE_VERSION=vX.Y.Z   the version to compare with (default: computed)
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
. scripts/release-common.sh
require_tools git jq

fetch_release_tags || echo "release-check: warning, could not fetch the release tags from GitHub; using local tags" >&2

RELEASE_VERSION="${RELEASE_VERSION:-$(next_version)}"
export RELEASE_VERSION
scripts/release-notes.sh --check
scripts/release-manifest.sh --check

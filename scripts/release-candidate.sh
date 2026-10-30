#!/usr/bin/env bash
# Cuts the release candidate (README, "Preparing a release"): points
# rc/<version> at this commit and opens the merge request into release, squash
# off, with the CHANGELOG.md section as its description, or refreshes the one
# already open from that branch. <version> is the top section of
# CHANGELOG.md; nothing happens when it is already tagged, and the script
# fails when it is not the version the release workflow would tag. CI runs it
# in cut_release_candidate on main whenever CHANGELOG.md changes, as the
# release bot. Without the bot's token it only writes the merge request text
# to release-candidate-mr.md.
#
# Usage: scripts/release-candidate.sh
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
. scripts/release-common.sh
require_tools git curl jq base64 ssh-keygen

fetch_release_tags || echo "release-candidate: warning, could not fetch the release tags from GitHub; using local tags" >&2

version=$(top_section_version)
if [[ -z "$version" ]]; then
  echo "release-candidate: no '## vX.Y.Z' section in ${CHANGELOG}; run prepare_release first" >&2
  exit 1
fi
if tag_exists "$version"; then
  echo "release-candidate: ${version} is already tagged; nothing to cut"
  exit 0
fi
expected="${RELEASE_VERSION:-$(next_version)}"
expected="${expected%%-[0-9]*}"   # merge request pipelines append -<pipeline id>
if [[ "$version" != "$expected" ]]; then
  echo "release-candidate: top section of ${CHANGELOG} is ${version} but the next release is ${expected}; run prepare_release again, or fix ${VERSIONS_FILE}" >&2
  exit 1
fi

sha=$(git rev-parse HEAD)
branch="rc/${version}"
title="Release ${version}"
{
  cat <<MR
Release candidate for ${version}: \`${branch}\` points at ${sha:0:12}, the commit on \`main\` that carries the release-prep merge. Opened by the cut_release_candidate job${CI_PIPELINE_URL:+ (${CI_PIPELINE_URL})}.

Merging fast-forwards \`release\` to that commit: the GitHub workflow tags ${version} and GoReleaser publishes the release, which the Terraform Registry picks up. Squash is off on purpose, so that \`release\` stays a commit of \`main\`. If more work should ship, merge it on \`main\` and run prepare_release again: once the new section merges, this branch moves, the description is refreshed and approvals start over.

### ${version}

MR
  scripts/release-notes.sh
} > release-candidate-mr.md

if ! bot_available; then
  echo "release-candidate: would push ${sha} as ${branch} and open \"${title}\" into release; no bot token, so nothing was pushed. The merge request text is in release-candidate-mr.md"
  exit 0
fi

bot_push "$sha" "$branch"
open_or_update_mr "$branch" release "$title" release-candidate-mr.md \
  "$(jq -n --argjson assignees "$(assignee_ids)" '{ remove_source_branch: true, squash: false, assignee_ids: $assignees }')"

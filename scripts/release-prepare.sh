#!/usr/bin/env bash
# Opens the release-prep merge request (README, "Preparing a release"): renders
# the fragments since the last tag into CHANGELOG.md, commits it on
# release-prep/<version> and opens the merge request into main, or refreshes
# the one already open from that branch. CI runs it in the manual
# prepare_release job on main, as the release bot. Without the bot's token it
# stops after the commit: release-prep.patch holds the commit and
# release-prep-mr.md the merge request text.
#
# Usage: scripts/release-prepare.sh
#   VERSION=vX.Y.Z   the version to release (default: next patch of
#                    MAJOR_VERSION.MINOR_VERSION in versions.env, the rule the
#                    release workflow applies with scripts/tag_semver.sh). A
#                    new MAJOR.MINOR must end in .0 and bumps versions.env in
#                    the same commit; within the current MAJOR.MINOR only the
#                    next patch is possible, because that is the number the
#                    workflow will tag.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
. scripts/release-common.sh
require_tools git curl jq base64 ssh-keygen

if ! git diff --quiet -- "$CHANGELOG" "$VERSIONS_FILE"; then
  echo "release-prepare: ${CHANGELOG} or ${VERSIONS_FILE} has uncommitted changes; commit or discard them first" >&2
  exit 1
fi

fetch_release_tags || echo "release-prepare: warning, could not fetch the release tags from GitHub; using local tags" >&2

# An explicit version either is the next patch (then it changes nothing) or
# starts a new MAJOR.MINOR, in which case versions.env is bumped here so that
# the release workflow tags the same number when the candidate merges.
bump=false
if [[ -n "${VERSION:-}" ]]; then
  if [[ ! "$VERSION" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    echo "release-prepare: VERSION \"${VERSION}\" is not vX.Y.Z" >&2
    exit 1
  fi
  major="${BASH_REMATCH[1]}" minor="${BASH_REMATCH[2]}" patch="${BASH_REMATCH[3]}"
  if [[ "$major.$minor" == "$(versions_env_major_minor)" ]]; then
    expected=$(next_version)
    if [[ "$VERSION" != "$expected" ]]; then
      echo "release-prepare: the release workflow will tag ${expected} for ${major}.${minor}; a version cannot skip or repeat a patch number (leave it empty)" >&2
      exit 1
    fi
  else
    last=$(release_tags | head -n 1)
    if [[ "$patch" != "0" ]]; then
      echo "release-prepare: a new MAJOR.MINOR starts at v${major}.${minor}.0, not ${VERSION}" >&2
      exit 1
    fi
    if [[ "$(printf '%s\n' "$last" "$VERSION" | sort -V | tail -n 1)" != "$VERSION" ]]; then
      echo "release-prepare: ${VERSION} is not above the latest release ${last}" >&2
      exit 1
    fi
    bump=true
  fi
fi
version="${VERSION:-$(next_version)}"
branch="release-prep/${version}"
title="Release ${version}"

# The branch comes first: go-changelog reads the repository through an
# in-memory clone that only follows branches and tags, so a detached HEAD,
# which every CI checkout is, fails with "object not found".
git checkout -q -B "$branch"
if $bump; then
  set_versions_env "$major" "$minor"
  echo "release-prepare: ${VERSIONS_FILE} bumped to ${major}.${minor}"
fi
VERSION="$version" scripts/changelog-build.sh

if git diff --quiet -- "$CHANGELOG"; then
  echo "release-prepare: ${CHANGELOG} already has the ${version} section for these fragments; nothing to do"
  exit 0
fi

if bot_available; then
  configure_bot_identity
else
  configure_fallback_identity
fi

git add "$CHANGELOG" "$VERSIONS_FILE"
git commit -q -m "$title" ${CI_PIPELINE_URL:+-m "Rendered by the prepare_release job: ${CI_PIPELINE_URL}"}
git format-patch -1 --stdout > release-prep.patch

last=$(release_tags | head -n 1)
{
  cat <<MR
The \`${version}\` section of \`CHANGELOG.md\`, rendered from the fragments merged since \`${last}\`: this is what the release announces. Opened by the prepare_release job${CI_PIPELINE_URL:+ (${CI_PIPELINE_URL})}.

Review the wording, fix it on the branch if needed, and merge. The next pipeline on \`main\` cuts \`rc/${version}\` and opens the merge request into \`release\`. If more work should ship, merge it first and run prepare_release again: the section is rendered again and this merge request updated, until the version is tagged.

### ${version}

MR
  RELEASE_VERSION='' scripts/release-notes.sh
} > release-prep-mr.md

if ! bot_available; then
  echo "release-prepare: committed ${title} on ${branch}; the commit is in release-prep.patch and the merge request text in release-prep-mr.md"
  echo "release-prepare: no bot token, so nothing was pushed. Push the branch and open the merge request into main yourself: git push -u origin ${branch}"
  exit 0
fi

bot_push HEAD "$branch"
open_or_update_mr "$branch" "${CI_DEFAULT_BRANCH:-main}" "$title" release-prep-mr.md \
  "$(jq -n --argjson assignees "$(assignee_ids)" '{ remove_source_branch: true, squash: true, assignee_ids: $assignees }')"

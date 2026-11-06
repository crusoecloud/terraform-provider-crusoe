#!/usr/bin/env bash
# Opens the release-prep merge request (RELEASING.md): renders
# the fragments since the last tag into CHANGELOG.md, commits it on
# release-prep/<version> and opens the merge request into main, or refreshes
# the one already open from that branch. CI runs it in the manual
# prepare_release job on main, as the release bot. Without the bot's token it
# stops after the commit: release-prep.patch holds the commit and
# release-prep-mr.md the merge request text.
#
# Usage: scripts/release-prepare.sh
#   VERSION=vX.Y.Z   the version to release, v optional; or patch, minor or
#                    major, counted from the latest tag (default: next patch of
#                    MAJOR_VERSION.MINOR_VERSION in versions.env, the rule the
#                    release workflow applies with scripts/tag_semver.sh). A
#                    new MAJOR.MINOR must end in .0 and bumps versions.env in
#                    the same commit; within the current MAJOR.MINOR only the
#                    next patch is possible, because that is the number the
#                    workflow will tag.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
. scripts/release-common.sh
require_tools git jq
if bot_available; then require_tools curl base64 ssh-keygen; fi   # the push and the merge request

if ! git diff --quiet -- "$CHANGELOG" "$VERSIONS_FILE"; then
  echo "release-prepare: ${CHANGELOG} or ${VERSIONS_FILE} has uncommitted changes; commit or discard them first" >&2
  exit 1
fi
check_go_mod_replace

fetch_release_tags || echo "release-prepare: warning, could not fetch the release tags from GitHub; using local tags" >&2

# An explicit version either is the next patch (then it changes nothing) or
# starts a new MAJOR.MINOR, in which case versions.env is bumped here so that
# the release workflow tags the same number when the candidate merges.
bump=false
last=$(release_tags | head -n 1)
explicit=${VERSION:+set}  # empty: the next patch, or the release in flight
last_mm="${last#v}"; last_mm="${last_mm%.*}"  # MAJOR.MINOR of the latest release
# "patch", "minor" and "major" name the bump from the latest release; a
# number says it exactly.
case "${VERSION:-}" in
  patch|minor|major) VERSION=$(bump_version "$VERSION" "$last") ;;
esac
if [[ -n "${VERSION:-}" ]]; then
  VERSION="v${VERSION#v}"  # the v is optional on input
  if [[ ! "$VERSION" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    echo "release-prepare: VERSION \"${VERSION#v}\" is not X.Y.Z" >&2
    exit 1
  fi
  major="${BASH_REMATCH[1]}" minor="${BASH_REMATCH[2]}" patch="${BASH_REMATCH[3]}"
  if [[ "$major.$minor" == "$last_mm" ]]; then
    expected=$(bump_version patch "$last")
    if [[ "$VERSION" != "$expected" ]]; then
      echo "release-prepare: the release workflow will tag ${expected} for ${major}.${minor}; a version cannot skip or repeat a patch number" >&2
      exit 1
    fi
    # versions.env ahead of the latest release: a minor or major in flight
    # goes back to a patch, so the file goes back with it.
    [[ "$(versions_env_major_minor)" == "$major.$minor" ]] || bump=true
  else
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
title="Prepare release ${version}"

# What the fragments call for is a suggestion, never an automatic bump
# (RELEASING.md, "1. Run prepare_release"): the releaser reads it and decides.
suggested=$(suggested_bump "$last")
chosen=$(bump_kind "$version" "$last")
advice=""
if (( $(bump_rank "$suggested") > $(bump_rank "$chosen") )); then
  suggested_version=$(bump_version "$suggested" "$last")
  if [[ -z "$explicit" ]]; then
    echo "release-prepare: the fragments since ${last} ($(fragment_type_counts "$last")) call for a ${suggested} release, ${suggested_version}, and version was left empty, which means the ${chosen} ${version}. Set version to ${suggested}, or to ${chosen} to prepare ${version} anyway, and run again (\"Retry job with modified values\")." >&2
    exit 1
  fi
  advice="The fragments since ${last} ($(fragment_type_counts "$last")) call for a ${suggested} release, ${suggested_version}; this merge request prepares the ${chosen} ${version}. To follow the suggestion, retry prepare_release with version set to ${suggested} (\"Retry job with modified values\"): it opens the merge request for ${suggested_version} and closes this one."
  echo "release-prepare: WARNING: ${advice}" >&2
else
  echo "release-prepare: the fragments since ${last} call for a ${suggested} release; preparing ${version} (${chosen})"
fi

# The branch comes first: go-changelog reads the repository through an
# in-memory clone that only follows branches and tags, so a detached HEAD,
# which every CI checkout is, fails with "object not found".
git checkout -q -B "$branch"
if $bump; then
  set_versions_env "$major" "$minor"
  echo "release-prepare: ${VERSIONS_FILE} set to ${major}.${minor}"
fi
# The top section names a release in flight when it was prepared but never
# tagged: a different version replaces it (changelog-build.sh), and its
# candidate into release is closed below.
top=$(top_section_version)
replaced=""
if [[ -n "$top" && "$top" != "$version" ]] && ! tag_exists "$top"; then
  replaced=$top
  echo "release-prepare: ${top} was prepared but never tagged; ${version} replaces it, and rc/${top} is closed when this merges"
fi
VERSION="$version" scripts/changelog-build.sh

if git diff --quiet -- "$CHANGELOG"; then
  echo "release-prepare: ${CHANGELOG} already has the ${version} section for these fragments; nothing to do"
  # A change of mind back to this version leaves the other one's merge
  # request behind.
  ! bot_available || close_superseded_mrs release-prep/ "${CI_DEFAULT_BRANCH:-main}" "$branch" "$version"
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

{
  [[ -z "$advice" ]] || printf '> **Version suggestion.** %s\n\n' "$advice"
  # shellcheck disable=SC2016 # markdown backticks, not a command
  [[ -z "$replaced" ]] || printf '> **Note.** This replaces the `%s` section, prepared but never tagged. When this merges, the candidate for `%s` opens and `rc/%s` is closed; do not merge `rc/%s` meanwhile.\n\n' "$replaced" "$version" "$replaced" "$replaced"
  cat <<MR
The \`${version}\` section of \`CHANGELOG.md\`, rendered from the fragments merged since \`${last}\`: this is what the release announces. Opened by the prepare_release job${CI_PIPELINE_URL:+ (${CI_PIPELINE_URL})}.

Review the wording and merge. A wrong entry is fixed in its fragment, through a merge request into \`main\`, and prepare_release run again: the release manifest is built from the fragments, and the next run replaces this section. The next pipeline on \`main\` cuts \`rc/${version}\` and opens the merge request into \`release\`. If more work should ship, merge it first and run prepare_release again: the section is rendered again and this merge request updated, until the version is tagged.

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
close_superseded_mrs release-prep/ "${CI_DEFAULT_BRANCH:-main}" "$branch" "$version"

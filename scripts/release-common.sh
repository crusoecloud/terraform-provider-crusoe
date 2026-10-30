#!/usr/bin/env bash
# Shared by the changelog and release scripts (changelog-build.sh,
# release-notes.sh, release-manifest.sh): the version rule, the release tags
# and the top section of CHANGELOG.md. Source it from the repository root; it
# is not a command.

CHANGELOG="CHANGELOG.md"
VERSIONS_FILE="versions.env"
# Releases are tagged on GitHub by .github/workflows/tag-release.yml, not on
# GitLab, so the vX.Y.Z tags exist only there. The repository is public.
GITHUB_REPO="crusoecloud/terraform-provider-crusoe"

# Copies the release tags from GitHub into this clone, so that release_tags
# and go-changelog can see them. The GitLab remote has none.
fetch_release_tags() {
  git fetch -q "https://github.com/${GITHUB_REPO}.git" '+refs/tags/v*:refs/tags/v*'
}

# Release tags only (vX.Y.Z), newest first. tag_semver.sh never creates
# pre-release tags, and a hand-made one must not shift the patch number.
release_tags() {
  git tag -l 'v[0-9]*' --sort=-version:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' || true
}

# MAJOR.MINOR as versions.env declares it.
versions_env_major_minor() {
  local major minor
  major=$(sed -n 's/^export MAJOR_VERSION=\([0-9]*\).*/\1/p' "$VERSIONS_FILE")
  minor=$(sed -n 's/^export MINOR_VERSION=\([0-9]*\).*/\1/p' "$VERSIONS_FILE")
  echo "${major}.${minor}"
}

# set_versions_env MAJOR MINOR: rewrites the two values in place.
set_versions_env() {
  sed -i.bak -e "s/^export MAJOR_VERSION=[0-9]*/export MAJOR_VERSION=$1/" -e "s/^export MINOR_VERSION=[0-9]*/export MINOR_VERSION=$2/" "$VERSIONS_FILE"
  rm -f "${VERSIONS_FILE}.bak"
  [[ "$(versions_env_major_minor)" == "$1.$2" ]] || { echo "release: could not set MAJOR_VERSION/MINOR_VERSION in ${VERSIONS_FILE}" >&2; exit 1; }
}

# The version tag_semver.sh would create: the next patch of
# MAJOR_VERSION.MINOR_VERSION from versions.env.
next_version() {
  local major minor prev
  major=${1:-$(versions_env_major_minor)}; minor=${major#*.}; major=${major%%.*}
  prev=$(release_tags | grep -E "^v${major}\.${minor}\.[0-9]+$" | head -n 1 || true)
  if [[ -n "$prev" ]]; then
    echo "v${major}.${minor}.$(( ${prev##*.} + 1 ))"
  else
    echo "v${major}.${minor}.0"
  fi
}

# The version named by the top section of CHANGELOG.md; empty when there is
# none. Sections written by hand up to 1.8.0 have no "v" and do not count.
top_section_version() {
  grep -m 1 '^## v[0-9]' "$CHANGELOG" | awk '{ print $2 }' || true
}

tag_exists() {
  git rev-parse -q --verify "refs/tags/$1" >/dev/null
}

# require_tools NAME...: stop with one clear message when a command is
# missing, before anything is rendered, committed or pushed.
require_tools() {
  local missing=()
  for tool in "$@"; do
    command -v "$tool" >/dev/null || missing+=("$tool")
  done
  if (( ${#missing[@]} > 0 )); then
    echo "release: missing on this machine: ${missing[*]}" >&2
    exit 1
  fi
}

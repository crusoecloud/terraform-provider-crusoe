#!/usr/bin/env bash
# Waits for the GitHub release of the version just merged into release to
# carry release-manifest.json, then writes RELEASE_VERSION to release.env for
# notify_docs, the trigger job that starts the docs pipeline (RELEASING.md).
# The version is the top section of CHANGELOG.md, the one the GitHub workflow
# tags. The workflow starts after the push mirror and uploads the assets last,
# so the file shows up some minutes after the merge.
#
# Usage: scripts/release-wait.sh
#   RELEASE_VERSION=vX.Y.Z      the version to wait for (default: the top section)
#   RELEASE_WAIT_SECONDS=1800   how long to wait before giving up
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
. scripts/release-common.sh
require_tools git curl

version="${RELEASE_VERSION:-$(top_section_version)}"
if [[ -z "$version" ]]; then
  echo "release-wait: no '## vX.Y.Z' section in ${CHANGELOG}; nothing to announce" >&2
  exit 1
fi
url="https://github.com/${GITHUB_REPO}/releases/download/${version}/release-manifest.json"
wait_seconds="${RELEASE_WAIT_SECONDS:-1800}"
deadline=$(( $(date +%s) + wait_seconds ))

until curl -fsLI -o /dev/null "$url"; do
  if (( $(date +%s) >= deadline )); then
    echo "release-wait: ${url} is still missing after ${wait_seconds}s; the GitHub workflow may have failed. Once the release exists, start a pipeline on main of crusoeenergy/island/external/docs with RELEASE_ARTIFACT=terraform and RELEASE_VERSION=${version}" >&2
    exit 1
  fi
  echo "release-wait: ${url} not there yet; checking again in 30s"
  sleep 30
done

echo "RELEASE_VERSION=${version}" > release.env
echo "release-wait: ${url} is there; notify_docs will import it"

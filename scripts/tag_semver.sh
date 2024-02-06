#!/usr/bin/env bash
set -e

MAJOR_VERSION=$1
MINOR_VERSION=$2
TAG_PREFIX=$3

# find the latest tag
NEW_VERSION="${TAG_PREFIX}v${MAJOR_VERSION}.${MINOR_VERSION}.0"
git fetch -q --tags --prune --prune-tags
tags=$(git tag -l ${TAG_PREFIX}v${MAJOR_VERSION}.${MINOR_VERSION}.* --sort=-version:refname)
if [[ ! -z "$tags" ]]; then
  arr=(${tags})
  for val in ${arr[@]}; do
    if [[ "$val" =~ ^${TAG_PREFIX}v${MAJOR_VERSION}+\.${MINOR_VERSION}\.[0-9]+$ ]]; then
      prev_build=$(echo ${val} | cut -d. -f3)
      new_build=$((prev_build+1))
      NEW_VERSION="${TAG_PREFIX}v${MAJOR_VERSION}.${MINOR_VERSION}.${new_build}"
      break
    fi
  done
fi

echo "Version for this commit: ${NEW_VERSION}"
echo "RELEASE_VERSION=${NEW_VERSION}" >> variables.env

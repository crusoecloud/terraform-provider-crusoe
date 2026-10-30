#!/usr/bin/env bash
# Shared by the changelog and release scripts (changelog-build.sh,
# release-notes.sh, release-manifest.sh, release-prepare.sh,
# release-candidate.sh, release-check.sh): the version rule, the release tags,
# the top section of CHANGELOG.md, the GitLab API and the release bot's git
# identity. Source it from the repository root; it is not a command.

CHANGELOG="CHANGELOG.md"
VERSIONS_FILE="versions.env"
# Releases are tagged on GitHub by .github/workflows/tag-release.yml, not on
# GitLab, so the vX.Y.Z tags exist only there. The repository is public.
GITHUB_REPO="crusoecloud/terraform-provider-crusoe"

# The release bot (devx/terraform, module ccx_release_bot). Its token and
# signing key (base64, one line) are protected, masked CI variables, so they
# exist on main only; without them the scripts stop before pushing and leave
# their output behind as files, which the jobs keep as artifacts.
BOT_TOKEN="${SVC_CRUSOE_CCX_RELEASE_BOT_TOKEN:-}"
BOT_SIGNING_KEY="${SVC_CRUSOE_CCX_RELEASE_BOT_SIGNING_KEY:-}"

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

# True when the script can push and open merge requests: the bot's token is
# set and the script runs in GitLab CI, where the API location and the project
# come from the predefined variables.
bot_available() {
  [[ -n "$BOT_TOKEN" && -n "${CI_PROJECT_ID:-}" ]]
}

# Commits as the bot: its name and verified e-mail from GET /user, so that
# commit_committer_check accepts the push, and SSH signatures with the key
# registered on the account, so that reject_unsigned_commits does too.
configure_bot_identity() {
  local user name email key
  command -v ssh-keygen >/dev/null || { echo "release: ssh-keygen is required to sign commits (openssh-client)" >&2; exit 1; }
  [[ -n "$BOT_SIGNING_KEY" ]] || { echo "release: SVC_CRUSOE_CCX_RELEASE_BOT_SIGNING_KEY is not set" >&2; exit 1; }
  user=$(curl -fsS --header "PRIVATE-TOKEN: ${BOT_TOKEN}" "${CI_API_V4_URL}/user")
  name=$(printf '%s' "$user" | jq -r '.name')
  email=$(printf '%s' "$user" | jq -r '.email // empty')
  [[ -n "$email" ]] || { echo "release: GET /user returned no e-mail for the bot; it needs a verified e-mail to commit here" >&2; exit 1; }
  key="${HOME}/.release-bot-signing-key"
  # The variable holds the key base64-encoded, one line, so that GitLab can
  # mask it; decode it to a file only this user can read.
  (umask 077 && printf '%s' "$BOT_SIGNING_KEY" | base64 -d > "$key")
  [[ -s "$key" ]] || { echo "release: could not decode SVC_CRUSOE_CCX_RELEASE_BOT_SIGNING_KEY (expected base64)" >&2; exit 1; }
  git config user.name "$name"
  git config user.email "$email"
  git config gpg.format ssh
  git config user.signingkey "$key"
  git config commit.gpgsign true
}

# Without the bot, commit as whoever runs the script: the local git identity,
# or in CI the person who started the job.
configure_fallback_identity() {
  if ! git config user.email >/dev/null; then
    git config user.name "${GITLAB_USER_NAME:-Release prep}"
    git config user.email "${GITLAB_USER_EMAIL:-support@crusoecloud.com}"
  fi
}

# bot_push REF BRANCH: pushes REF as BRANCH, replacing it when it exists. The
# token reaches git through a credential helper, not the remote URL, so that
# an error message from git cannot print it.
bot_push() {
  # shellcheck disable=SC2016  # git's shell expands the token, not this one
  RELEASE_BOT_GIT_TOKEN="$BOT_TOKEN" GIT_TERMINAL_PROMPT=0 \
    git -c credential.helper='!f() { echo "username=oauth2"; echo "password=${RELEASE_BOT_GIT_TOKEN}"; }; f' \
    push --force "https://${CI_SERVER_HOST}/${CI_PROJECT_PATH}.git" "$1:refs/heads/$2"
}

# gitlab_api METHOD PATH [JSON]: a call on this project; fails on HTTP errors.
gitlab_api() {
  local method=$1 path=$2 body=${3:-}
  curl -fsS -X "$method" \
    --header "PRIVATE-TOKEN: ${BOT_TOKEN}" \
    --header "Content-Type: application/json" \
    ${body:+--data "$body"} \
    "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}${path}"
}

# Who started the job, as the JSON list assignee_ids takes; empty outside CI.
assignee_ids() {
  if [[ -n "${GITLAB_USER_ID:-}" ]]; then echo "[${GITLAB_USER_ID}]"; else echo "[]"; fi
}

# open_or_update_mr SOURCE TARGET TITLE DESCRIPTION_FILE [EXTRA_JSON]
# Refreshes the merge request already open from SOURCE, or creates it. The
# EXTRA_JSON attributes (squash, assignees...) are sent both times, so that a
# setting changed by hand in between, squash on the release MR for example, is
# put back.
open_or_update_mr() {
  local source=$1 target=$2 title=$3 description_file=$4 extra=${5:-'{}'}
  local encoded iid payload
  encoded=$(jq -rn --arg s "$source" '$s | @uri')
  iid=$(gitlab_api GET "/merge_requests?source_branch=${encoded}&state=opened" | jq -r '.[0].iid // empty')
  if [[ -n "$iid" ]]; then
    payload=$(jq -n --arg title "$title" --rawfile desc "$description_file" --argjson extra "$extra" \
      '{ title: $title, description: $desc } + $extra')
    gitlab_api PUT "/merge_requests/${iid}" "$payload" | jq -r '"merge request updated: " + .web_url'
  else
    payload=$(jq -n --arg source "$source" --arg target "$target" --arg title "$title" \
      --rawfile desc "$description_file" --argjson extra "$extra" \
      '{ source_branch: $source, target_branch: $target, title: $title, description: $desc } + $extra')
    gitlab_api POST "/merge_requests" "$payload" | jq -r '"merge request opened: " + .web_url'
  fi
}

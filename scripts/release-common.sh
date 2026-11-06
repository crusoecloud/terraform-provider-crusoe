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
MR_URL=""  # the merge request open_or_update_mr last opened or updated

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

# fragments_since LAST [THIS]: the fragments added between the release tag
# LAST (empty or "-": none yet) and THIS (default HEAD), the set go-changelog
# renders into the next section.
fragments_since() {
  if [[ -z "${1:-}" || "$1" == "-" ]]; then
    git ls-tree -r --name-only "${2:-HEAD}" -- .changelog | grep -E '\.txt$' || true
  else
    git diff --name-only --diff-filter=A "$1" "${2:-HEAD}" -- .changelog | grep -E '\.txt$' || true
  fi
}

# fragment_types_since LAST [THIS]: the release-note type of every block in
# those fragments, one per line, read at THIS.
fragment_types_since() {
  local f
  while IFS= read -r f; do
    git show "${2:-HEAD}:${f}" | sed -n 's/^```release-note:\([a-z-]*\).*/\1/p'
  done < <(fragments_since "$1" "${2:-HEAD}")
}

# fragment_type_counts LAST [THIS]: "2 feature, 1 bug", most frequent first.
fragment_type_counts() {
  fragment_types_since "$1" "${2:-HEAD}" | sort | uniq -c | sort -rn \
    | awk '{ printf "%s%s %s", sep, $1, $2; sep = ", " }'
}

# bump_for_types TYPE...: the release the fragment types call for, as major,
# minor or patch: a breaking change, new or deprecated functionality, the rest.
bump_for_types() {
  local t bump=patch
  for t in "$@"; do
    case "$t" in
      breaking-change) bump=major ;;
      feature|enhancement|deprecation) [[ "$bump" == major ]] || bump=minor ;;
    esac
  done
  echo "$bump"
}

# suggested_bump LAST [THIS]: what the fragments since LAST call for. Below
# 1.0.0 anything may change, so a breaking change calls for a minor there.
suggested_bump() {
  local bump
  # shellcheck disable=SC2046 # one word per type on purpose
  bump=$(bump_for_types $(fragment_types_since "$1" "${2:-HEAD}"))
  [[ "$bump" == major && "${1#v}" == 0.* ]] && bump=minor
  echo "$bump"
}

# bump_kind VERSION LAST: how VERSION differs from LAST: major, minor or patch.
bump_kind() {
  local v=${1#v} last=${2#v}
  if [[ "${v%%.*}" != "${last%%.*}" ]]; then echo major
  elif [[ "${v%.*}" != "${last%.*}" ]]; then echo minor
  else echo patch; fi
}

# bump_rank KIND: patch 0, minor 1, major 2, to compare two kinds.
bump_rank() {
  case "$1" in major) echo 2 ;; minor) echo 1 ;; *) echo 0 ;; esac
}

# bump_version KIND LAST: the version after the release tag LAST for KIND =
# major, minor or patch, counted from LAST alone.
bump_version() {
  local major minor patch
  [[ -n "${2:-}" ]] || { echo "release: no release tag to count from" >&2; return 1; }
  IFS=. read -r major minor patch <<< "${2#v}"
  case "$1" in
    major) echo "v$((major + 1)).0.0" ;;
    minor) echo "v${major}.$((minor + 1)).0" ;;
    *) echo "v${major}.${minor}.$((patch + 1))" ;;
  esac
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

# A replace directive in go.mod points a module at a preview or a local copy.
# Lint allows the client-go ones, so the prep and the check before the merge
# into release are the two places that refuse one. Every directive has a =>.
check_go_mod_replace() {
  if grep -q '=>' go.mod; then
    echo "release: go.mod has a replace directive; a release cannot ship a replaced module" >&2
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
# an error message from git cannot print it. The runner's git config, included
# from .git/config and from GIT_CONFIG_GLOBAL, rewrites this project's plain
# URL into one carrying the job token, which cannot push, and can add a helper
# of its own: the user name in the URL keeps the rewrite from matching, and the
# empty helper first clears the list of helpers (gitcredentials(7)).
bot_push() {
  # shellcheck disable=SC2016  # git's shell expands the token, not this one
  RELEASE_BOT_GIT_TOKEN="$BOT_TOKEN" GIT_TERMINAL_PROMPT=0 \
    git -c credential.helper= \
      -c credential.helper='!f() { echo "username=oauth2"; echo "password=${RELEASE_BOT_GIT_TOKEN}"; }; f' \
      push --force "https://oauth2@${CI_SERVER_HOST}/${CI_PROJECT_PATH}.git" "$1:refs/heads/$2"
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
    MR_URL=$(gitlab_api PUT "/merge_requests/${iid}" "$payload" | jq -r .web_url)
    echo "merge request updated: ${MR_URL}"
  else
    payload=$(jq -n --arg source "$source" --arg target "$target" --arg title "$title" \
      --rawfile desc "$description_file" --argjson extra "$extra" \
      '{ source_branch: $source, target_branch: $target, title: $title, description: $desc } + $extra')
    MR_URL=$(gitlab_api POST "/merge_requests" "$payload" | jq -r .web_url)
    echo "merge request opened: ${MR_URL}"
  fi
}

# close_superseded_mrs PREFIX TARGET KEEP VERSION: a retry of prepare_release
# with another version opens new merge requests (the branches carry the
# version), so the other open ones from a PREFIX branch into TARGET, KEEP
# aside, are closed with a note that points at the one for VERSION (MR_URL),
# and their branches deleted; merge requests from forks are left alone, their
# branch names are not ours. Best effort: a failure is reported, not fatal.
close_superseded_mrs() {
  local prefix=$1 target=$2 keep=$3 version=$4 encoded iid source note
  encoded=$(jq -rn --arg s "$target" '$s | @uri')
  while IFS=$'\t' read -r iid source; do
    [[ -n "$iid" ]] || continue
    note="Closed by the release bot: the release is now ${version}${MR_URL:+, see ${MR_URL}}; this merge request and its branch are superseded."
    gitlab_api POST "/merge_requests/${iid}/notes" "$(jq -n --arg body "$note" '{ body: $body }')" >/dev/null \
      || echo "release: could not comment on !${iid}" >&2
    gitlab_api PUT "/merge_requests/${iid}" '{ "state_event": "close" }' >/dev/null \
      || { echo "release: could not close !${iid} (${source}); close it by hand" >&2; continue; }
    gitlab_api DELETE "/repository/branches/$(jq -rn --arg s "$source" '$s | @uri')" >/dev/null \
      || echo "release: could not delete ${source}; delete it by hand" >&2
    echo "merge request !${iid} (${source}) closed: superseded by ${version}"
  done < <(gitlab_api GET "/merge_requests?state=opened&target_branch=${encoded}&per_page=100" \
    | jq -r --arg prefix "$prefix" --arg keep "$keep" '.[] | select(.source_project_id == .project_id) | select(.source_branch | startswith($prefix)) | select(.source_branch != $keep) | [.iid, .source_branch] | @tsv')
}

# Releasing the provider

## How to

1. Look at the [changes since the last release](https://gitlab.com/crusoeenergy/island/external/terraform-provider-crusoe/-/compare/release...main):
   the commits that would ship and, under `.changelog/`, the fragments that
   become the release notes.
2. Open the [latest pipeline on `main`](https://gitlab.com/crusoeenergy/island/external/terraform-provider-crusoe/-/pipelines/latest?ref=main),
   select the name of the `prepare_release` job and press "Run job". Leave
   `version` empty for a patch release; type `minor`, `major` or `vX.Y.0` for
   a minor or major one. Left empty, the job stops if the fragments call for
   more than a patch, and says which value to set.
3. The job opens the merge request `Prepare release vX.Y.Z` into `main`,
   assigned to you. Read the changelog section and merge it.
4. That merge opens a second merge request, `Release vX.Y.Z` into `release`.
   Get one approval from Product Managers and one from CCX. Leave "Squash
   commits" unticked.
5. Merge it. The GitHub workflow tags the release and publishes it, the
   Terraform Registry picks it up, and the GitLab pipeline opens the Crusoe
   Updates merge request in the docs repo.

## Detailed information

`release` only accepts fast-forward merges, so a release is always a commit of
`main`. The flow is: a release-prep merge request on `main` that adds the
changelog section, a release-candidate branch that points at that commit, and a
merge request from it into `release`. GitLab mirrors `release` to
[GitHub](https://github.com/crusoecloud/terraform-provider-crusoe), where the
`semver tag` workflow tags and publishes; GitLab never tags, and the release
tags exist only on GitHub. Every customer-visible change arrives with a
changelog fragment (see [`.changelog/README.md`](.changelog/README.md)), so the
release notes are generated, not written, at release time. Two CI jobs do the
branch and merge-request work as the release bot; the human steps are the
reviews.

### 1. Run `prepare_release`

Open the [latest pipeline on `main`](https://gitlab.com/crusoeenergy/island/external/terraform-provider-crusoe/-/pipelines/latest?ref=main)
and select the name of the `prepare_release` job: the job page has a
`version` field and a "Run job" button. (The play icon in the pipeline graph
skips the form; it asks for confirmation and then runs the job with the
default.) Leave `version` empty for a patch release: the job takes the next
patch of `MAJOR_VERSION.MINOR_VERSION` in `versions.env`, counted from the tags
on GitHub, the same rule `tag_semver.sh` applies in the workflow. For a minor
or major release, enter `minor`, `major` or the exact `vX.Y.0` (the `v` is
optional): the job bumps `versions.env` in the same commit, so the merge
request carries both files. A patch number cannot be skipped: the workflow
always creates the one after the latest tag, and the job refuses anything
else.

The fragments say which release they call for: a `breaking-change` entry a
major, a `feature`, `enhancement` or `deprecation` entry a minor, anything
else a patch (see "Types" in `.changelog/README.md`). The job compares that
with the version it prepares. With `version` empty, it stops when the
fragments call for more than a patch and says which value to set. With
`version` set, it prepares what was asked and, when the fragments call for
more, says so in its log and at the top of the merge request. It never bumps
on its own: to change your mind, retry the job with `version` set to
`minor` or `major`; it opens the merge request for that version and closes
the one it had opened. A release in flight changes number the same way: when
the top section of `CHANGELOG.md` was prepared but never tagged, the new
version replaces it, and its candidate into `release` is closed when the new
one is cut; do not merge the old candidate meanwhile.

The job renders the fragments added since the last tag into a new
`## vX.Y.Z (date)` section at the top of `CHANGELOG.md`, commits it on
`release-prep/vX.Y.Z` and opens the merge request into `main`, assigned to
you, with the section as its description. It fails when no fragment was added
since the last tag (see "Releases without a fragment"), and when `go.mod` has a
`replace` directive, which lint allows for `client-go` previews but a release
must not carry.

Before running it, check the deprecation messages
(`common.FormatDeprecationWithReplacement`) merged since the last release: one
that names a version already released was written for a release that did not
include it, and must name the version being prepared.

Read the section and merge. A wrong entry is fixed in its fragment, through a
merge request into `main`, and `prepare_release` is run again: the release
manifest for Crusoe Updates is built from the fragments, not from
`CHANGELOG.md`, and the next run would overwrite an edit made on the branch
anyway. The release-prep merge request needs no fragment and no label:
`changelog_check` skips merge requests that only change `CHANGELOG.md` and
`versions.env`.

If more work lands on `main` before the release is cut and it should ship too,
run `prepare_release` again on the new `main`: the section is rendered again
and the merge request updated, as long as the version is not tagged yet.

Once the job has run on a pipeline, or was cancelled, its page shows "Retry"
instead of "Run job". "Retry" runs it again with the same `version`; the
arrow next to it, "Retry job with modified values", brings the form back,
for a run with a different `version` (a minor instead of a patch, say)
without a new pipeline. A new commit on `main` starts a new pipeline anyway,
with a fresh "Run job".

### 2. Get the release merge request approved

Merging the prep merge request runs `cut_release_candidate` on `main`, which
creates `rc/vX.Y.Z` pointing at that commit and opens the merge request
`Release vX.Y.Z` into `release`, squash off, with the section as its
description. That list is what the reviewers approve: GitLab requires one
approval from Product Managers and one from CCX on this branch. The merge
request pipeline runs `changelog_release_check`, which fails when the top
section of `CHANGELOG.md` is not the version the workflow is about to tag.

The candidate branch is a pointer, nothing more. Do not commit on it, and do
not tick "Squash commits": `release` is fast-forward only, so a commit that
exists only on the candidate would block the next release (1.1.0 needed
17ec31d to reconcile one). To ship more, go back to step 1; when the new
section merges, the job moves the branch and refreshes the description, and
the approvals start over.

### 3. Merge

Merging into `release` starts two things. On GitLab, the `release` pipeline:
`test_and_lint`, `changelog_release_check` on the merged commit, and
`wait_for_release`, which polls the GitHub release for up to 30 minutes. On
GitHub, where the push mirror delivers the merge a minute or so later, the
`semver tag` workflow: it computes the version from `versions.env` and the
existing tags, checks that the top section of `CHANGELOG.md` is that version,
pushes the tag and runs GoReleaser with the section as the release notes. The
signed binaries go to the GitHub release, which the Terraform Registry picks up
on its own, and the workflow posts to `#ccx-ci-releases`. The release also
carries `release-manifest.json` (one entry per fragment, with its product tag)
for the docs site; `make release-manifest` prints it locally.

Once the manifest is on the release, `notify_docs` starts a pipeline in the
docs repo
([`crusoeenergy/island/external/docs`](https://gitlab.com/crusoeenergy/island/external/docs))
with `RELEASE_ARTIFACT=terraform` and the version. Its `changelog-import` job
downloads the manifest from the GitHub release, turns every entry that carries
a product tag into a [Crusoe Updates](https://docs.crusoe.ai/updates/) entry
and opens a merge request, as the same release bot, for the docs team to
review; nothing is published without that review. GitLab runs the trigger as
the person who merged into `release`, who needs at least Developer access on
the docs project; members of the `island/external` group have it.

Afterwards, check that the tag is on the merge commit of the release merge
request, that the GitHub release body shows the changelog, that the
[Registry](https://registry.terraform.io/providers/crusoecloud/crusoe) lists
the version, that the announcement reached `#ccx-ci-releases` (silence may be
an unset `SLACK_CCX_CI_RELEASES_WEBHOOK_URL`, which the GitHub Actions secret
and the GitLab variable both need), that `git log origin/main..origin/release`
prints nothing, and that the docs pipeline linked from `notify_docs` opened
its merge request when the release had tagged entries.

### By hand

The jobs only save typing; the merge requests they open are ordinary ones, and
every step can be done locally when CI is unavailable.

```sh
git fetch origin
git checkout -b release-prep/v1.8.1 origin/main
make changelog                     # minor or major: bump versions.env first,
git add CHANGELOG.md versions.env  # then VERSION=v1.9.0 make changelog
git commit -m "Prepare release v1.8.1"
git push -u origin release-prep/v1.8.1
```

`make changelog-preview` prints the section without touching the file, and
`scripts/release-prepare.sh` does the three middle lines in one go (with
`VERSION=v1.9.0` it also bumps `versions.env`). The scripts fetch the tags from
GitHub themselves. Until the version is tagged, running `make changelog` again
replaces the section. Open the merge request into `main` and merge it. Then:

```sh
git fetch origin
git branch rc/v1.8.1 origin/main      # the commit that contains the section
git push origin rc/v1.8.1
```

Open the merge request into `release` titled `Release v1.8.1`, paste the new
`CHANGELOG.md` section into the description and untick "Squash commits": the
project pre-ticks it on every new merge request.

### Releases without a fragment

A release needs at least one changelog line: `prepare_release` stops with
"no fragments in .changelog since vX.Y.Z; nothing to release", and
`changelog_release_check` fails if such a commit reaches `release` anyway. If
something merged with `skip-changelog` is the reason to release (a dependency
with a CVE, a fix nobody would notice), say so in a fragment, through a normal
merge request into `main`, then prepare the release as usual. If nothing a
customer could notice changed, there is nothing to announce, and probably no
reason to release yet.

### Hotfixes

Fix it on `main` through a normal merge request with its fragment, then prepare
and release as above. Because `release` is always a commit of `main`, a hotfix
cannot skip `main`; `main` is releasable at all times, so nothing on it should
be waiting (README, "Contributing").

### If the release fails

The workflow stops before tagging when the top section of `CHANGELOG.md` is
not the version it computed: no tag exists and nothing was published. That is
a release-prep merge request that was skipped, or a candidate that was
squashed. Fix it on `main`, prepare the release again and merge into `release`
again.

If the workflow fails after the tag (GoReleaser, signing, GitHub), the tag
exists but the release is incomplete. When the cause is outside the repository
(a secret, the signing key, GitHub itself), fix it and re-run the failed job
from the workflow run on GitHub. When the cause is in the repository
(`.goreleaser.yml`, a release script, the workflow itself), a re-run cannot
pick the fix up: a maintainer deletes the tag on GitHub, and the release if
GoReleaser created one, the fix merges on `main` through a normal merge
request, and the candidate is cut by hand from the new `main` (see "By hand").
The top section is still that version and the workflow counts versions from
the tags, so it tags the same version again. Pushing the fix to `release` with
the tag in place does nothing useful: the workflow computes the next patch and
stops at the changelog check. (v1.7.1 predates the changelog and went the
other way: its fix shipped as v1.7.2, and the tag stayed with no release
behind it.)

On the GitLab side, `wait_for_release` gives up after 30 minutes and fails the
pipeline with the command to run instead. Once the GitHub release carries
`release-manifest.json`, retry the job, or run a pipeline on the docs repo's
`main` with `RELEASE_ARTIFACT=terraform` and `RELEASE_VERSION=vX.Y.Z`, which
does the same thing as `notify_docs`. If only `notify_docs` failed, the
release is out and the pipeline shows a warning; fix the cause on the docs side
and retry the job, or run that pipeline.

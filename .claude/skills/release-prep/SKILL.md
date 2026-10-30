---
name: release-prep
description: Use when cutting or preparing a release of this provider — rendering the CHANGELOG section with `make changelog`, bumping versions.env, opening the release-prep MR into main, or the main→release MR. Triggers include "release X.Y", "cut a release", "prep the release", "update the changelog for the release", "bump the version".
---

# Release Prep

Two MRs, in order: **(1)** a prep commit into `main`, **(2)** `main → release` (never squashed). Policy: CLAUDE.md "Changelog and Versioning" + readme "Versioning" / "Contributing" / "Maintaining Changelog". This is the operational checklist.

## 1. Establish the delta

```bash
git fetch origin
git fetch https://github.com/crusoecloud/terraform-provider-crusoe.git '+refs/tags/v*:refs/tags/v*'   # release tags live on GitHub, not on GitLab
git log --oneline origin/release..origin/main   # exactly what ships — every customer-visible commit needs a fragment in .changelog/
make changelog-preview                           # what the new section will say; a missing change means a missing fragment
git log --oneline origin/main..origin/release   # must be EMPTY; commits only on release = a past release MR was squashed (see 17ec31d) — reconcile before anything else
```

## 2. Decide the bump

- New resource / data source / attribute → **minor**. Fixes, docs, refactors only → **patch**. Breaking → **major** (+ `UPGRADE NOTES`).
- **Patch: CHANGELOG only — do NOT touch `versions.env`.** `scripts/tag_semver.sh` auto-increments the patch digit from existing tags (precedent: `f102f58`, 1.3.1).
- Minor/major: bump `versions.env` in the same prep commit.
- `git tag -l 'vX.Y.*'` — confirm the tag the pipeline will mint is the one you expect.

## 3. Prep commit (branch off `origin/main`)

- `CHANGELOG.md`: `make changelog` (patch) or `make changelog VERSION=vX.Y.0` (minor/major) renders the fragments merged since the last tag into a new `## vX.Y.Z (date)` section at the top. Never write the section by hand: a wrong or missing entry is fixed in its fragment (a missing one gets a fragment of its own, named after the original MR, merged into `main` first), then render again; until the tag exists the command replaces the section.
- **Deprecation sweep** (precedent: `e3707ea`): `grep -rn 'FormatDeprecationWithReplacement' internal/` — any message naming an already-shipped version was written against a release that didn't include it; correct it to X.Y.Z. Data-source copies carry the notice in `MarkdownDescription` only, never `DeprecationMessage` (1.3.1 lesson: attribute deprecations propagate to whole-object references).
- `grep -n replace go.mod` — must be empty. Lint allow-lists client-go replaces, so CI will NOT catch a preview-SDK pin reaching a release.
- `make docs` if any description changed; commit the regenerated files. `make precommit`.
- Scope: `CHANGELOG.md`, `versions.env`, deprecation-string corrections, regenerated docs — nothing else.
- Commit/MR title: `Release vX.Y.Z` (+ `and correct deprecation versions` when applicable). MR into `main`; squash is fine here.

## 4. Release MR

1. Wait for the prep MR to merge and `main`'s pipeline to go green. **Freeze `main` until the release ships** — anything merging now is missing from the changelog.
2. Open `main → release`, title `Release vX.Y.Z`. **Squash OFF, merge commit, never delete the source branch.** No file edits in this MR.
3. On merge: the GitHub semver-tag workflow tags `vX.Y.Z`, goreleaser publishes to the registry, announcement lands in `#ccx-ci-releases`.

## 5. Verify

- Tag points at the release merge commit; registry lists X.Y.Z; Slack post arrived (silence may just be an unset webhook — confirm in the GitLab UI).
- `git log origin/main..origin/release` still empty → the next release is safe.

## Red flags — stop

- Touching `versions.env` for a patch release.
- Squashing (or rebasing) the `main → release` MR. The scar: `56a4a5e` (squashed 1.1.0) needed `17ec31d` to reconcile.
- Editing `CHANGELOG.md`/`versions.env` in any commit other than the prep commit, or on the `release` branch.
- Writing or editing a `CHANGELOG.md` section by hand instead of fixing the fragment and running `make changelog`.
- A `replace` directive in `go.mod`.
- A deprecation message naming a version that already shipped.

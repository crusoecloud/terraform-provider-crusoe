# Changelog fragments

`CHANGELOG.md` and the GitHub release notes are generated from the files in this
directory with [go-changelog](https://github.com/hashicorp/go-changelog). Add one
file per merge request, named after the MR number: `.changelog/<MR>.txt` (the
number in the MR URL, so `!123` becomes `123.txt`).

CI (`changelog_check`) fails the pipeline of a merge request without one, unless
it carries the label `skip-changelog` (or `dependencies`), comes from Renovate,
or only changes files no customer can notice: Markdown files, `docs/`,
`examples/`, `templates/`, `.gitlab-ci.yml`, `.gitlab/`, `.github/`, `.claude/`,
`.gitignore`, `.golangci.yml`, `mise.toml`, `versions.env`, Go test files,
Terraform test files (`*.tftest.hcl`) and `testdata/`. Those are skipped
automatically, with the file list in the job log.

## When is an entry needed

For anything a customer can notice: a new resource, data source or attribute, a
changed default, a changed plan or apply behaviour, a fixed bug, a security fix,
a deprecation, a breaking change. Not for refactors, build or release tooling,
or dependency bumps with no visible effect; add the `skip-changelog` label for
those. Tests, docs, examples and CI config need nothing: CI skips the check when
a merge request touches only those.

### Changes behind a feature flag

A change that only customers with a feature flag can reach gets no entry while
the flag is off for customers: add the `skip-changelog` label. When a merge
request mixes the two, the fragment covers only the part every customer sees.

The feature is announced once, when it becomes generally available, with a
single `feature` entry that describes it as it is then, not the renames and
fixes it went through behind the flag. The flag is turned on server-side, which
cuts no provider release, so the entry goes in:

- the merge request that removes the flag check from the provider, when there
  is one;
- otherwise, a small merge request of its own, merged before the flag is turned
  on for everyone, that says which provider version is needed ("requires
  provider vX.Y.Z or later").

Make the fragment part of the general-availability ticket so it is not
forgotten.

## Format

One or more fenced blocks, each tagged with a type. The fence must start at
column 0. The examples below are indented on purpose so that this README is not
parsed as an entry.

    ```release-note:feature
    `crusoe_load_balancer` manages external load balancers.
    ```

Write for a customer, in one or two sentences: what the provider does now and
when it applies, in the present tense, with resources, attributes and values in
backticks. Not what the team did, and not how it was implemented. The same text
goes to `CHANGELOG.md`, to the GitHub release notes and, with a product tag, to
the body of the Crusoe Updates entry, so it has to read well on the site.

A merge request with several changes uses several blocks in the same file:

    ```release-note:enhancement
    `crusoe_vpc_firewall_rule` takes `source_port_ranges` and `destination_port_ranges`, lists of single ports, port ranges or `*`.
    ```

    ```release-note:bug
    `crusoe_kubernetes_node_pool` no longer forces replacement on the first plan after an import.
    ```

## Types

| Type | Section in CHANGELOG.md | Use for |
|---|---|---|
| `breaking-change` | BREAKING CHANGES | Users must change configuration or state to keep working |
| `security` | SECURITY | Security fixes |
| `feature` | FEATURES | New resources, data sources or capabilities |
| `enhancement` | ENHANCEMENTS | New attributes, better plans, changed defaults |
| `bug` | BUG FIXES | Something that was wrong and is fixed |
| `deprecation` | DEPRECATIONS | Still works, will be removed later |
| `note` | NOTES | Anything else users should know |

The sections written by hand up to 1.8.0 used NEW FEATURES and UPGRADE NOTES
for what is `feature` and `breaking-change` here.

## Product tag and title for Crusoe Updates

Entries that may appear on [Crusoe Updates](https://docs.crusoecloud.com/updates/)
also name the product they touch, with an HTML comment inside the block. The
comment is invisible in the rendered changelog and in the GitHub release notes.

    ```release-note:feature
    `crusoe_load_balancer` manages external load balancers.
    <!-- site: infrastructure-cloud/networking -->
    <!-- title: Manage external load balancers with the Terraform provider -->
    ```

The value is `<category>/<subcategory>` from the site's
[`changelogConfig.json`](https://gitlab.com/crusoeenergy/island/external/docs/-/blob/main/src/data/changelogConfig.json)
(`platforms.<category>.subcategories.<subcategory>`), for example
`infrastructure-cloud/networking`, `security/iam` or `general/api`. A change to
the provider itself that touches no product, such as its configuration or
authentication, is tagged `developer-tools/terraform`. There is no copy of that
list here: the check reads the file from the docs repo and prints the valid
values when a tag is wrong. If the file cannot be read, the check reports the
tag as not validated and passes; the docs side validates again.

`feature`, `security` and `breaking-change` entries must carry a tag, or opt
out explicitly when the change is not worth a site entry:

    ```release-note:feature
    The provider is also published for Linux on IBM Z (`linux_s390x`).
    <!-- site: none -->
    ```

For the other types the tag is optional: add one when a bug fix or an
enhancement deserves a site entry. Leave it out, or write
`<!-- site: none -->`, and the entry goes only to `CHANGELOG.md` and the GitHub
release notes, not to Crusoe Updates.

An entry with a product tag also carries the title of its site entry, in a
second comment: `<!-- title: ... -->`. One line, at most 70 characters, no
trailing period, written like the titles on Crusoe Updates ("Manage external
load balancers with the Terraform provider"): say the outcome, name the
provider or Terraform when the product alone would not make it obvious, and do
not start with "Added" or "Fixed", the badge already says that. The check
requires it whenever there is a tag other than `none`. The text itself goes to
the site as the entry body; see "Format" above for how to write it.

`make changelog-tags` lists the valid product tags.

At release time the tag ends up in `release-manifest.json`
(`make release-manifest`), attached to the GitHub release, from which the docs
repo drafts a site entry for each tagged line and opens a merge request for the
docs team.

## Writing a fragment with Claude Code

    /changelog            # or: /changelog 123, when the branch has no merge request yet

The command (`.claude/commands/changelog.md`) reads this file, looks at the
branch's diff and merge request, picks the type and the product tag from the
live list, writes the text and the title in the site's voice, saves
`.changelog/<MR>.txt` and runs the check. It asks when it cannot tell whether
the change is customer-visible or which product it touches. Read what it wrote
before you push: the author owns the fragment.

## Checking a fragment locally

    make changelog-check FILE=.changelog/123.txt

## How it becomes a changelog

`make changelog` on the release-prep branch (see [`README.md`](../README.md),
"Contributing") renders every fragment added since the last release tag with
`changelog.tmpl` and `note.tmpl` into a new section at the top of
`CHANGELOG.md`. The release then publishes that section as the GitHub release
notes.

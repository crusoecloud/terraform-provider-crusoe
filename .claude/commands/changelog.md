---
description: Draft the changelog fragment for the current branch (.changelog/<MR>.txt) — type, customer-facing text, product tag and site title per .changelog/README.md — and validate it with make changelog-check.
argument-hint: [MR number] [what changed, when the diff does not say]
allowed-tools: Read, Grep, Glob, Bash, Write, Edit
---

Draft or update the changelog fragment for the merge request of the current branch.

The user invoked this command with: $ARGUMENTS

## Steps

1. **Read the rules first.** `.changelog/README.md` describes the format, the types, the product tag and the title. Follow it over anything you remember.

2. **Find the merge request number.** The file is `.changelog/<MR IID>.txt`. Use the number in `$ARGUMENTS` if there is one; otherwise run `glab mr view --output json 2>/dev/null | jq -r .iid` for the current branch. If there is no merge request yet, stop and ask the user to open it or to give you the number. The file name depends on it; never guess it.

3. **Understand the change.** Run `git fetch -q origin main`, then `git diff origin/main...HEAD --stat`, the diff of the non-test Go files (`git diff origin/main...HEAD -- '*.go' ':!*_test.go'`), the diff of the generated docs (`git diff origin/main...HEAD -- docs/`, which shows schema changes as the customer sees them), the commit messages, and `glab mr view` for the title and description. Then decide:
   - **Is anything customer-visible?** Refactors, CI, tests, docs, examples and dependency bumps get no fragment. Say so and suggest the `skip-changelog` label instead. Never write a filler fragment.
   - **Type.** `feature` for a new resource, data source or capability; `enhancement` when an existing one does more or better, or gets a new attribute; `bug`; `security`; `deprecation`; `breaking-change` when users must change their configuration or state to keep working (an attribute removed or renamed without a deprecation, a changed config key, a forced replacement that did not exist before); `note` for anything else worth telling.
   - **Product.** The product the change touches, as `<category>/<subcategory>` from `make changelog-tags` (the live list from the docs repo). A change to the provider itself (configuration, authentication, builds) is `developer-tools/terraform`. `feature`, `security` and `breaking-change` must carry a tag or say `<!-- site: none -->`; use `none` when the change is real but not worth an entry on Crusoe Updates (a small attribute, a plan message). For the other types, add a tag only when the change deserves a site entry.

4. **Write the text for a customer, not for the team.** One or two sentences: what the provider does now and when it applies. Resources, attributes and values in backticks, present tense ("`crusoe_load_balancer` takes `vpc_id`"), no ticket numbers, no implementation details, no "in this MR". A breaking change also says what the user has to do. The text goes verbatim to `CHANGELOG.md`, to the GitHub release notes and, with a tag, to the body of the Crusoe Updates entry (the docs side appends a link to the release). If a page on docs.crusoecloud.com covers the feature, end with `See [Page title](https://docs.crusoecloud.com/<path>).` using the page's real title, and check with `curl -sI` that the URL answers 200. One link at most; none is fine.

5. **Write the title** (required with a product tag). One line, at most 70 characters, no trailing period, in the style of the titles on Crusoe Updates: the outcome, not the mechanism ("Manage external load balancers with the Terraform provider"), naming the provider or Terraform when the product alone would not make it obvious. Do not start with "Added" or "Fixed"; the badge on the site says that.

6. **Write the file and validate it.** Exactly this shape, comments after the text:

   ~~~
   ```release-note:<type>
   <text>
   <!-- site: <category>/<subcategory> -->
   <!-- title: <title> -->
   ```
   ~~~

   Then run `make changelog-check FILE=.changelog/<MR>.txt` and fix whatever it reports. If the fragment already exists, update it; add a second block only when the merge request has two independent customer-visible changes.

7. **Show the result** and say what you were unsure about: the product, whether the change deserves a site entry, the wording of the title. The author decides; the reviewer sees the fragment in the diff.

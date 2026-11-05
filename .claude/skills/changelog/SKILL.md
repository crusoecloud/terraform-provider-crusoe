---
name: changelog
description: Draft the changelog fragment of the current branch, .changelog/<MR>.txt, per .changelog/README.md - type, customer-facing text, product tag and site title - and validate it with make changelog-check. Use before opening a merge request with a customer-visible change, when the user asks for a fragment, a release note or a changelog entry, or when a fragment needs rewording. Tells a change behind a server-side flag from one every customer sees, and asks before deciding what the diff cannot answer.
argument-hint: [MR number] [what changed, when the diff does not say]
allowed-tools: Read, Grep, Glob, Bash, Write, Edit, AskUserQuestion
---

Draft or update the changelog fragment for the merge request of the current branch, and settle with the author what the diff cannot settle.

The user invoked this skill with: $ARGUMENTS

## Steps

1. **Read the rules first.** `.changelog/README.md` describes the format, the types, the product tag and the title. Follow it over anything you remember.

2. **Find the merge request number.** The file is `.changelog/<MR IID>.txt`. Use the number in `$ARGUMENTS` if there is one; otherwise run `glab mr view --output json 2>/dev/null | jq -r .iid` for the current branch. If there is no merge request yet, stop and ask the user to open it or to give you the number. The file name depends on it; never guess it.

3. **Understand the change.** Run `git fetch -q origin main`, then `git diff origin/main...HEAD --stat`, the diff of the non-test Go files (`git diff origin/main...HEAD -- '*.go' ':!*_test.go'`), the diff of the generated docs (`git diff origin/main...HEAD -- docs/`, which shows schema changes as the customer sees them), the commit messages, and `glab mr view` for the title and description. Then work out, without asking yet:
   - **Is it behind a server-side flag?** The provider has no flags of its own; the backend enables some features per organization, and the diff shows it: a description that starts with `common.LimitedAvailabilityMessage`, a `common.NewDevelopmentWarning*Modifier` on the attribute, or `common.DevelopmentMessage` for what cannot be enabled on request yet. A change that **adds** such a marker gets no fragment while the flag is off for everyone else: say so, suggest the `skip-changelog` label, and remind the author that the entry belongs to the general-availability ticket. A change that **removes** the marker is the general-availability merge request: one `feature` entry that describes the feature as it is now, not its history behind the flag, and says which provider version is needed ("requires provider vX.Y.Z or later"), since the flag is turned on server-side and cuts no release. A merge request that mixes a flagged part and a part every customer sees gets a fragment for the visible part only.
   - **Is anything customer-visible?** Refactors, CI, tests, docs, examples and dependency bumps get no fragment. Say so and suggest the `skip-changelog` label instead. Never write a filler fragment.
   - **Type.** `feature` for a new resource, data source or capability; `enhancement` when an existing one does more or better, or gets a new attribute; `bug`; `security`; `deprecation`; `breaking-change` when users must change their configuration or state to keep working (an attribute removed or renamed without a deprecation, a changed config key, a forced replacement that did not exist before); `note` for anything else worth telling.
   - **Product.** The product the change touches, as `<category>/<subcategory>` from `make changelog-tags` (the live list from the docs repo). A change to the provider itself (configuration, authentication, builds) is `developer-tools/terraform`. `feature`, `security` and `breaking-change` must carry a tag or say `<!-- site: none -->`; use `none` when the change is real but not worth an entry on Crusoe Updates (a small attribute, a plan message). For the other types, add a tag only when the change deserves a site entry.

4. **Ask before writing what the diff cannot answer.** One `AskUserQuestion` call with only the questions that apply, your recommendation as the first option and the reason for it in that option's description. Do not ask what the diff already says (whether the resource is new, which attribute changed, whether a marker was added or removed), and do not ask about the text: show it in step 8 and let the author edit the file. Worth asking:
   - **Fragment or `skip-changelog`**, only when the diff leaves it open: a changed behaviour on an internal path, an effect you cannot name from the customer's side.
   - **Which part the fragment covers**, when the merge request mixes a flagged part and a visible one.
   - **Crusoe Updates or changelog only**, for a `bug` or an `enhancement`. Recommend the site when customers reported the problem, worked around it in their configuration, or have to change something; recommend the changelog alone for a fix few would meet or a small attribute.
   - **Which product**, when two tags fit. The product the customer manages wins over `developer-tools/terraform`, which is for the provider's own configuration, authentication and builds.
   - **Which title**, two or three options when the entry goes to the site, at least one naming Terraform or the provider.

   Wait for the answers: they set the type, the tag and the title below.

5. **Write the text for a customer, not for the team.** One or two sentences: what the provider does now and when it applies. Resources, attributes and values in backticks, present tense ("`crusoe_load_balancer` takes `vpc_id`"), no ticket numbers, no implementation details, no "in this MR". A breaking change also says what the user has to do; a fix that customers worked around says the workaround can go. The text goes verbatim to `CHANGELOG.md`, to the GitHub release notes and, with a tag, to the body of the Crusoe Updates entry (the docs side appends a link to the release). If a page on docs.crusoe.ai covers the feature, end with `See [Page title](https://docs.crusoe.ai/<path>).` using the page's real title, and check with `curl -sI` that the URL answers 200. One link at most; none is fine.

   Cut the mechanism. Too long: "Replacement now depends on which add-ons are set, not on their order: an order-only difference plans as an in-place update that settles after a single apply, with no API call." Enough: "Only adding or removing an add-on replaces the cluster now; an order-only difference plans as one in-place change that settles after a single apply."

6. **Write the title** (required with a product tag). One line, at most 70 characters, no trailing period, in the style of the titles on Crusoe Updates: the outcome, not the mechanism ("Manage external load balancers with the Terraform provider"), naming the provider or Terraform when the product alone would not make it obvious. Do not start with "Added" or "Fixed"; the badge on the site says that.

7. **Write the file and validate it.** Exactly this shape, comments after the text:

   ~~~
   ```release-note:<type>
   <text>
   <!-- site: <category>/<subcategory> -->
   <!-- title: <title> -->
   ```
   ~~~

   Then run `make changelog-check FILE=.changelog/<MR>.txt` and fix whatever it reports. If the fragment already exists, update it; add a second block only when the merge request has two independent customer-visible changes.

8. **Show the result** and the decisions behind it, yours and the author's: the type, the product, site entry or not, and what you are still unsure about. The author owns the fragment and edits the file. Once the branch is pushed, suggest `@crusoe-bot /review terraform` on the merge request: its changelog agent checks the text against the code.

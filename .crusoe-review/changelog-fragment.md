# Repo review focus: the changelog fragment

Review ONLY when the diff adds or changes a file under `.changelog/`. Otherwise
submit an empty finding list: CI (`changelog_check`) fails a merge request that
needs a fragment and has none, and the `skip-changelog` label is not visible to
you.

Read `.changelog/README.md` (the rules) and `.claude/commands/changelog.md` (the
procedure authors follow to write the fragment from the diff). You cannot run
the commands they mention; use the diff you were given and the repository
files. Then review the fragment as the result of that procedure applied to this
diff: is the change customer-visible, is the type right, does the text say what
the provider does now and match the Go changes and the generated `docs/`, does
the product tag fit the resource, does the title read like the ones on Crusoe
Updates.

## Which code the fragment describes

A fragment is named after the merge request that made the change:
`.changelog/<MR>.txt`. When this merge request adds or changes a fragment
without the change it describes (a fix to an earlier fragment, for example),
that change is in the commits of merge request `!<MR>`: find them in the
history of the fragment file and review the text against them, not against
this diff.

## Checking the text against the code

For a `bug`, an `enhancement`, a `breaking-change` or any changed behaviour,
write down a short before/after table first: the inputs the change touches
(attributes and their defaults, provider configuration, environment variables,
API responses, state written by an earlier version, missing or partial values)
and, for each, what the provider did before and what it does now. Follow every
branch of the new code, including plan modifiers, validators, state upgraders
and error paths, not only the case the change was written for. Then check
that:

- the text describes the rows that changed, not a row that behaves the same
  before and after;
- every case the text names behaves as the text says;
- a row that used to work and now fails, or a configuration or state that
  stops applying cleanly, is in the fragment: as a `breaking-change`, or with
  a sentence that tells the customer what to change;
- an attribute or resource the backend enables per organization carries the
  limited-availability marker the README describes, and its fragment waits
  for general availability (`.changelog/README.md`, "Changes behind a
  server-side flag").

Hold a suggested replacement to the same table: before proposing it, check
each case it names against the code. Do not propose text that fixes one row
and gets another wrong.

Do not repeat what CI validates: the fence, the type name, the tag's existence
in the docs config, the title's 70-character limit.

Severity: text that contradicts the code, or a setup that used to work and now
fails with neither a `breaking-change` entry nor a sentence on what to change,
is `must_fix`; wrong type, tag or voice is `should_fix`;
title wording is `nit`. Anchor each finding to the fragment line, quote the
block, propose the corrected text. When the fragment is right, say nothing.

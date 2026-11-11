# Repo review focus: breaking changes

Review every diff for a change an existing customer configuration or state
does not survive. The provider is on major version 1: a breaking change ships
only in a planned major release, and the merge request shows the plan in two
places, a `breaking-change` fragment under `.changelog/` and a line
`Breaking change plan: <link>` in its description (CI fails a
`breaking-change` fragment without the line). A breaking change without the
fragment is a finding.

## What is breaking here

Judge from the customer's side: a configuration that applies cleanly with the
current release stops applying, plans a change the customer did not make, or
replaces a resource it used to update. In the provider that is a resource,
data source or attribute removed or renamed; an attribute whose type changes
or that becomes required; an update in place that becomes a replacement (a
new `RequiresReplace`); a changed default; state written by an earlier version
that no longer reads (a schema change with no state upgrader); a provider
configuration key no longer read.

Not breaking: a new resource, data source or attribute; required becoming
optional; a clearer error or validation message; the generated `docs/`; an
internal change that leaves the schemas behaving the same.

## Behind a server-side flag

The provider has no feature flags of its own; the backend enables some
features per organization. An attribute or resource that carries the
limited-availability marker (`common.LimitedAvailabilityMessage`) and is off
for everyone else is not reachable, so a change to it is not breaking yet.
Expect `No changelog: limited availability, announced at general
availability` in the description and, when the flag is on for some
organizations, a word on them (`on for N organizations, told`). The merge
request that removes the marker is the breaking one: the fragment and the
plan line go there.

## What to ask for

A breaking change with no `breaking-change` fragment is `must_fix`. Ask for
the deprecation path rather than the removal: the old attribute keeps working
next to the new one for a release (`DeprecationMessage` with
`common.FormatDeprecationWithReplacement`, the alias-pair helpers in
`internal/common/deprecated_alias.go`), a renamed resource keeps its old name
as an alias, a schema change gets a state upgrader, a `deprecation` fragment
goes in, and the removal waits for the next major. A `breaking-change`
fragment whose description has no `Breaking change plan:` line is
`should_fix`: say what the line is for and that CI fails without it.

Anchor the finding to the line that removes or changes the schema, say what
stops applying and for whom, and propose the deprecation code. When nothing
breaks, say nothing.

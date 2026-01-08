## MR Title

```
JIRA-TICKET: Short description of the change
```

<!-- Keep under 72 characters. Example: CCX-1641: Refactor instance_group resource and data source -->

---

## TL;DR

<!-- Brief 1-2 sentence summary of major changes -->

### Breaking Changes

<!-- If no breaking changes, replace table with "None" -->

| Field | Before | After |
|-------|--------|-------|
| `field_name` | Old behavior | New behavior |

### Key Additions
<!-- Bullet list of key additions -->
- Addition 1
- Addition 2

---

## Table of Contents
- [Change Description](#change-description)
- [Linked JIRA Issue](#linked-jira-issue)
- [Related / Blocking Changes](#related--blocking-changes)
- [Testing Done](#testing-done)
- [Risks / Follow Ups](#risks--follow-ups--relevant-subsequent-tickets)
- [AI Code Generation](#ai-code-generation)
- [Recommended Prompt](#recommended-prompt-for-similar-changes)

---

## Change Description

<!-- Organize changes by category. Remove sections that don't apply. -->

### 1. Implementation Changes (`internal/<package>/`)

**Resource (`<resource>_resource.go`):**
- Change 1
- Change 2

**Data Source (`<resource>_data_source.go`):**
- Change 1

**State Upgrades (`<resource>_resource_upgrade.go`):**
- Schema version bump (e.g., v0 → v1)
- Field mappings (e.g., `old_field` → `new_field`)

**Utilities (`util.go`):**
- Shared description constants
- Mapping functions (API → Terraform model)
- Helper functions

### 2. Go Unit Tests (`internal/<package>/*_test.go`)

- `<resource>_resource_test.go` - Tests resource metadata.
- `<resource>_data_source_test.go` - Tests data source metadata.
- `util_test.go` - Tests schema consistency, API-to-model mapping functions, and shared utilities.

### 3. Terraform Tests (`examples/<resource>/tests/`)

**Unit Tests (`unit.tftest.hcl`):**
- Test scenarios

**Integration Tests (`integration.tftest.hcl`):**
- Test scenarios

### 4. Documentation (`examples/resources/<resource>/`)

- `resource.tf` - Example configuration for docs generation
- `import.sh` - Import command example

---

## Linked JIRA Issue

[JIRA-TICKET](https://crusoecloud.atlassian.net/browse/JIRA-TICKET)

---

## Related / Blocking Changes

<!-- MRs related to this change, or "None" -->

None

---

## Testing Done

### Go Unit Tests
```bash
make test
# Results summary
```

### Terraform Unit Tests (plan-only)
```bash
cd examples/<resource>
terraform test -filter=tests/unit.tftest.hcl
# Results summary
```

### Terraform Integration Tests (requires credentials)
```bash
cd examples/<resource>
terraform test -filter=tests/integration.tftest.hcl
# Results summary
```

### Linting
```bash
make lint
# Results summary
```

---

## Risks / Follow Ups / Relevant subsequent tickets

### Risks
- Risk 1
- Risk 2

### Follow Ups
- [ ] Follow up task 1
- [ ] Follow up task 2

---

## AI Code Generation

<!--
Describe how AI tools contributed to the CODE CHANGES in this MR (not the MR description).
If no AI tools were used for code changes, replace with "None".

Examples of AI contributions to document:
- Code refactoring or implementation
- Test generation
- Documentation updates
- Bug fixes or improvements suggested by AI

All AI-generated code should be reviewed and tested before committing.
-->

This MR was developed with assistance from Claude Code.

- **Tool**: Claude Code CLI
- **Model**: claude-opus-4-5-20251101
- **Contributions**:
  - Implementation of resource and data source changes
  - State upgrade logic
  - Unit test generation
  - Documentation updates
  - Commit message drafting

All AI-generated code was reviewed and tested before committing.

---

Closes JIRA-TICKET

---

## Recommended Prompt for Similar Changes

<!--
If starting this work from scratch, what prompt would achieve the final state efficiently?
This helps future developers replicate similar changes quickly.
-->

```
Prompt goes here...
```

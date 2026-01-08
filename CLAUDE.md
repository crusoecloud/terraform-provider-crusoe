# CLAUDE.md

This file provides guidance to Claude Code when working with this Terraform provider.

## TL;DR

- **Build**: `make build` | **Test + Lint**: `make dev` | **Pre-commit**: `make precommit`
- **Breaking changes**: Avoid for released resources; use deprecation for field renames (see [Breaking Changes Policy](#breaking-changes-policy))
- **Resource packages** in `internal/<resource>/` with `_resource.go`, `_data_source.go`, `*_test.go`, `util.go`
- **Examples & tests** in `examples/<resource>/` with `main.tf` and `tests/*.tftest.hcl`
- **ID fields**: Always suffix with `ID` (e.g., `ProjectID`, `InstanceTemplateID`)
- **API errors**: Use `common.UnpackAPIError(err)`, not `err.Error()`
- **State extraction**: Use `getResourceModel()` helper pattern
- **Schema validators**: Add `stringvalidator`/`int64validator` directly in schema
- **Testing validators**: Go unit tests (not Terraform `expect_failures`) for schema validators
- **Reference implementation**: `internal/instance_group/` follows all current patterns

## Table of Contents

- [CLAUDE.md](#claudemd)
  - [TL;DR](#tldr)
  - [Table of Contents](#table-of-contents)
  - [Repository Overview](#repository-overview)
  - [Build \& Test Commands](#build--test-commands)
  - [Project Structure](#project-structure)
  - [Breaking Changes Policy](#breaking-changes-policy)
    - [Major Version Release Strategy](#major-version-release-strategy)
  - [Terraform Provider Patterns](#terraform-provider-patterns)
    - [Resource Structure](#resource-structure)
    - [Naming Conventions](#naming-conventions)
    - [State Extraction Helper](#state-extraction-helper)
    - [Error Handling](#error-handling)
    - [Schema Descriptions](#schema-descriptions)
    - [Deprecated Fields](#deprecated-fields)
    - [HTTP Response Handling](#http-response-handling)
    - [State Upgrades](#state-upgrades)
    - [Schema Validators](#schema-validators)
  - [Testing](#testing)
    - [Go Unit Tests](#go-unit-tests)
    - [Terraform Unit Tests](#terraform-unit-tests)
    - [Terraform Integration Tests](#terraform-integration-tests)
  - [Code Style](#code-style)
    - [Common Lint Errors](#common-lint-errors)
  - [Creating Merge Request Descriptions](#creating-merge-request-descriptions)
    - [MR Template](#mr-template)
    - [Generating an MR Description](#generating-an-mr-description)
    - [Output Location](#output-location)

## Repository Overview

This is the Crusoe Cloud Terraform Provider, enabling infrastructure-as-code management of Crusoe Cloud resources.

## Build & Test Commands

```bash
make build          # Build the provider
make dev            # Run tests + lint
make test           # Run tests only
make lint           # Run golangci-lint only
make precommit      # Run tests + lint with auto-fix
make docs           # Generate documentation
```

## Project Structure

```
internal/
├── common/           # Shared utilities (API client, helpers, validators)
├── <resource>/       # Resource packages (vm, disk, vpc_network, etc.)
│   ├── <resource>_resource.go           # CRUD operations
│   ├── <resource>_resource_test.go      # Resource schema and mapping tests
│   ├── <resource>_data_source.go        # Read-only data source
│   ├── <resource>_data_source_test.go   # Data source schema and mapping tests
│   ├── <resource>_resource_upgrade.go   # State migrations (if needed)
│   ├── util.go                          # Package-specific helpers, shared descriptions
│   └── util_test.go                     # Schema consistency tests

examples/
├── <resource>/
│   ├── main.tf                        # Example configuration
│   └── tests/
│       ├── unit.tftest.hcl            # Plan-only validation tests
│       └── integration.tftest.hcl     # Apply/destroy tests
```

## Breaking Changes Policy

**Avoid breaking changes for released resources.** Most resources in this provider are publicly released and used by customers. Breaking their Terraform configurations on upgrade causes significant disruption.

1. **Field Renames**: Use deprecation, not removal

   ```go
   // Old field - mark deprecated but keep functional
   "instance_template": schema.StringAttribute{
       Optional:           true,
       Computed:           true,
       DeprecationMessage: common.FormatDeprecationWithReplacement("v0.6.0", "instance_template_id"),
   },
   // New field - add alongside old field
   "instance_template_id": schema.StringAttribute{
       Optional: true,
       Computed: true,
   },
   ```

2. **Field Removal**: Only after deprecation period (minimum one minor version)

3. **Behavior Changes**: Must be backwards compatible or behind a new field

### Major Version Release Strategy

Once all resources have been migrated to new patterns with proper deprecations:

1. Announce deprecation timeline to customers
2. Release major version (v1.0.0 or v2.0.0) that removes deprecated fields
3. Document migration guide for customers

## Terraform Provider Patterns

> **Note:** The patterns below are recommended standards being rolled out. They were first applied to `internal/instance_group/`. Other packages may still use older patterns and should be updated incrementally **with deprecations** (see Breaking Changes Policy above).

### Resource Structure

Each resource package follows this pattern:

1. **Resource struct** with `*common.CrusoeClient`
2. **Model struct** with `types.String`, `types.Int64`, `types.List` fields using `tfsdk` tags
3. **Schema method** defining attributes with `schema.StringAttribute`, etc.
4. **CRUD methods**: `Create`, `Read`, `Update`, `Delete`

### Naming Conventions

For ID fields, always include `ID` as a suffix in both the struct field name and the `tfsdk` tag:

```go
// Good
InstanceTemplateID types.String `tfsdk:"instance_template_id"`
ProjectID          types.String `tfsdk:"project_id"`

// Bad
InstanceTemplate   types.String `tfsdk:"instance_template"`
Template           types.String `tfsdk:"template_id"`
```

### State Extraction Helper

Use `getResourceModel()` to extract state/plan with error handling:

```go
var errGetResourceModel = errors.New("unable to get resource model")

func getResourceModel(ctx context.Context, source tfDataGetter, dest *myResourceModel, respDiags *diag.Diagnostics) error {
    diags := source.Get(ctx, dest)
    respDiags.Append(diags...)

    if respDiags.HasError() {
        return errGetResourceModel
    }

    return nil
}

// Usage:
var state myResourceModel
if err := getResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
    return
}
```

### Error Handling

Always use `common.UnpackAPIError(err)` for API errors (not `err.Error()`).

### Schema Descriptions

Extract shared descriptions to constants in `util.go` when the same description is used in both resource and data source schemas.

### Deprecated Fields

See [Breaking Changes Policy](#breaking-changes-policy) for when to use deprecation vs removal.

For released resources requiring deprecation:

- Mark with `DeprecationMessage` using `common.FormatDeprecationWithReplacement()`
- Keep both old and new fields functional during deprecation period
- Handle fallback logic: prefer new field, fall back to old field if new is empty
- Preserve deprecated field values from plan/state (don't overwrite from API)

### HTTP Response Handling

Use `common.ValidateHTTPStatus()` for consistent status code validation.

**Response body cleanup:** Always close response bodies with a nil check before the error check. This ensures bodies are closed even when the API returns both an error and a response:

```go
dataResp, httpResp, err := r.client.APIClient.MyApi.DoSomething(ctx, ...)
if httpResp != nil {
    defer httpResp.Body.Close()
}
if err != nil {
    resp.Diagnostics.AddError(...)
    return
}
```

> **Note:** Most resources currently place `defer httpResp.Body.Close()` after the error check. The pattern above is preferred and `internal/instance_group/` serves as the reference implementation.

### State Upgrades

When schema changes require migrating existing Terraform state, create a `<resource>_resource_upgrade.go` file:

1. **Bump schema version** in the resource:

   ```go
   resp.Schema = schema.Schema{
       Version: 1,  // Increment from 0
   }
   ```

2. **Define prior state model** for the old schema:

   ```go
   type myResourceModelV0 struct {
       ID       types.String `tfsdk:"id"`
       OldField types.String `tfsdk:"old_field"`
   }
   ```

3. **Implement `UpgradeState`** method:

   ```go
   func (r *myResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
       return map[int64]resource.StateUpgrader{
           0: {
               PriorSchema: &schema.Schema{
                   Attributes: map[string]schema.Attribute{
                       "id":        schema.StringAttribute{Computed: true},
                       "old_field": schema.StringAttribute{Required: true},
                   },
               },
               StateUpgrader: upgradeStateV0ToV1,
           },
       }
   }
   ```

4. **Write upgrader function** to map old fields to new:

   ```go
   func upgradeStateV0ToV1(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
       var oldState myResourceModelV0
       resp.Diagnostics.Append(req.State.Get(ctx, &oldState)...)
       if resp.Diagnostics.HasError() {
           return
       }

       newState := myResourceModel{
           ID:       oldState.ID,
           NewField: oldState.OldField,  // Renamed field
       }
       resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
   }
   ```

**Key points:**

- Each upgrader jumps directly to current version (v0→v2, not v0→v1→v2)
- When adding v2, update v0 upgrader to also handle v2 changes
- Set removed/new fields to `types.StringNull()` etc. (populated by Read)
- Reference: `internal/instance_group/instance_group_resource_upgrade.go`

### Schema Validators

Add validators directly in the schema for input constraints:

```go
import (
    "github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
    "github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
    "github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

"name": schema.StringAttribute{
    Required: true,
    Validators: []validator.String{
        stringvalidator.LengthAtLeast(1),
    },
},
"desired_count": schema.Int64Attribute{
    Optional: true,
    Validators: []validator.Int64{
        int64validator.AtLeast(0),
    },
},
```

## Testing

### Go Unit Tests

Separate test files for resource, data source, and shared utilities:

**`<resource>_resource_test.go`** - Resource-specific tests:

- Schema validators are present
- Plan modifiers (RequiresReplace, UseStateForUnknown)
- Required/optional/computed field attributes
- API-to-Terraform model mapping functions
- Resource metadata (type name)

**`<resource>_data_source_test.go`** - Data source-specific tests:

- Schema structure and nested attributes
- All nested fields are computed
- API-to-model mapping functions
- Data source metadata (type name)

**`util_test.go`** - Shared/consistency tests:

- Schema field consistency between resource and data source
- Shared description constants are defined

Example validator test:

```go
func TestInstanceGroupResourceSchema(t *testing.T) {
    ctx := context.Background()
    r := NewInstanceGroupResource()
    schemaResp := &resource.SchemaResponse{}
    r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

    // Type assert to access Validators field
    attr, ok := schemaResp.Schema.Attributes["desired_count"].(schema.Int64Attribute)
    if !ok {
        t.Fatal("desired_count attribute not found")
    }
    if len(attr.Validators) == 0 {
        t.Error("desired_count should have validators")
    }
}
```

### Terraform Unit Tests

Located in `examples/<resource>/tests/unit.tftest.hcl`. Use `command = plan` for validation without creating resources:

```hcl
variables {
  name_prefix = "tf-test-"
  vm_count    = 3
}

run "validate_resource_name" {
  command = plan

  assert {
    condition     = my_resource.name == "${var.name_prefix}resource"
    error_message = "Expected name '${var.name_prefix}resource', got '${my_resource.name}'."
  }
}
```

**Limitations:**

- Cannot test computed values (IDs) at plan time - use integration tests
- Provider schema validators cannot be tested with `expect_failures` - use Go unit tests

### Terraform Integration Tests

Located in `examples/<resource>/tests/integration.tftest.hcl`. Use `command = apply` for full lifecycle testing:

```hcl
run "create_resource" {
  command = apply

  assert {
    condition     = my_resource.id != null
    error_message = "Resource was not created successfully."
  }
}
```

## Code Style

- Follow existing patterns in the codebase
- Run `make precommit` before committing
- Keep nil checks only where necessary (Go's `len()` and `append()` are nil-safe)

### Common Lint Errors

Watch out for these frequently triggered lint errors:

- **nlreturn**: Missing blank line before `return` statements
- **gofumpt**: Using `var x =` instead of `x :=` for short variable declarations
- **gocritic/hugeParam**: Triggered when implementing Terraform Plugin Framework interfaces (e.g., validators) where the signature is fixed. Use `//nolint:gocritic // hugeParam: <param> signature required by <interface>`. Example: `//nolint:gocritic // hugeParam: req signature required by validator.String interface`

## Creating Merge Request Descriptions

Use Claude Code to generate comprehensive MR descriptions based on branch changes.

### MR Template

The MR template is located at `.claude/mr-template.md` and includes sections for:

- MR Title (for GitLab title field, keep under 72 characters)
- Recommended Squashed Commit Message
- TL;DR with breaking changes table
- Change Description
- Testing Done
- Risks / Follow Ups
- AI Code Generation disclosure
- Recommended Prompt for similar changes

### Generating an MR Description

Use this prompt with Claude Code to generate an MR description for the current branch:

```
Generate an MR description for this branch. Use the template at .claude/mr-template.md and:

1. Compare all changes between main and HEAD (git diff main..HEAD, git log main..HEAD)
2. List all files changed with their status (Added/Modified/Deleted)
3. Fill out every section of the template with detailed information
4. Include a breaking changes table if there are any breaking changes
5. Add a files changed summary table in the TL;DR section
6. Write a recommended prompt that would achieve these changes efficiently

Save the output to .claude/MR Output/<branch-name>.md
```

### Output Location

MR descriptions are saved to `.claude/MR Output/` which is gitignored. Copy the content to GitLab when creating your MR:

- Use the **MR Title** section for the GitLab title field
- Use everything from **TL;DR** onwards for the description body

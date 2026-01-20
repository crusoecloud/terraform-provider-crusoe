package common

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ImmutableStringModifier prevents changes to a string attribute with a clear error message.
// This is preferable to RequiresReplace() when you want to block changes entirely
// rather than silently destroying and recreating the resource.
// Use NewImmutableStringModifier to create an instance with proper defaults.
type ImmutableStringModifier struct {
	Summary string
	Message string
}

// NewImmutableStringModifier creates an ImmutableStringModifier with the given summary and message.
// Empty strings will use defaults:
// - Summary: "Immutable Attribute Change Not Allowed"
// - Message: "Cannot change this attribute from %q to %q. This field is immutable."
func NewImmutableStringModifier(summary, message string) ImmutableStringModifier {
	if summary == "" {
		summary = "Immutable Attribute Change Not Allowed"
	}
	if message == "" {
		message = "Cannot change this attribute from %q to %q. This field is immutable."
	}

	return ImmutableStringModifier{Summary: summary, Message: message}
}

func (m ImmutableStringModifier) Description(ctx context.Context) string {
	return "Prevents changes to this attribute."
}

func (m ImmutableStringModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

//nolint:gocritic // hugeParam: req must be passed by value to implement planmodifier.String interface
func (m ImmutableStringModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// Allow during create
	if req.StateValue.IsNull() {
		return
	}

	// Allow if no change
	if req.StateValue.Equal(req.PlanValue) {
		return
	}

	// Error if trying to change immutable attribute
	resp.Diagnostics.AddError(
		m.Summary,
		fmt.Sprintf(m.Message, req.StateValue.ValueString(), req.PlanValue.ValueString()),
	)
}

// PrivateControlPlaneWarningModifier adds a warning when private control plane is enabled.
type PrivateControlPlaneWarningModifier struct{}

// NewPrivateControlPlaneWarningModifier creates a new PrivateControlPlaneWarningModifier.
func NewPrivateControlPlaneWarningModifier() PrivateControlPlaneWarningModifier {
	return PrivateControlPlaneWarningModifier{}
}

func (m PrivateControlPlaneWarningModifier) Description(ctx context.Context) string {
	return "Warns when private control plane is enabled."
}

func (m PrivateControlPlaneWarningModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

//nolint:gocritic // hugeParam: req must be passed by value to implement planmodifier.Bool interface
func (m PrivateControlPlaneWarningModifier) PlanModifyBool(_ context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	// Only warn if private is being set to true
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}

	// Show warning during create (state is null) or when value is changing
	// This appears during both `terraform plan` and the plan display in `terraform apply`
	if req.PlanValue.Equal(types.BoolValue(true)) {
		if req.StateValue.IsNull() || !req.StateValue.Equal(req.PlanValue) {
			resp.Diagnostics.AddWarning(
				"Private Clusters in Limited Availability",
				"Private Clusters require specific account access. To request access, please contact support.",
			)
		}
	}
}

// PrivateNodePoolsWarningModifier adds a warning when private node pool is enabled.
type PrivateNodePoolsWarningModifier struct{}

// NewPrivateNodePoolsWarningModifier creates a new PrivateNodePoolsWarningModifier.
func NewPrivateNodePoolsWarningModifier() PrivateNodePoolsWarningModifier {
	return PrivateNodePoolsWarningModifier{}
}

func (m PrivateNodePoolsWarningModifier) Description(ctx context.Context) string {
	return "Warns when private node pools is enabled."
}

func (m PrivateNodePoolsWarningModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

//nolint:gocritic // hugeParam: req must be passed by value to implement planmodifier.String interface
func (m PrivateNodePoolsWarningModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// Only warn if public_ip_type is being set to "none"
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}

	// Show warning during create (state is null) or when value is changing
	// This appears during both `terraform plan` and the plan display in `terraform apply`
	if req.PlanValue.Equal(types.StringValue("none")) {
		if req.StateValue.IsNull() || !req.StateValue.Equal(req.PlanValue) {
			resp.Diagnostics.AddWarning(
				"Private Node Pools in Limited Availability",
				"Private Node Pools require specific account access. To request access and enable public_ip_type = 'none', please contact support.",
			)
		}
	}
}

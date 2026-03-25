package common

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	defaultDevWarningSummary = "Feature In Development"
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

// DevelopmentWarningInt64Modifier adds a warning when a field that is in development is used.
// The warning only appears when the field is first set or its value changes, not on every plan/refresh.
type DevelopmentWarningInt64Modifier struct {
	Summary string
	Message string
}

// NewDevelopmentWarningInt64Modifier creates a DevelopmentWarningInt64Modifier with the given summary and message.
// Empty strings will use defaults:
// - Summary: "Feature In Development"
// - Message: DevelopmentMessage
func NewDevelopmentWarningInt64Modifier(summary, message string) DevelopmentWarningInt64Modifier {
	if summary == "" {
		summary = defaultDevWarningSummary
	}
	if message == "" {
		message = DevelopmentMessage
	}

	return DevelopmentWarningInt64Modifier{Summary: summary, Message: message}
}

func (m DevelopmentWarningInt64Modifier) Description(_ context.Context) string {
	return "Warns when a feature in development is used."
}

func (m DevelopmentWarningInt64Modifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

//nolint:gocritic // hugeParam: req signature required by planmodifier.Int64 interface
func (m DevelopmentWarningInt64Modifier) PlanModifyInt64(_ context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}

	if req.StateValue.IsNull() || !req.StateValue.Equal(req.PlanValue) {
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("%s: %s", m.Summary, req.Path),
			m.Message,
		)
	}
}

// DevelopmentWarningStringModifier adds a warning when a string field that is in development is used.
// The warning only appears when the field is first set or its value changes, not on every plan/refresh.
type DevelopmentWarningStringModifier struct {
	Summary string
	Message string
}

// NewDevelopmentWarningStringModifier creates a DevelopmentWarningStringModifier with the given summary and message.
// Empty strings will use defaults:
// - Summary: "Feature In Development"
// - Message: DevelopmentMessage
func NewDevelopmentWarningStringModifier(summary, message string) DevelopmentWarningStringModifier {
	if summary == "" {
		summary = defaultDevWarningSummary
	}
	if message == "" {
		message = DevelopmentMessage
	}

	return DevelopmentWarningStringModifier{Summary: summary, Message: message}
}

func (m DevelopmentWarningStringModifier) Description(_ context.Context) string {
	return "Warns when a feature in development is used."
}

func (m DevelopmentWarningStringModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

//nolint:gocritic // hugeParam: req signature required by planmodifier.String interface
func (m DevelopmentWarningStringModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}

	if req.StateValue.IsNull() || !req.StateValue.Equal(req.PlanValue) {
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("%s: %s", m.Summary, req.Path),
			m.Message,
		)
	}
}

// DevelopmentWarningBoolModifier adds a warning when a bool field that is in development is used.
// The warning only appears when the field is first set or its value changes, not on every plan/refresh.
type DevelopmentWarningBoolModifier struct {
	Summary string
	Message string
}

// NewDevelopmentWarningBoolModifier creates a DevelopmentWarningBoolModifier with the given summary and message.
// Empty strings will use defaults:
// - Summary: "Feature In Development"
// - Message: DevelopmentMessage
func NewDevelopmentWarningBoolModifier(summary, message string) DevelopmentWarningBoolModifier {
	if summary == "" {
		summary = defaultDevWarningSummary
	}
	if message == "" {
		message = DevelopmentMessage
	}

	return DevelopmentWarningBoolModifier{Summary: summary, Message: message}
}

func (m DevelopmentWarningBoolModifier) Description(_ context.Context) string {
	return "Warns when a feature in development is used."
}

func (m DevelopmentWarningBoolModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

//nolint:gocritic // hugeParam: req signature required by planmodifier.Bool interface
func (m DevelopmentWarningBoolModifier) PlanModifyBool(_ context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}

	if req.StateValue.IsNull() || !req.StateValue.Equal(req.PlanValue) {
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("%s: %s", m.Summary, req.Path),
			m.Message,
		)
	}
}

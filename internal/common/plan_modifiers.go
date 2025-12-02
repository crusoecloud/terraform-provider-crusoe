package common

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
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

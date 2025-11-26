package common

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestNewImmutableStringModifier_Defaults(t *testing.T) {
	tests := []struct {
		name            string
		summary         string
		message         string
		expectedSummary string
		expectedMessage string
	}{
		{
			name:            "uses defaults when empty strings provided",
			summary:         "",
			message:         "",
			expectedSummary: "Immutable Attribute Change Not Allowed",
			expectedMessage: "Cannot change this attribute from %q to %q. This field is immutable.",
		},
		{
			name:            "uses custom values when provided",
			summary:         "Custom Summary",
			message:         "Custom message from %q to %q",
			expectedSummary: "Custom Summary",
			expectedMessage: "Custom message from %q to %q",
		},
		{
			name:            "uses custom summary with default message",
			summary:         "Custom Summary Only",
			message:         "",
			expectedSummary: "Custom Summary Only",
			expectedMessage: "Cannot change this attribute from %q to %q. This field is immutable.",
		},
		{
			name:            "uses default summary with custom message",
			summary:         "",
			message:         "Custom message only from %q to %q",
			expectedSummary: "Immutable Attribute Change Not Allowed",
			expectedMessage: "Custom message only from %q to %q",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modifier := NewImmutableStringModifier(tt.summary, tt.message)

			if modifier.Summary != tt.expectedSummary {
				t.Errorf("Summary = %q, want %q", modifier.Summary, tt.expectedSummary)
			}
			if modifier.Message != tt.expectedMessage {
				t.Errorf("Message = %q, want %q", modifier.Message, tt.expectedMessage)
			}
		})
	}
}

func TestImmutableStringModifier_Description(t *testing.T) {
	modifier := NewImmutableStringModifier("", "")
	ctx := context.Background()

	desc := modifier.Description(ctx)
	if desc != "Prevents changes to this attribute." {
		t.Errorf("Description() = %q, want %q", desc, "Prevents changes to this attribute.")
	}

	mdDesc := modifier.MarkdownDescription(ctx)
	if mdDesc != desc {
		t.Errorf("MarkdownDescription() = %q, want %q", mdDesc, desc)
	}
}

func TestImmutableStringModifier_PlanModifyString(t *testing.T) {
	tests := []struct {
		name          string
		stateValue    types.String
		planValue     types.String
		expectError   bool
		errorContains string
		customSummary string
		customMessage string
	}{
		{
			name:        "allows create (null state)",
			stateValue:  types.StringNull(),
			planValue:   types.StringValue("1.2.3-cmk.1"),
			expectError: false,
		},
		{
			name:        "allows no change",
			stateValue:  types.StringValue("1.2.3-cmk.1"),
			planValue:   types.StringValue("1.2.3-cmk.1"),
			expectError: false,
		},
		{
			name:          "errors on change with default message",
			stateValue:    types.StringValue("1.2.3-cmk.1"),
			planValue:     types.StringValue("1.2.4-cmk.2"),
			expectError:   true,
			errorContains: "Cannot change this attribute",
		},
		{
			name:          "errors on change with custom message",
			stateValue:    types.StringValue("1.2.3-cmk.1"),
			planValue:     types.StringValue("1.2.4-cmk.2"),
			expectError:   true,
			customSummary: "Kubernetes Version Change Not Supported",
			customMessage: "In-place Kubernetes version upgrades are not supported. Cannot change from %q to %q.",
			errorContains: "In-place Kubernetes version upgrades are not supported",
		},
		{
			name:        "allows unknown plan value during create",
			stateValue:  types.StringNull(),
			planValue:   types.StringUnknown(),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := tt.customSummary
			message := tt.customMessage
			modifier := NewImmutableStringModifier(summary, message)

			req := planmodifier.StringRequest{
				StateValue: tt.stateValue,
				PlanValue:  tt.planValue,
			}
			resp := &planmodifier.StringResponse{}

			modifier.PlanModifyString(context.Background(), req, resp)

			if tt.expectError {
				if !resp.Diagnostics.HasError() {
					t.Error("expected error but got none")
				}
				// Check that the error message contains expected text
				found := false
				for _, diag := range resp.Diagnostics.Errors() {
					if contains(diag.Detail(), tt.errorContains) || contains(diag.Summary(), tt.errorContains) {
						found = true

						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got %v", tt.errorContains, resp.Diagnostics)
				}
			} else if resp.Diagnostics.HasError() {
				t.Errorf("unexpected error: %v", resp.Diagnostics)
			}
		})
	}
}

func TestImmutableStringModifier_ErrorMessageFormat(t *testing.T) {
	// Verify the error message includes both old and new values
	modifier := NewImmutableStringModifier("Test Summary", "Cannot change from %q to %q please contact support")

	req := planmodifier.StringRequest{
		StateValue: types.StringValue("old-version"),
		PlanValue:  types.StringValue("new-version"),
	}
	resp := &planmodifier.StringResponse{}

	modifier.PlanModifyString(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("expected error but got none")
	}

	// Verify both values appear in the error
	errDetail := resp.Diagnostics.Errors()[0].Detail()
	if !contains(errDetail, "old-version") {
		t.Errorf("error message should contain old value, got: %s", errDetail)
	}
	if !contains(errDetail, "new-version") {
		t.Errorf("error message should contain new value, got: %s", errDetail)
	}
}

// contains checks if substr is in s
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || substr == "" ||
		(s != "" && substr != "" && searchString(s, substr)))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

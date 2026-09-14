package common

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestSemanticEqualityStringModifierSuppressesSpellingDiff is the import case
// the custom type alone does not cover.
//
// Semantic equality is applied by the framework in Create, Read, Update and
// ReadDataSource — but NOT in PlanResourceChange. So after `terraform import`,
// where Read has nothing prior to preserve and state takes the API's spelling,
// the next plan compares state against config with no semantic equality at all.
// Without this modifier that is a real diff on an immutable attribute.
func TestSemanticEqualityStringModifierSuppressesSpellingDiff(t *testing.T) {
	t.Parallel()

	modifier := NewSemanticEqualityStringModifier("same Kubernetes version", SameK8sVersion)

	// State holds what a CMKv2 control plane reports; config holds the display
	// name a create requires.
	req := planmodifier.StringRequest{
		StateValue:  types.StringValue("1.35.5"),
		PlanValue:   types.StringValue("1.35.5-cmk.22"),
		ConfigValue: types.StringValue("1.35.5-cmk.22"),
	}
	resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	modifier.PlanModifyString(context.Background(), req, resp)

	if !resp.PlanValue.Equal(req.StateValue) {
		t.Errorf("PlanValue = %v, want the stored %v so the diff is suppressed", resp.PlanValue, req.StateValue)
	}
	if resp.Diagnostics.HasError() {
		t.Errorf("unexpected diagnostics: %v", resp.Diagnostics)
	}
}

// TestSemanticEqualityStringModifierKeepsRealChange is the other half: a genuine
// version change must survive as a diff, so the immutability check still sees it.
func TestSemanticEqualityStringModifierKeepsRealChange(t *testing.T) {
	t.Parallel()

	modifier := NewSemanticEqualityStringModifier("same Kubernetes version", SameK8sVersion)

	req := planmodifier.StringRequest{
		StateValue:  types.StringValue("1.35.5-cmk.22"),
		PlanValue:   types.StringValue("1.36.1-cmk.1"),
		ConfigValue: types.StringValue("1.36.1-cmk.1"),
	}
	resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	modifier.PlanModifyString(context.Background(), req, resp)

	if !resp.PlanValue.Equal(req.PlanValue) {
		t.Errorf("PlanValue = %v, want the configured %v left alone", resp.PlanValue, req.PlanValue)
	}
}

// TestSemanticEqualityStringModifierIgnoresNullAndUnknown covers create (no
// prior state) and a value another modifier has left unknown.
func TestSemanticEqualityStringModifierIgnoresNullAndUnknown(t *testing.T) {
	t.Parallel()

	modifier := NewSemanticEqualityStringModifier("same Kubernetes version", SameK8sVersion)

	for name, req := range map[string]planmodifier.StringRequest{
		"create: no prior state": {
			StateValue: types.StringNull(), PlanValue: types.StringValue("1.35.5-cmk.22"),
		},
		"plan value unknown": {
			StateValue: types.StringValue("1.35.5"), PlanValue: types.StringUnknown(),
		},
		"plan value null": {
			StateValue: types.StringValue("1.35.5"), PlanValue: types.StringNull(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
			modifier.PlanModifyString(context.Background(), req, resp)

			if !resp.PlanValue.Equal(req.PlanValue) {
				t.Errorf("PlanValue = %v, want it untouched (%v)", resp.PlanValue, req.PlanValue)
			}
		})
	}
}

// TestSemanticEqualityThenImmutableStaysQuiet pins the interaction the schema
// depends on: the suppressor runs first, so by the time the immutability check
// compares plan against state the spelling difference is gone. Ordering matters
// — reverse these and the import case errors.
func TestSemanticEqualityThenImmutableStaysQuiet(t *testing.T) {
	t.Parallel()

	suppress := NewSemanticEqualityStringModifier("same Kubernetes version", SameK8sVersion)
	immutable := NewImmutableStringModifier("Version Change Not Supported", "Cannot change from %q to %q.")

	req := planmodifier.StringRequest{
		StateValue:  types.StringValue("1.35.5"),
		PlanValue:   types.StringValue("1.35.5-cmk.22"),
		ConfigValue: types.StringValue("1.35.5-cmk.22"),
	}

	resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	suppress.PlanModifyString(context.Background(), req, resp)

	// The framework feeds each modifier the running plan value.
	req.PlanValue = resp.PlanValue
	immutable.PlanModifyString(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("immutability check fired on a spelling-only difference: %v", resp.Diagnostics)
	}

	// And a real change still errors.
	realReq := planmodifier.StringRequest{
		StateValue:  types.StringValue("1.35.5-cmk.22"),
		PlanValue:   types.StringValue("1.36.1-cmk.1"),
		ConfigValue: types.StringValue("1.36.1-cmk.1"),
	}
	realResp := &planmodifier.StringResponse{PlanValue: realReq.PlanValue}
	suppress.PlanModifyString(context.Background(), realReq, realResp)
	realReq.PlanValue = realResp.PlanValue
	immutable.PlanModifyString(context.Background(), realReq, realResp)

	if !realResp.Diagnostics.HasError() {
		t.Error("a genuine version change should still be refused")
	}
}

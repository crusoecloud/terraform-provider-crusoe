package common

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestImmutableStringModifierSemanticEquality covers the escape hatch on its
// own, independent of any one attribute's wiring.
func TestImmutableStringModifierSemanticEquality(t *testing.T) {
	t.Parallel()

	plain := NewImmutableStringModifier("Immutable", "Cannot change from %q to %q.")
	aware := plain.WithSemanticEquality(SameK8sVersion)

	req := planmodifier.StringRequest{
		StateValue:  types.StringValue("1.35.5"),
		PlanValue:   types.StringValue("1.35.5-cmk.22"),
		ConfigValue: types.StringValue("1.35.5-cmk.22"),
	}

	// Without the hatch this is the pre-fix behaviour: a refused plan.
	plainResp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	plain.PlanModifyString(context.Background(), req, plainResp)
	if !plainResp.Diagnostics.HasError() {
		t.Error("without semantic equality, a spelling difference should still be refused")
	}

	awareResp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
	aware.PlanModifyString(context.Background(), req, awareResp)
	if awareResp.Diagnostics.HasError() {
		t.Errorf("with semantic equality, a spelling difference must be allowed: %v", awareResp.Diagnostics)
	}
	if !awareResp.PlanValue.Equal(req.PlanValue) {
		t.Errorf("PlanValue = %v, want the configured value left in place", awareResp.PlanValue)
	}

	// The hatch must not swallow a create (no prior state) or a real change.
	create := planmodifier.StringRequest{StateValue: types.StringNull(), PlanValue: types.StringValue("1.35.5-cmk.22")}
	createResp := &planmodifier.StringResponse{PlanValue: create.PlanValue}
	aware.PlanModifyString(context.Background(), create, createResp)
	if createResp.Diagnostics.HasError() {
		t.Errorf("create should never be refused: %v", createResp.Diagnostics)
	}

	change := planmodifier.StringRequest{
		StateValue: types.StringValue("1.35.5-cmk.22"),
		PlanValue:  types.StringValue("1.36.1-cmk.1"),
	}
	changeResp := &planmodifier.StringResponse{PlanValue: change.PlanValue}
	aware.PlanModifyString(context.Background(), change, changeResp)
	if !changeResp.Diagnostics.HasError() {
		t.Error("a genuine change must still be refused")
	}
}

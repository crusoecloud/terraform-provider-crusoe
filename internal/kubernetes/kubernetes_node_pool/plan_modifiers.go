package kubernetes_node_pool

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// SortTaintsByEffectKey returns a list plan modifier that sorts node
// taints by (effect, key) before TF compares plan to state. The server
// (kubernetes-manager) stores taints in this canonical order, so without
// this modifier users writing taints in a different order see perpetual
// drift on `terraform plan`.
func SortTaintsByEffectKey() planmodifier.List {
	return taintsSortPlanModifier{}
}

type taintsSortPlanModifier struct{}

func (m taintsSortPlanModifier) Description(_ context.Context) string {
	return "Sorts node taints by (effect, key) to match the server's canonical storage order, preventing drift when users specify taints in a different order."
}

func (m taintsSortPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

//nolint:gocritic // hugeParam: req signature required by planmodifier.List interface
func (m taintsSortPlanModifier) PlanModifyList(
	ctx context.Context,
	req planmodifier.ListRequest,
	resp *planmodifier.ListResponse,
) {
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}

	var models []nodeTaintModel
	diags := req.PlanValue.ElementsAs(ctx, &models, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	sort.SliceStable(models, func(i, j int) bool {
		return models[i].Effect.ValueString()+","+models[i].Key.ValueString() <
			models[j].Effect.ValueString()+","+models[j].Key.ValueString()
	})

	sortedList, diags := types.ListValueFrom(
		ctx,
		types.ObjectType{AttrTypes: nodeTaintAttrTypes()},
		models,
	)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.PlanValue = sortedList
}

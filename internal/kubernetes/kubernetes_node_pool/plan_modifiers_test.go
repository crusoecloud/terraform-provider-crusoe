package kubernetes_node_pool

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

// makeTaintList builds a TF ListValue from a slice of (key, value, effect) tuples.
func makeTaintList(t *testing.T, rows [][3]string) types.List {
	t.Helper()
	ctx := context.Background()
	objs := make([]attr.Value, 0, len(rows))
	for _, r := range rows {
		obj, diags := types.ObjectValue(
			nodeTaintAttrTypes(),
			map[string]attr.Value{
				"key":    types.StringValue(r[0]),
				"value":  types.StringValue(r[1]),
				"effect": types.StringValue(r[2]),
			},
		)
		require.False(t, diags.HasError(), "build object: %s", diags)
		objs = append(objs, obj)
	}
	list, diags := types.ListValue(types.ObjectType{AttrTypes: nodeTaintAttrTypes()}, objs)
	require.False(t, diags.HasError(), "build list: %s", diags)
	_ = ctx

	return list
}

func TestSortTaintsByEffectKey_unsortedInput(t *testing.T) {
	ctx := context.Background()
	mod := SortTaintsByEffectKey()

	input := makeTaintList(t, [][3]string{
		{"z", "v", "NoSchedule"},
		{"a", "v", "NoSchedule"},
		{"b", "v", "NoExecute"},
		{"a", "v", "NoExecute"},
	})

	req := planmodifier.ListRequest{PlanValue: input}
	resp := &planmodifier.ListResponse{PlanValue: input}
	mod.PlanModifyList(ctx, req, resp)
	require.False(t, resp.Diagnostics.HasError())

	want := makeTaintList(t, [][3]string{
		// NoExecute group, then NoSchedule group; keys asc within each.
		{"a", "v", "NoExecute"},
		{"b", "v", "NoExecute"},
		{"a", "v", "NoSchedule"},
		{"z", "v", "NoSchedule"},
	})
	require.True(t, resp.PlanValue.Equal(want), "plan not sorted as expected:\ngot:  %s\nwant: %s",
		resp.PlanValue, want)
}

func TestSortTaintsByEffectKey_alreadySorted(t *testing.T) {
	ctx := context.Background()
	mod := SortTaintsByEffectKey()

	input := makeTaintList(t, [][3]string{
		{"a", "v", "NoExecute"},
		{"a", "v", "NoSchedule"},
	})

	req := planmodifier.ListRequest{PlanValue: input}
	resp := &planmodifier.ListResponse{PlanValue: input}
	mod.PlanModifyList(ctx, req, resp)
	require.False(t, resp.Diagnostics.HasError())
	require.True(t, resp.PlanValue.Equal(input))
}

func TestSortTaintsByEffectKey_nullAndUnknown(t *testing.T) {
	ctx := context.Background()
	mod := SortTaintsByEffectKey()
	objType := types.ObjectType{AttrTypes: nodeTaintAttrTypes()}

	for _, tc := range []struct {
		name string
		in   types.List
	}{
		{name: "null", in: types.ListNull(objType)},
		{name: "unknown", in: types.ListUnknown(objType)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := planmodifier.ListRequest{PlanValue: tc.in}
			resp := &planmodifier.ListResponse{PlanValue: tc.in}
			mod.PlanModifyList(ctx, req, resp)
			require.False(t, resp.Diagnostics.HasError())
			require.True(t, resp.PlanValue.Equal(tc.in), "null/unknown should pass through unchanged")
		})
	}
}

package kubernetes_node_pool

import (
	"context"
	"testing"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestTfListToNodeTaints(t *testing.T) {
	ctx := context.Background()

	taints := []nodeTaintModel{
		{
			Key:    types.StringValue("gpu"),
			Value:  types.StringValue("true"),
			Effect: types.StringValue("NoSchedule"),
		},
		{
			Key:    types.StringValue("team"),
			Value:  types.StringValue("ml"),
			Effect: types.StringValue("PreferNoSchedule"),
		},
	}

	tfList, diags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: nodeTaintAttrTypes(),
	}, taints)
	if diags.HasError() {
		t.Fatalf("failed to create test list: %s", diags.Errors())
	}

	result, err := tfListToNodeTaints(ctx, tfList)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 taints, got %d", len(result))
	}
	if result[0].Key != "gpu" || result[0].Value != "true" || result[0].Effect != "NoSchedule" {
		t.Errorf("first taint mismatch: %+v", result[0])
	}
	if result[1].Key != "team" || result[1].Value != "ml" || result[1].Effect != "PreferNoSchedule" {
		t.Errorf("second taint mismatch: %+v", result[1])
	}
}

func TestTfListToNodeTaints_Null(t *testing.T) {
	ctx := context.Background()

	result, err := tfListToNodeTaints(ctx, types.ListNull(types.ObjectType{
		AttrTypes: nodeTaintAttrTypes(),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %+v", result)
	}
}

func TestNodeTaintsToTFList(t *testing.T) {
	ctx := context.Background()

	taints := []swagger.KubernetesNodeTaint{
		{Key: "gpu", Value: "true", Effect: "NoSchedule"},
	}

	tfList, diags := nodeTaintsToTFList(ctx, taints)
	if diags.HasError() {
		t.Fatalf("unexpected error: %s", diags.Errors())
	}
	if tfList.IsNull() {
		t.Fatal("expected non-null list")
	}

	result, err := tfListToNodeTaints(ctx, tfList)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(result) != 1 || result[0].Key != "gpu" {
		t.Errorf("round-trip mismatch: %+v", result)
	}
}

func TestNodeTaintsToTFList_Empty(t *testing.T) {
	ctx := context.Background()

	tfList, diags := nodeTaintsToTFList(ctx, []swagger.KubernetesNodeTaint{})
	if diags.HasError() {
		t.Fatalf("unexpected error: %s", diags.Errors())
	}
	if !tfList.IsNull() {
		t.Error("expected null list for empty taints")
	}
}

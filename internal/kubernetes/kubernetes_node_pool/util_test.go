package kubernetes_node_pool

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
)

func TestTfSetToNodeTaints(t *testing.T) {
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

	tfList, diags := types.SetValueFrom(ctx, types.ObjectType{
		AttrTypes: nodeTaintAttrTypes(),
	}, taints)
	if diags.HasError() {
		t.Fatalf("failed to create test list: %s", diags.Errors())
	}

	result, err := tfSetToNodeTaints(ctx, tfList)
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

func TestTfSetToNodeTaints_Null(t *testing.T) {
	ctx := context.Background()

	result, err := tfSetToNodeTaints(ctx, types.SetNull(types.ObjectType{
		AttrTypes: nodeTaintAttrTypes(),
	}))
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if result != nil {
		t.Errorf("expected nil, got %+v", result)
	}
}

func TestNodeTaintsToTFSet(t *testing.T) {
	ctx := context.Background()

	taints := []swagger.KubernetesNodeTaint{
		{Key: "gpu", Value: "true", Effect: "NoSchedule"},
	}

	tfList, diags := nodeTaintsToTFSet(ctx, taints)
	if diags.HasError() {
		t.Fatalf("unexpected error: %s", diags.Errors())
	}
	if tfList.IsNull() {
		t.Fatal("expected non-null list")
	}

	result, err := tfSetToNodeTaints(ctx, tfList)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if len(result) != 1 || result[0].Key != "gpu" {
		t.Errorf("round-trip mismatch: %+v", result)
	}
}

func TestNodeTaintsToTFSet_Empty(t *testing.T) {
	ctx := context.Background()

	tfList, diags := nodeTaintsToTFSet(ctx, []swagger.KubernetesNodeTaint{})
	if diags.HasError() {
		t.Fatalf("unexpected error: %s", diags.Errors())
	}
	if tfList.IsNull() {
		t.Error("expected empty list, got null")
	}
	if len(tfList.Elements()) != 0 {
		t.Errorf("expected 0 elements, got %d", len(tfList.Elements()))
	}
}

func TestValidateNodeTaintDuplicates(t *testing.T) {
	// no duplicates: same key with different effect is OK
	err := validateNodeTaintDuplicates([]swagger.KubernetesNodeTaint{
		{Key: "gpu", Effect: "NoSchedule"},
		{Key: "gpu", Effect: "NoExecute"},
	})
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}

	// single duplicate: should be flat one-liner, no bullet
	err = validateNodeTaintDuplicates([]swagger.KubernetesNodeTaint{
		{Key: "gpu", Effect: "NoSchedule"},
		{Key: "gpu", Effect: "NoSchedule"},
	})
	if err == nil {
		t.Fatal("expected error for duplicate taints")
	}
	if !strings.Contains(err.Error(), `"gpu"`) || !strings.Contains(err.Error(), `"NoSchedule"`) {
		t.Errorf("error should mention key and effect, got: %s", err)
	}
	if strings.Contains(err.Error(), "\n") {
		t.Errorf("single duplicate should be one-line, got: %q", err.Error())
	}

	// multiple duplicates: aggregated into a bullet list
	err = validateNodeTaintDuplicates([]swagger.KubernetesNodeTaint{
		{Key: "gpu", Effect: "NoSchedule"},
		{Key: "gpu", Effect: "NoSchedule"}, // dup #1
		{Key: "team", Effect: "NoExecute"},
		{Key: "team", Effect: "NoExecute"}, // dup #2
		{Key: "zone", Effect: "PreferNoSchedule"},
		{Key: "zone", Effect: "PreferNoSchedule"}, // dup #3
	})
	if err == nil {
		t.Fatal("expected error for multiple duplicates")
	}
	for _, want := range []string{`"gpu"`, `"team"`, `"zone"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("aggregated error missing %s, got: %s", want, err)
		}
	}
	if !strings.Contains(err.Error(), "\n  - ") {
		t.Errorf("multiple duplicates should use bullet list format, got: %q", err.Error())
	}

	// empty input
	err = validateNodeTaintDuplicates([]swagger.KubernetesNodeTaint{})
	if err != nil {
		t.Errorf("unexpected error for empty taints: %s", err)
	}
}

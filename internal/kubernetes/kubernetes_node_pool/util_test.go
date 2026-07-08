package kubernetes_node_pool

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

func TestStringOrNull(t *testing.T) {
	if got := stringOrNull(""); !got.IsNull() {
		t.Errorf("stringOrNull(\"\") = %v, want null", got)
	}
	if got := stringOrNull("nvlink-1"); got.ValueString() != "nvlink-1" {
		t.Errorf("stringOrNull(%q) = %q, want %q", "nvlink-1", got.ValueString(), "nvlink-1")
	}
}

func TestSortedInstanceIDs(t *testing.T) {
	got := sortedInstanceIDs([]string{"i-c", "i-a", "i-b"})
	if want := []string{"i-a", "i-b", "i-c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("sortedInstanceIDs = %v, want %v", got, want)
	}

	// The input slice must not be mutated.
	in := []string{"z", "a"}
	_ = sortedInstanceIDs(in)
	if !reflect.DeepEqual(in, []string{"z", "a"}) {
		t.Errorf("input was mutated: %v", in)
	}
}

// Test_nodePoolToResourceModel covers the shared transform all CRUD paths use:
// instance_ids sorted (CCX-4394), nvlink_domain_id empty→null (the normalization
// the data source previously omitted), API fields mapped, and the Terraform-only
// fields sourced from the reference model.
func Test_nodePoolToResourceModel(t *testing.T) {
	nodePool := &swagger.KubernetesNodePool{
		Id:             "np-1",
		ProjectId:      "proj-1",
		Count:          3,
		ImageId:        "1.2.3-cmk.4",
		Type_:          "a100.1x",
		ClusterId:      "cluster-1",
		InstanceIds:    []string{"i-c", "i-a", "i-b"},
		NvlinkDomainId: "", // absent → must normalize to null
	}
	ref := &kubernetesNodePoolResourceModel{
		IBPartitionID:   types.StringValue("ibp-1"),
		SSHKey:          types.StringValue("ssh-key"),
		BatchSize:       types.Int64Value(2),
		BatchPercentage: types.Int64Null(),
	}

	var diags diag.Diagnostics
	var model kubernetesNodePoolResourceModel
	nodePoolToResourceModel(context.Background(), nodePool, ref, &model, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got := model.ID.ValueString(); got != "np-1" {
		t.Errorf("id = %q, want %q", got, "np-1")
	}
	if !model.NvlinkDomainID.IsNull() {
		t.Errorf("nvlink_domain_id = %v, want null (empty API value normalizes to null)", model.NvlinkDomainID)
	}
	var ids []string
	if d := model.InstanceIDs.ElementsAs(context.Background(), &ids, false); d.HasError() {
		t.Fatalf("reading instance_ids: %v", d)
	}
	if want := []string{"i-a", "i-b", "i-c"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("instance_ids = %v, want %v (sorted)", ids, want)
	}
	if got := model.IBPartitionID.ValueString(); got != "ibp-1" {
		t.Errorf("ib_partition_id = %q, want it preserved from ref", got)
	}
}

// Test_nodePoolToResourceModel_createReadIdentical asserts the CCX-4492 criterion:
// given the same API object and reference, the transform now shared by Create,
// Read, and Update produces identical state.
func Test_nodePoolToResourceModel_createReadIdentical(t *testing.T) {
	nodePool := &swagger.KubernetesNodePool{
		Id:          "np-1",
		InstanceIds: []string{"i-2", "i-1"},
	}
	ref := &kubernetesNodePoolResourceModel{
		SSHKey:              types.StringValue("ssh-key"),
		RequestedNodeLabels: types.MapNull(types.StringType),
	}

	var d1, d2 diag.Diagnostics
	var createModel, readModel kubernetesNodePoolResourceModel
	nodePoolToResourceModel(context.Background(), nodePool, ref, &createModel, &d1)
	nodePoolToResourceModel(context.Background(), nodePool, ref, &readModel, &d2)
	if d1.HasError() || d2.HasError() {
		t.Fatalf("unexpected diagnostics: create=%v read=%v", d1, d2)
	}

	if !reflect.DeepEqual(createModel, readModel) {
		t.Errorf("Create and Read produced different state:\n create = %+v\n read   = %+v", createModel, readModel)
	}
}

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

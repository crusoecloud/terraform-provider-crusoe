package kubernetes_cluster

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

func mustMap(t *testing.T, m map[string]string) types.Map {
	t.Helper()
	elems := make(map[string]attr.Value, len(m))
	for k, v := range m {
		elems[k] = types.StringValue(v)
	}
	tfMap, diags := types.MapValue(types.StringType, elems)
	if diags.HasError() {
		t.Fatalf("building map: %v", diags)
	}

	return tfMap
}

func TestSortedNodePools(t *testing.T) {
	got := sortedNodePools([]string{"np-c", "np-a", "np-b"})
	want := []string{"np-a", "np-b", "np-c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sortedNodePools = %v, want %v", got, want)
	}

	// The input slice must not be mutated.
	in := []string{"z", "a"}
	_ = sortedNodePools(in)
	if !reflect.DeepEqual(in, []string{"z", "a"}) {
		t.Errorf("input was mutated: %v", in)
	}
}

// Test_clusterToResourceModel_extraArgsUnsetStayNull checks that when the user
// leaves an extra-args field unset (ref null) and the API echoes an empty map, the
// transform keeps the field null instead of storing {}.
func Test_clusterToResourceModel_extraArgsUnsetStayNull(t *testing.T) {
	cluster := &swagger.KubernetesCluster{
		Id:                 "cluster-1",
		ApiserverExtraArgs: map[string]string{}, // API echoes {} for an unconfigured field
	}
	ref := &kubernetesClusterResourceModel{
		ApiserverExtraArgs: types.MapNull(types.StringType), // user left it unset
	}

	var diags diag.Diagnostics
	var model kubernetesClusterResourceModel
	clusterToResourceModel(cluster, ref, &model, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if !model.ApiserverExtraArgs.IsNull() {
		t.Errorf("apiserver_extra_args = %v, want null (an API {} must not overwrite an unset field)", model.ApiserverExtraArgs)
	}
}

// Test_clusterToResourceModel covers the field mapping: nodepool_ids sorted, OIDC
// preserved from the reference, a set extra-args field sourced from the API, and an
// unset one kept null.
func Test_clusterToResourceModel(t *testing.T) {
	cluster := &swagger.KubernetesCluster{
		Id:                 "cluster-1",
		ProjectId:          "proj-1",
		Name:               "my-cluster",
		Version:            "1.2.3-cmk.4",
		NodePools:          []string{"np-c", "np-a", "np-b"},
		ApiserverExtraArgs: map[string]string{"audit-log-maxage": "30"},
		SchedulerExtraArgs: map[string]string{}, // API echoes {}
	}
	ref := &kubernetesClusterResourceModel{
		OIDCIssuerURL:              types.StringValue("https://issuer.example"),
		ApiserverExtraArgs:         mustMap(t, map[string]string{"audit-log-maxage": "30"}), // user set it
		SchedulerExtraArgs:         types.MapNull(types.StringType),                         // user left it unset
		ControllerManagerExtraArgs: types.MapNull(types.StringType),
	}

	var diags diag.Diagnostics
	var model kubernetesClusterResourceModel
	clusterToResourceModel(cluster, ref, &model, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got := model.ID.ValueString(); got != "cluster-1" {
		t.Errorf("id = %q, want %q", got, "cluster-1")
	}
	if got := model.OIDCIssuerURL.ValueString(); got != "https://issuer.example" {
		t.Errorf("oidc_issuer_url = %q, want it preserved from ref", got)
	}

	var ids []string
	if d := model.NodePoolIds.ElementsAs(context.Background(), &ids, false); d.HasError() {
		t.Fatalf("reading nodepool_ids: %v", d)
	}
	if want := []string{"np-a", "np-b", "np-c"}; !reflect.DeepEqual(ids, want) {
		t.Errorf("nodepool_ids = %v, want %v (sorted)", ids, want)
	}

	if model.ApiserverExtraArgs.IsNull() {
		t.Error("apiserver_extra_args should be set (ref non-null), got null")
	}
	if !model.SchedulerExtraArgs.IsNull() {
		t.Errorf("scheduler_extra_args = %v, want null (ref unset, API {})", model.SchedulerExtraArgs)
	}
}

// Test_clusterToResourceModel_createReadIdentical checks that, given the same API
// object and reference, the shared transform used by Create, Read, and Update
// produces identical state.
func Test_clusterToResourceModel_createReadIdentical(t *testing.T) {
	cluster := &swagger.KubernetesCluster{
		Id:                 "cluster-1",
		NodePools:          []string{"np-2", "np-1"},
		ApiserverExtraArgs: map[string]string{},
	}
	ref := &kubernetesClusterResourceModel{
		OIDCClientID:               types.StringValue("client-1"),
		ApiserverExtraArgs:         types.MapNull(types.StringType),
		SchedulerExtraArgs:         types.MapNull(types.StringType),
		ControllerManagerExtraArgs: types.MapNull(types.StringType),
	}

	var d1, d2 diag.Diagnostics
	var createModel, readModel kubernetesClusterResourceModel
	clusterToResourceModel(cluster, ref, &createModel, &d1)
	clusterToResourceModel(cluster, ref, &readModel, &d2)
	if d1.HasError() || d2.HasError() {
		t.Fatalf("unexpected diagnostics: create=%v read=%v", d1, d2)
	}

	if !reflect.DeepEqual(createModel, readModel) {
		t.Errorf("Create and Read produced different state:\n create = %+v\n read   = %+v", createModel, readModel)
	}
}

package kubernetes_cluster

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
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
		RoutingMode:        routingModeNative,
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
	if got := model.RoutingMode.ValueString(); got != routingModeNative {
		t.Errorf("routing_mode = %q, want %q (API is the source of truth)", got, routingModeNative)
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

// Test_versionUsesSemanticEqualityType guards the wiring that makes a CMKv2
// cluster creatable. The preservation itself happens in the framework, which
// only invokes semantic equality when the attribute declares the custom type —
// so dropping CustomType here would silently restore "Provider produced
// inconsistent result after apply" on every CMKv2 create. Both schemas are
// checked because state flows through each.
func Test_versionUsesSemanticEqualityType(t *testing.T) {
	ctx := context.Background()

	resourceSchema := &resource.SchemaResponse{}
	NewKubernetesClusterResource().Schema(ctx, resource.SchemaRequest{}, resourceSchema)

	versionAttr, ok := resourceSchema.Schema.Attributes["version"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("resource version attribute is %T, want schema.StringAttribute",
			resourceSchema.Schema.Attributes["version"])
	}
	if _, isVersionType := versionAttr.CustomType.(common.K8sVersionType); !isVersionType {
		t.Errorf("resource version CustomType = %T, want common.K8sVersionType", versionAttr.CustomType)
	}

	dataSourceSchema := &datasource.SchemaResponse{}
	NewKubernetesClusterDataSource().Schema(ctx, datasource.SchemaRequest{}, dataSourceSchema)

	dsAttr, ok := dataSourceSchema.Schema.Attributes["version"].(dsschema.StringAttribute)
	if !ok {
		t.Fatalf("data source version attribute is %T, want dsschema.StringAttribute",
			dataSourceSchema.Schema.Attributes["version"])
	}
	if _, isVersionType := dsAttr.CustomType.(common.K8sVersionType); !isVersionType {
		t.Errorf("data source version CustomType = %T, want common.K8sVersionType", dsAttr.CustomType)
	}
}

// Test_versionIsImmutableButSpellingAware pins the two halves of the version
// attribute's plan behaviour: a real version change is still refused, and a
// difference that is only a difference in spelling is not.
//
// The second half is what makes `terraform import` of a cluster workable. The
// framework does not apply semantic equality during PlanResourceChange, so
// after an import — where Read has no prior value to preserve and state takes
// the API's spelling — the immutability check is the thing that would otherwise
// refuse a plan over a version nobody changed.
func Test_versionIsImmutableButSpellingAware(t *testing.T) {
	ctx := context.Background()

	resourceSchema := &resource.SchemaResponse{}
	NewKubernetesClusterResource().Schema(ctx, resource.SchemaRequest{}, resourceSchema)

	versionAttr, ok := resourceSchema.Schema.Attributes["version"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("version attribute is %T, want schema.StringAttribute", resourceSchema.Schema.Attributes["version"])
	}
	if len(versionAttr.PlanModifiers) != 1 {
		t.Fatalf("version has %d plan modifiers, want 1", len(versionAttr.PlanModifiers))
	}

	modifier, ok := versionAttr.PlanModifiers[0].(common.ImmutableStringModifier)
	if !ok {
		t.Fatalf("version plan modifier is %T, want common.ImmutableStringModifier", versionAttr.PlanModifiers[0])
	}

	// A CMKv2 control plane reports the upstream semver; the configuration
	// carries the display name. Same version, so the plan must survive.
	spelling := planmodifier.StringRequest{
		StateValue:  types.StringValue("1.35.5"),
		PlanValue:   types.StringValue("1.35.5-cmk.22"),
		ConfigValue: types.StringValue("1.35.5-cmk.22"),
	}
	spellingResp := &planmodifier.StringResponse{PlanValue: spelling.PlanValue}
	modifier.PlanModifyString(ctx, spelling, spellingResp)
	if spellingResp.Diagnostics.HasError() {
		t.Errorf("a spelling-only difference was refused: %v", spellingResp.Diagnostics)
	}
	if !spellingResp.PlanValue.Equal(spelling.PlanValue) {
		t.Errorf("PlanValue = %v, want the configured value kept so one apply settles state on it",
			spellingResp.PlanValue)
	}

	// A genuine upgrade is still refused.
	upgrade := planmodifier.StringRequest{
		StateValue:  types.StringValue("1.35.5-cmk.22"),
		PlanValue:   types.StringValue("1.36.1-cmk.1"),
		ConfigValue: types.StringValue("1.36.1-cmk.1"),
	}
	upgradeResp := &planmodifier.StringResponse{PlanValue: upgrade.PlanValue}
	modifier.PlanModifyString(ctx, upgrade, upgradeResp)
	if !upgradeResp.Diagnostics.HasError() {
		t.Error("a genuine version change should still be refused")
	}
}

// Test_dnsNameSurvivesUnrelatedChanges guards the pin that keeps an imported
// cluster from planning a no-op update forever. MarkComputedNilsAsUnknown
// stamps every Computed attribute whose config is null as unknown as soon as
// anything else in the plan differs, and it runs before plan modifiers —
// UseStateForUnknown is what undoes that.
func Test_dnsNameSurvivesUnrelatedChanges(t *testing.T) {
	resourceSchema := &resource.SchemaResponse{}
	NewKubernetesClusterResource().Schema(context.Background(), resource.SchemaRequest{}, resourceSchema)

	dnsAttr, ok := resourceSchema.Schema.Attributes["dns_name"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("dns_name attribute is %T, want schema.StringAttribute", resourceSchema.Schema.Attributes["dns_name"])
	}
	if len(dnsAttr.PlanModifiers) == 0 {
		t.Fatal("dns_name has no plan modifiers; it needs UseStateForUnknown to stay known across unrelated changes")
	}
}

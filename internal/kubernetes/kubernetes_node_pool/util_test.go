package kubernetes_node_pool

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
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
		IBPartitionID:        types.StringValue("ibp-1"),
		TransportPartitionID: types.StringValue("tp-1"),
		SSHKey:               types.StringValue("ssh-key"),
		BatchSize:            types.Int64Value(2),
		BatchPercentage:      types.Int64Null(),
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
	// Both halves of the partition alias pair are Terraform-only: the API does not return
	// them, so each must survive from the reference model unchanged.
	if got := model.IBPartitionID.ValueString(); got != "ibp-1" {
		t.Errorf("ib_partition_id = %q, want it preserved from ref", got)
	}
	if got := model.TransportPartitionID.ValueString(); got != "tp-1" {
		t.Errorf("transport_partition_id = %q, want it preserved from ref", got)
	}
}

// Test_nodePoolToResourceModel_aliasPairFromRef checks each combination of the partition
// alias pair through the reference model. The transform must not invent a value for the
// half the configuration leaves unset, in either direction.
func Test_nodePoolToResourceModel_aliasPairFromRef(t *testing.T) {
	tests := []struct {
		name                  string
		refIB, refTransport   types.String
		wantIB, wantTransport types.String
	}{
		{
			name:  "configuration uses the deprecated name",
			refIB: types.StringValue("ibp-1"), refTransport: types.StringNull(),
			wantIB: types.StringValue("ibp-1"), wantTransport: types.StringNull(),
		},
		{
			name:  "configuration uses the replacement name",
			refIB: types.StringNull(), refTransport: types.StringValue("tp-1"),
			wantIB: types.StringNull(), wantTransport: types.StringValue("tp-1"),
		},
		{
			name:  "configuration sets neither",
			refIB: types.StringNull(), refTransport: types.StringNull(),
			wantIB: types.StringNull(), wantTransport: types.StringNull(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := &kubernetesNodePoolResourceModel{
				IBPartitionID:        tt.refIB,
				TransportPartitionID: tt.refTransport,
			}

			var diags diag.Diagnostics
			var model kubernetesNodePoolResourceModel
			nodePoolToResourceModel(context.Background(),
				&swagger.KubernetesNodePool{Id: "np-1"}, ref, &model, &diags)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if !model.IBPartitionID.Equal(tt.wantIB) {
				t.Errorf("ib_partition_id = %v, want %v", model.IBPartitionID, tt.wantIB)
			}
			if !model.TransportPartitionID.Equal(tt.wantTransport) {
				t.Errorf("transport_partition_id = %v, want %v", model.TransportPartitionID, tt.wantTransport)
			}
		})
	}
}

// Test_nodePoolSchemaAliasPair checks the two schema-level facts the docs and the plan
// behavior depend on: transport_partition_id carries a description (an undescribed
// attribute renders bare in the generated docs), and neither half still carries the
// UseStateForUnknown modifier, which never applied to an Optional-only attribute.
func Test_nodePoolSchemaAliasPair(t *testing.T) {
	schemaResp := &resource.SchemaResponse{}
	NewKubernetesNodePoolResource().Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to build schema: %v", schemaResp.Diagnostics)
	}

	for _, name := range []string{attrIBPartitionID, attrTransportPartitionID} {
		attr, ok := schemaResp.Schema.Attributes[name].(schema.StringAttribute)
		if !ok {
			t.Fatalf("%s is not a StringAttribute", name)
		}

		if attr.Description == "" {
			t.Errorf("%s has no description, which renders bare in the generated docs", name)
		}
		if len(attr.PlanModifiers) != 1 {
			t.Errorf("%s has %d plan modifiers, want exactly the alias pair modifier",
				name, len(attr.PlanModifiers))
		}
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

// TestVersionUsesSemanticEqualityType is the node pool's counterpart to the
// cluster's identical guard. version is mapped from the API's image_id, whose
// spelling the provider does not control: a node pool reports the resolved
// worker image's canonical version, never the bootstrap tarball suffix a
// customer may legitimately configure. Without the custom type the framework
// never asks whether the two spellings mean the same version.
func TestVersionUsesSemanticEqualityType(t *testing.T) {
	schemaResp := &resource.SchemaResponse{}
	NewKubernetesNodePoolResource().Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	attr, ok := schemaResp.Schema.Attributes["version"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("version attribute is %T, want schema.StringAttribute", schemaResp.Schema.Attributes["version"])
	}
	if _, isVersionType := attr.CustomType.(common.K8sVersionType); !isVersionType {
		t.Errorf("version CustomType = %T, want common.K8sVersionType", attr.CustomType)
	}
}

// TestNodePoolNeedsRolloutIgnoresVersionSpelling checks that rollout detection
// uses the same notion of "same version" as state does. A tarball-carrying
// config against the canonical version the API reports is one version, not a
// change, and must not propose replacing every node in the pool.
func TestNodePoolNeedsRolloutIgnoresVersionSpelling(t *testing.T) {
	base := func(version string) *kubernetesNodePoolResourceModel {
		return &kubernetesNodePoolResourceModel{
			Version:             common.NewK8sVersionValue(version),
			RequestedNodeLabels: types.MapNull(types.StringType),
			NodeTaints:          types.SetNull(types.ObjectType{AttrTypes: nodeTaintAttrTypes()}),
		}
	}

	plan := base("1.35.5-cmk.22-https://example.com/bootstrap.tar.gz")
	state := base("1.35.5-cmk.22")
	if nodePoolNeedsRollout(plan, state) {
		t.Error("a tarball suffix alone is not a version change and must not trigger a rollout")
	}

	plan = base("1.35.5-cmk.23")
	state = base("1.35.5-cmk.22")
	if !nodePoolNeedsRollout(plan, state) {
		t.Error("a different build of one release is a real version change and should trigger a rollout")
	}
}

// TestPatchRequestCarriesSSHKey pins the one field whose absence is destructive
// rather than merely missing.
//
// The generated client has no omitempty on ssh_public_key, so an unset field
// still serializes as `"ssh_public_key": ""`. The gateway's own field is a
// pointer, so the empty string arrives as a present value and reaches the
// backend as a request to change the key — and the node pool v2 update path has
// no empty guard, so it rebuilds the pool's instance template with no key at
// all. Every update, including a bare instance_count change, silently stripped
// the customer's SSH key from future nodes.
//
// The assertion is on the marshalled bytes because the bug lives in the
// serialization, not in the struct.
func TestPatchRequestCarriesSSHKey(t *testing.T) {
	const sshKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI test@example.com"

	patch := swagger.KubernetesNodePoolPatchRequest{
		Count:        3,
		SshPublicKey: sshKey,
	}

	body, err := json.Marshal(patch)
	if err != nil {
		t.Fatalf("marshalling the patch request: %v", err)
	}

	var sent map[string]any
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("unmarshalling the patch body: %v", err)
	}

	got, present := sent["ssh_public_key"]
	if !present {
		t.Fatal("ssh_public_key is absent from the patch body; the generated client has no omitempty, " +
			"so this should never happen")
	}
	if got != sshKey {
		t.Errorf("ssh_public_key = %q, want the pool's standing key %q; an empty value rebuilds the "+
			"instance template without an SSH key on the v2 backend", got, sshKey)
	}
}

// TestV2FieldsAbsentOnV1Pool is the central contract for the four fields only a
// v2-backed node pool carries: absence is carried through, never defaulted.
//
// The API omits all four for a v1-backed pool, and each absence means something
// specific. Defaulting update_settings to {allow_scale_down: false} would claim
// scale-down is configured and off; defaulting consent_mode to "off" would claim
// remediation is deliberately disabled on a pool that has its own remediation
// path; and an empty health block would claim the pool is healthy when the
// backend only meant it does not report.
func TestV2FieldsAbsentOnV1Pool(t *testing.T) {
	var diags diag.Diagnostics
	var model kubernetesNodePoolResourceModel
	ref := &kubernetesNodePoolResourceModel{RequestedNodeLabels: types.MapNull(types.StringType)}

	// A v1-backed pool: no consent mode, no update settings, no health, and a
	// current the generated client cannot tell apart from a real zero.
	nodePoolToResourceModel(context.Background(), &swagger.KubernetesNodePool{Id: "pool-1"}, ref, &model, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if !model.ConsentMode.IsNull() {
		t.Errorf("consent_mode = %v, want null", model.ConsentMode)
	}
	if !model.UpdateSettings.IsNull() {
		t.Errorf("update_settings = %v, want null", model.UpdateSettings)
	}
	if !model.Health.IsNull() {
		t.Errorf("health = %v, want null (absent means not reported, not healthy)", model.Health)
	}
	if !model.Current.IsNull() {
		t.Errorf("current = %v, want null; reporting 0 would read as no ready nodes", model.Current)
	}
}

// TestV2FieldsPresentOnV2Pool is the other half: a genuine zero must survive.
func TestV2FieldsPresentOnV2Pool(t *testing.T) {
	var diags diag.Diagnostics
	var model kubernetesNodePoolResourceModel
	ref := &kubernetesNodePoolResourceModel{RequestedNodeLabels: types.MapNull(types.StringType)}

	nodePoolToResourceModel(context.Background(), &swagger.KubernetesNodePool{
		Id:             "pool-1",
		ConsentMode:    "propose",
		UpdateSettings: &swagger.KubernetesNodePoolUpdateSettings{AllowScaleDown: true},
		Current:        0, // genuinely no ready nodes yet
	}, ref, &model, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got := model.ConsentMode.ValueString(); got != "propose" {
		t.Errorf("consent_mode = %q, want %q", got, "propose")
	}
	if model.UpdateSettings.IsNull() {
		t.Fatal("update_settings should be present")
	}
	if model.Current.IsNull() || model.Current.ValueInt64() != 0 {
		t.Errorf("current = %v, want 0; a v2 pool serving zero means zero ready nodes", model.Current)
	}
	if !planAllowsScaleDown(context.Background(), model.UpdateSettings) {
		t.Error("allow_scale_down should read back as true")
	}
}

// TestHealthIssuesSortedAndPresent covers the derived block, including the
// ordering the CCX-4394 rule requires of any API-ordered Computed list.
func TestHealthIssuesSortedAndPresent(t *testing.T) {
	var diags diag.Diagnostics
	health, d := healthToTFObject(context.Background(), &swagger.KubernetesNodePoolHealth{
		Issues: []swagger.KubernetesNodePoolHealthIssue{
			{Code: "NODE_NOT_READY", Message: "one node is not ready", AffectedCount: 1,
				AffectedNodeIds: []string{"vm-c", "vm-a", "vm-b"}},
			{Code: "INSUFFICIENT_CAPACITY", Message: "waiting for capacity", AffectedCount: 2},
		},
	})
	diags.Append(d...)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if health.IsNull() {
		t.Fatal("health should be present when there are issues")
	}

	var decoded struct {
		Issues []struct {
			Code            types.String `tfsdk:"code"`
			Message         types.String `tfsdk:"message"`
			Since           types.String `tfsdk:"since"`
			AffectedCount   types.Int64  `tfsdk:"affected_count"`
			AffectedNodeIDs types.List   `tfsdk:"affected_node_ids"`
		} `tfsdk:"issues"`
	}
	if d := health.As(context.Background(), &decoded, basetypes.ObjectAsOptions{}); d.HasError() {
		t.Fatalf("reading the health object: %v", d)
	}

	if len(decoded.Issues) != 2 {
		t.Fatalf("got %d issues, want 2", len(decoded.Issues))
	}
	// Sorted by code, so the capacity issue comes first regardless of API order.
	if got := decoded.Issues[0].Code.ValueString(); got != "INSUFFICIENT_CAPACITY" {
		t.Errorf("first issue code = %q, want INSUFFICIENT_CAPACITY (issues sort by code)", got)
	}
	// since is optional; an absent one is null rather than an empty string.
	if !decoded.Issues[0].Since.IsNull() {
		t.Errorf("since = %v, want null when the API omits it", decoded.Issues[0].Since)
	}

	var nodeIDs []string
	if d := decoded.Issues[1].AffectedNodeIDs.ElementsAs(context.Background(), &nodeIDs, false); d.HasError() {
		t.Fatalf("reading affected_node_ids: %v", d)
	}
	if want := []string{"vm-a", "vm-b", "vm-c"}; !reflect.DeepEqual(nodeIDs, want) {
		t.Errorf("affected_node_ids = %v, want %v (sorted)", nodeIDs, want)
	}
}

// TestUpdateSettingsPresenceRidesOnThePointer pins the distinction the API
// depends on. allow_scale_down has omitempty, so a present block holding false
// marshals as {} — the pointer is the only thing that separates "leave the
// stored setting alone" from "turn scale-down off".
func TestUpdateSettingsPresenceRidesOnThePointer(t *testing.T) {
	ctx := context.Background()

	absent, diags := tfObjectToUpdateSettings(ctx, types.ObjectNull(updateSettingsAttrTypes()))
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if absent != nil {
		t.Error("an absent block must yield a nil pointer, so the request omits update_settings entirely")
	}

	configured, diags := types.ObjectValue(updateSettingsAttrTypes(), map[string]attr.Value{
		"allow_scale_down": types.BoolValue(false),
	})
	if diags.HasError() {
		t.Fatalf("building the object: %v", diags)
	}

	present, diags := tfObjectToUpdateSettings(ctx, configured)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if present == nil {
		t.Fatal("a configured block must yield a non-nil pointer even when allow_scale_down is false")
	}
	if present.AllowScaleDown {
		t.Error("allow_scale_down should be false")
	}

	body, err := json.Marshal(present)
	if err != nil {
		t.Fatalf("marshalling update settings: %v", err)
	}
	if string(body) != "{}" {
		t.Errorf("update_settings marshalled as %s; expected {} because allow_scale_down has omitempty — "+
			"if this changes, re-check that the API still reads a present block as an assertion", body)
	}
}

// TestConsentModeValuesExcludeDetectOnly guards the one enum value that must
// not be offered: the API rejects detect_only as "not yet available", so
// accepting it here would only let a configuration fail on every apply.
func TestConsentModeValuesExcludeDetectOnly(t *testing.T) {
	for _, mode := range consentModeValues {
		if mode == "detect_only" {
			t.Fatal("detect_only must not be an accepted consent_mode; the API rejects it")
		}
	}
	if want := []string{"auto", "propose", "off"}; !reflect.DeepEqual(consentModeValues, want) {
		t.Errorf("consentModeValues = %v, want %v", consentModeValues, want)
	}
}

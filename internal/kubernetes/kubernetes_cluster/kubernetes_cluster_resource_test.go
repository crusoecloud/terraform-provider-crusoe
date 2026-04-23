package kubernetes_cluster

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestKubernetesClusterResource_Metadata(t *testing.T) {
	r := NewKubernetesClusterResource()

	req := resource.MetadataRequest{ProviderTypeName: "crusoe"}
	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), req, resp)

	expected := "crusoe_kubernetes_cluster"
	if resp.TypeName != expected {
		t.Errorf("TypeName: expected %q, got %q", expected, resp.TypeName)
	}
}

func TestKubernetesClusterResource_ExtraArgsSchemaAttributes(t *testing.T) {
	r := NewKubernetesClusterResource()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	for _, fieldName := range []string{"apiserver_extra_args", "scheduler_extra_args", "controller_manager_extra_args"} {
		attr, ok := schemaResp.Schema.Attributes[fieldName].(schema.MapAttribute)
		if !ok {
			t.Fatalf("%s attribute not found or not MapAttribute", fieldName)
		}

		if !attr.Optional {
			t.Errorf("%s should be optional", fieldName)
		}

		if attr.ElementType != types.StringType {
			t.Errorf("%s element type should be StringType", fieldName)
		}
	}
}

func TestTfMapToStringMap_Nil(t *testing.T) {
	result := tfMapToStringMap(context.Background(), types.MapNull(types.StringType))
	if result != nil {
		t.Errorf("expected nil for null map, got %v", result)
	}
}

func TestTfMapToStringMap_Unknown(t *testing.T) {
	result := tfMapToStringMap(context.Background(), types.MapUnknown(types.StringType))
	if result != nil {
		t.Errorf("expected nil for unknown map, got %v", result)
	}
}

func TestTfMapToStringMap_Values(t *testing.T) {
	tfMap, diags := types.MapValue(types.StringType, map[string]attr.Value{
		"key1": types.StringValue("val1"),
		"key2": types.StringValue("val2"),
	})
	if diags.HasError() {
		t.Fatalf("failed to build types.Map: %v", diags)
	}

	result := tfMapToStringMap(context.Background(), tfMap)

	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result["key1"] != "val1" {
		t.Errorf("key1: expected %q, got %q", "val1", result["key1"])
	}
	if result["key2"] != "val2" {
		t.Errorf("key2: expected %q, got %q", "val2", result["key2"])
	}
}

func TestStringMapToTFMap_Nil(t *testing.T) {
	result, diags := stringMapToTFMap(context.Background(), nil)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if !result.IsNull() {
		t.Errorf("expected null map for nil input, got %v", result)
	}
}

func TestStringMapToTFMap_Values(t *testing.T) {
	input := map[string]string{
		"flag1": "value1",
		"flag2": "value2",
	}

	result, diags := stringMapToTFMap(context.Background(), input)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	elements := result.Elements()
	if len(elements) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(elements))
	}

	for k, v := range input {
		elem, ok := elements[k]
		if !ok {
			t.Errorf("missing key %q in result", k)

			continue
		}

		strVal, ok := elem.(types.String)
		if !ok {
			t.Errorf("element %q is not types.String", k)

			continue
		}

		if strVal.ValueString() != v {
			t.Errorf("element %q: expected %q, got %q", k, v, strVal.ValueString())
		}
	}
}

func TestTfMapToStringMap_RoundTrip(t *testing.T) {
	input := map[string]string{
		"audit-log-maxage":         "30",
		"enable-admission-plugins": "NodeRestriction",
	}

	tfMap, diags := stringMapToTFMap(context.Background(), input)
	if diags.HasError() {
		t.Fatalf("stringMapToTFMap diagnostics: %v", diags)
	}

	result := tfMapToStringMap(context.Background(), tfMap)

	if len(result) != len(input) {
		t.Fatalf("expected %d entries, got %d", len(input), len(result))
	}

	for k, v := range input {
		if result[k] != v {
			t.Errorf("key %q: expected %q, got %q", k, v, result[k])
		}
	}
}

// Sad path: empty (non-null) map must NOT become nil.
// The PATCH API distinguishes nil (preserve existing args) from empty (clear args),
// so losing the distinction here would silently break arg removal.
func TestTfMapToStringMap_EmptyMapIsNotNil(t *testing.T) {
	tfMap, diags := types.MapValue(types.StringType, map[string]attr.Value{})
	if diags.HasError() {
		t.Fatalf("failed to build empty types.Map: %v", diags)
	}

	result := tfMapToStringMap(context.Background(), tfMap)

	if result == nil {
		t.Error("expected empty map (not nil) for empty non-null types.Map; nil would preserve args instead of clearing them")
	}

	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
}

// Sad path: empty Go map must produce a non-null types.Map.
// A null result would be indistinguishable from "not set" in state.
func TestStringMapToTFMap_EmptyMapIsNotNull(t *testing.T) {
	result, diags := stringMapToTFMap(context.Background(), map[string]string{})
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if result.IsNull() {
		t.Error("expected non-null types.Map for empty (non-nil) Go map")
	}

	if len(result.Elements()) != 0 {
		t.Errorf("expected 0 elements, got %d", len(result.Elements()))
	}
}

// Sad path: an empty-string value must be preserved, not dropped.
// Flags like --some-flag="" are valid and must round-trip correctly.
func TestTfMapToStringMap_PreservesEmptyStringValue(t *testing.T) {
	tfMap, diags := types.MapValue(types.StringType, map[string]attr.Value{
		"flag-with-empty-value": types.StringValue(""),
	})
	if diags.HasError() {
		t.Fatalf("failed to build types.Map: %v", diags)
	}

	result := tfMapToStringMap(context.Background(), tfMap)

	v, ok := result["flag-with-empty-value"]
	if !ok {
		t.Fatal("key \"flag-with-empty-value\" was dropped; empty-string values must be preserved")
	}

	if v != "" {
		t.Errorf("expected empty string, got %q", v)
	}
}

// Sad path: extra args fields must not have RequiresReplace.
// These fields are updated in-place via PATCH; RequiresReplace would force
// unnecessary cluster recreation on every extra-args change.
func TestKubernetesClusterResource_ExtraArgsHaveNoRequiresReplace(t *testing.T) {
	r := NewKubernetesClusterResource()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	for _, fieldName := range []string{"apiserver_extra_args", "scheduler_extra_args", "controller_manager_extra_args"} {
		attr, ok := schemaResp.Schema.Attributes[fieldName].(schema.MapAttribute)
		if !ok {
			t.Fatalf("%s attribute not found or not MapAttribute", fieldName)
		}

		if len(attr.PlanModifiers) != 0 {
			t.Errorf("%s should have no plan modifiers (got %d); adding RequiresReplace would force cluster recreation on arg changes",
				fieldName, len(attr.PlanModifiers))
		}
	}
}

// Sad path: extra args fields must not be Computed.
// Marking them Computed would cause Terraform to silently overwrite user config
// with API state when args are unset, masking drift.
func TestKubernetesClusterResource_ExtraArgsAreNotComputed(t *testing.T) {
	r := NewKubernetesClusterResource()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	for _, fieldName := range []string{"apiserver_extra_args", "scheduler_extra_args", "controller_manager_extra_args"} {
		attr, ok := schemaResp.Schema.Attributes[fieldName].(schema.MapAttribute)
		if !ok {
			t.Fatalf("%s attribute not found or not MapAttribute", fieldName)
		}

		if attr.Computed {
			t.Errorf("%s must not be Computed; that would silently overwrite user config with API state", fieldName)
		}
	}
}

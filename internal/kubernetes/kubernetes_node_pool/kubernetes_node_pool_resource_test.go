package kubernetes_node_pool

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestKubernetesNodePoolResource_BatchFieldsHavePlanModifiers(t *testing.T) {
	r := NewKubernetesNodePoolResource()

	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	for _, fieldName := range []string{"batch_size", "batch_percentage"} {
		attr, ok := schemaResp.Schema.Attributes[fieldName].(schema.Int64Attribute)
		if !ok {
			t.Fatalf("%s attribute not found or not Int64Attribute", fieldName)
		}

		if len(attr.PlanModifiers) == 0 {
			t.Errorf("%s should have plan modifiers", fieldName)
		}
	}
}

func TestKubernetesNodePoolResource_NodeTaintsBlockExists(t *testing.T) {
	r := NewKubernetesNodePoolResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	block, ok := schemaResp.Schema.Blocks["node_taints"]
	if !ok {
		t.Fatal("node_taints block not found in schema")
	}

	setBlock, ok := block.(schema.SetNestedBlock)
	if !ok {
		t.Fatal("node_taints is not a SetNestedBlock")
	}

	// verify key has validators
	keyAttr, ok := setBlock.NestedObject.Attributes["key"].(schema.StringAttribute)
	if !ok {
		t.Fatal("key attribute is not a StringAttribute")
	}
	if len(keyAttr.Validators) == 0 {
		t.Error("key attribute should have validators")
	}

	// verify key, value, effect attributes exist
	for _, field := range []string{"key", "value", "effect"} {
		if _, exists := setBlock.NestedObject.Attributes[field]; !exists {
			t.Errorf("node_taints block missing %s attribute", field)
		}
	}

	// verify value has validators
	valueAttr, ok := setBlock.NestedObject.Attributes["value"].(schema.StringAttribute)
	if !ok {
		t.Fatal("value attribute is not a StringAttribute")
	}
	if len(valueAttr.Validators) == 0 {
		t.Error("value attribute should have validators")
	}

	// verify effect has validators
	effectAttr, ok := setBlock.NestedObject.Attributes["effect"].(schema.StringAttribute)
	if !ok {
		t.Fatal("effect attribute is not a StringAttribute")
	}
	if len(effectAttr.Validators) == 0 {
		t.Error("effect attribute should have validators")
	}
}

func TestKubernetesNodePoolResource_AutoscalingConfigBlockExists(t *testing.T) {
	r := NewKubernetesNodePoolResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	block, ok := schemaResp.Schema.Blocks["autoscaling_config"]
	if !ok {
		t.Fatal("autoscaling_config block not found in schema")
	}

	nestedBlock, ok := block.(schema.SingleNestedBlock)
	if !ok {
		t.Fatal("autoscaling_config is not a SingleNestedBlock")
	}

	enabledAttr, ok := nestedBlock.Attributes["enabled"].(schema.BoolAttribute)
	if !ok {
		t.Fatal("enabled attribute is not a BoolAttribute")
	}
	// Optional in the schema (Required inside a SingleNestedBlock breaks
	// block removal); presence is enforced by ValidateConfig.
	if !enabledAttr.Optional || enabledAttr.Required {
		t.Error("enabled should be optional (presence enforced in ValidateConfig)")
	}

	// bounds are optional (omitted when disabling) and carry the shape
	// validators
	for _, field := range []string{"min_node_size", "max_node_size"} {
		boundAttr, ok := nestedBlock.Attributes[field].(schema.Int64Attribute)
		if !ok {
			t.Fatalf("%s attribute is not an Int64Attribute", field)
		}
		// bounds must be plain Optional: Computed here breaks
		// removal-means-disable (the framework plans a removed block as a
		// partially-unknown object instead of null)
		if !boundAttr.Optional || boundAttr.Computed {
			t.Errorf("%s should be optional and not computed", field)
		}
		if len(boundAttr.Validators) == 0 {
			t.Errorf("%s should have validators", field)
		}
	}
}

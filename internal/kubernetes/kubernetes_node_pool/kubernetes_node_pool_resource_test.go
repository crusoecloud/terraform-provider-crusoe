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

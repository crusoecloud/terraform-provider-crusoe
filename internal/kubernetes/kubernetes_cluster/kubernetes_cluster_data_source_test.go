package kubernetes_cluster

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestKubernetesClusterDataSource_Metadata(t *testing.T) {
	ds := NewKubernetesClusterDataSource()

	req := datasource.MetadataRequest{ProviderTypeName: "crusoe"}
	resp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), req, resp)

	expected := "crusoe_kubernetes_cluster"
	if resp.TypeName != expected {
		t.Errorf("TypeName: expected %q, got %q", expected, resp.TypeName)
	}
}

func TestKubernetesClusterDataSource_ExtraArgsSchemaAttributes(t *testing.T) {
	ds := NewKubernetesClusterDataSource()

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)

	for _, fieldName := range []string{"apiserver_extra_args", "scheduler_extra_args", "controller_manager_extra_args"} {
		mapAttr, ok := schemaResp.Schema.Attributes[fieldName].(schema.MapAttribute)
		if !ok {
			t.Fatalf("%s attribute not found or not MapAttribute", fieldName)
		}

		if !mapAttr.Computed {
			t.Errorf("%s should be Computed in the data source", fieldName)
		}

		if mapAttr.ElementType != types.StringType {
			t.Errorf("%s element type should be StringType", fieldName)
		}
	}
}

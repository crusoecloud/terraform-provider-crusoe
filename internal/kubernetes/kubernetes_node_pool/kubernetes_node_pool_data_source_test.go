package kubernetes_node_pool

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func TestKubernetesNodePoolDataSource_Metadata(t *testing.T) {
	ds := NewKubernetesNodePoolDataSource()

	req := datasource.MetadataRequest{ProviderTypeName: "crusoe"}
	resp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), req, resp)

	expected := "crusoe_kubernetes_node_pool"
	if resp.TypeName != expected {
		t.Errorf("TypeName: expected %q, got %q", expected, resp.TypeName)
	}
}

// TestKubernetesNodePoolDataSource_APIFieldsComputed verifies that fields
// returned by the API are Computed so Terraform persists them to state
// (CCX-2834).
func TestKubernetesNodePoolDataSource_APIFieldsComputed(t *testing.T) {
	ds := NewKubernetesNodePoolDataSource()

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)

	for _, fieldName := range []string{
		"image_id", "type", "instance_count", "cluster_id", "subnet_id",
		"node_labels", "instance_ids", "state", "name",
		"ephemeral_storage_for_containerd", "nvlink_domain_id", "public_ip_type",
	} {
		attr, found := schemaResp.Schema.Attributes[fieldName]
		if !found {
			t.Errorf("attribute %q not found", fieldName)

			continue
		}
		if !attr.IsComputed() {
			t.Errorf("attribute %q should be Computed in the data source", fieldName)
		}
	}
}

// TestKubernetesNodePoolDataSource_ImageIDNotMislabeled locks in the fix for the
// model/schema mismatch: the schema exposes image_id (not version), and the
// model field maps to it (CCX-2834).
func TestKubernetesNodePoolDataSource_ImageIDNotMislabeled(t *testing.T) {
	ds := NewKubernetesNodePoolDataSource()

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)

	if _, found := schemaResp.Schema.Attributes["image_id"]; !found {
		t.Error("expected image_id attribute in the data source schema")
	}
	if _, found := schemaResp.Schema.Attributes["version"]; found {
		t.Error("unexpected version attribute: node pool image field should be image_id, not version")
	}
}

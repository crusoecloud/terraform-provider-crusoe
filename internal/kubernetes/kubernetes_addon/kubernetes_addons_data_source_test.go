package kubernetes_addon

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func TestKubernetesAddonsDataSource_Metadata(t *testing.T) {
	ds := NewKubernetesAddonsDataSource()

	resp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "crusoe"}, resp)

	if expected := "crusoe_kubernetes_addons"; resp.TypeName != expected {
		t.Errorf("TypeName: expected %q, got %q", expected, resp.TypeName)
	}
}

func TestKubernetesAddonsDataSource_Schema(t *testing.T) {
	ds := NewKubernetesAddonsDataSource()
	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)
	attrs := schemaResp.Schema.Attributes

	if a, ok := attrs["cluster_id"]; !ok || !a.IsRequired() {
		t.Error("cluster_id should be Required")
	}
	if a, ok := attrs["project_id"]; !ok || !a.IsOptional() || !a.IsComputed() {
		t.Error("project_id should be Optional+Computed")
	}

	// Every nested add-on field must be Computed so Terraform persists it.
	addons, ok := attrs["addons"].(schema.ListNestedAttribute)
	if !ok {
		t.Fatal("addons attribute not found or not a ListNestedAttribute")
	}
	for _, f := range []string{"addon_type", "version", "installed_version", "status", "ready", "category"} {
		a, found := addons.NestedObject.Attributes[f]
		if !found {
			t.Errorf("nested attribute %q not found", f)

			continue
		}
		if !a.IsComputed() {
			t.Errorf("nested attribute %q should be Computed", f)
		}
	}
}

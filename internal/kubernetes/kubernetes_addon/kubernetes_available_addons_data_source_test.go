package kubernetes_addon

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func TestKubernetesAvailableAddonsDataSource_Metadata(t *testing.T) {
	ds := NewKubernetesAvailableAddonsDataSource()

	resp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), datasource.MetadataRequest{ProviderTypeName: "crusoe"}, resp)

	if expected := "crusoe_kubernetes_available_addons"; resp.TypeName != expected {
		t.Errorf("TypeName: expected %q, got %q", expected, resp.TypeName)
	}
}

func TestKubernetesAvailableAddonsDataSource_Schema(t *testing.T) {
	ds := NewKubernetesAvailableAddonsDataSource()
	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)
	attrs := schemaResp.Schema.Attributes

	if a, ok := attrs["project_id"]; !ok || !a.IsOptional() || !a.IsComputed() {
		t.Error("project_id should be Optional+Computed")
	}
	for _, f := range []string{"kubernetes_version", "cluster_id"} {
		a, ok := attrs[f]
		if !ok || !a.IsOptional() {
			t.Errorf("%q should be Optional", f)
		}
	}

	addons, ok := attrs["addons"].(schema.ListNestedAttribute)
	if !ok {
		t.Fatal("addons attribute not found or not a ListNestedAttribute")
	}
	for _, f := range []string{"addon_type", "description", "category", "removable", "versions"} {
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

// TestKubernetesAvailableAddonsDataSource_ConfigValidators verifies the
// exactly-one-of (kubernetes_version / cluster_id) validator is wired up.
func TestKubernetesAvailableAddonsDataSource_ConfigValidators(t *testing.T) {
	ds, ok := NewKubernetesAvailableAddonsDataSource().(datasource.DataSourceWithConfigValidators)
	if !ok {
		t.Fatal("data source should implement DataSourceWithConfigValidators")
	}
	if len(ds.ConfigValidators(context.Background())) == 0 {
		t.Error("expected at least one config validator (ExactlyOneOf)")
	}
}

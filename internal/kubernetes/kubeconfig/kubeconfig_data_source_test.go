package kubeconfig

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func TestKubeConfigDataSource_Metadata(t *testing.T) {
	ds := NewKubeConfigDataSource()

	req := datasource.MetadataRequest{ProviderTypeName: "crusoe"}
	resp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), req, resp)

	expected := "crusoe_kubeconfig"
	if resp.TypeName != expected {
		t.Errorf("TypeName: expected %q, got %q", expected, resp.TypeName)
	}
}

func TestKubeConfigDataSource_ClusterIDRequired(t *testing.T) {
	ds := NewKubeConfigDataSource()

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)

	attr, ok := schemaResp.Schema.Attributes["cluster_id"].(schema.StringAttribute)
	if !ok {
		t.Fatal("cluster_id attribute not found")
	}
	if !attr.Required {
		t.Error("cluster_id should be Required")
	}
}

func TestKubeConfigDataSource_ComputedFields(t *testing.T) {
	ds := NewKubeConfigDataSource()

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)

	for _, fieldName := range []string{
		"cluster_address", "cluster_ca_certificate", "cluster_name",
		"client_certificate", "client_key", "username", "kubeconfig_yaml",
	} {
		attr, found := schemaResp.Schema.Attributes[fieldName]
		if !found {
			t.Errorf("attribute %q not found", fieldName)

			continue
		}
		if !attr.IsComputed() {
			t.Errorf("attribute %q should be Computed", fieldName)
		}
	}
}

func TestKubeConfigDataSource_SensitiveFields(t *testing.T) {
	ds := NewKubeConfigDataSource()

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)

	for _, fieldName := range []string{"client_key", "kubeconfig_yaml"} {
		attr, ok := schemaResp.Schema.Attributes[fieldName].(schema.StringAttribute)
		if !ok {
			t.Fatalf("%s attribute not found or not StringAttribute", fieldName)
		}
		if !attr.Sensitive {
			t.Errorf("attribute %q should be Sensitive", fieldName)
		}
	}
}

func TestKubeConfigDataSource_AuthTypeValidator(t *testing.T) {
	ds := NewKubeConfigDataSource()

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)

	attr, ok := schemaResp.Schema.Attributes["auth_type"].(schema.StringAttribute)
	if !ok {
		t.Fatal("auth_type attribute not found")
	}
	if len(attr.Validators) == 0 {
		t.Error("auth_type should have validators")
	}
}

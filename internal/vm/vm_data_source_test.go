package vm

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func TestVMDataSource_Metadata(t *testing.T) {
	ds := NewVMDataSource()

	req := datasource.MetadataRequest{ProviderTypeName: "crusoe"}
	resp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), req, resp)

	expected := "crusoe_compute_instance"
	if resp.TypeName != expected {
		t.Errorf("TypeName: expected %q, got %q", expected, resp.TypeName)
	}
}

// TestVMDataSource_DisksComputed verifies that the disks list and its nested
// fields are Computed. disks is returned by the API, not a lookup filter, so it
// must be Computed for Terraform to persist it to state (CCX-2832).
func TestVMDataSource_DisksComputed(t *testing.T) {
	ds := NewVMDataSource()

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)

	disksAttr, ok := schemaResp.Schema.Attributes["disks"].(schema.ListNestedAttribute)
	if !ok {
		t.Fatal("disks attribute not found or not a ListNestedAttribute")
	}

	if !disksAttr.IsComputed() {
		t.Error("disks should be Computed in the data source")
	}

	for _, fieldName := range []string{"id", "attachment_type", "mode"} {
		attr, found := disksAttr.NestedObject.Attributes[fieldName]
		if !found {
			t.Errorf("disks nested field %q not found", fieldName)

			continue
		}
		if !attr.IsComputed() {
			t.Errorf("disks nested field %q should be Computed in the data source", fieldName)
		}
	}
}

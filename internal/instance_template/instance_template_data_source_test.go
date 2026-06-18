package instance_template

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

func TestInstanceTemplatesDataSource_Metadata(t *testing.T) {
	ds := NewInstanceTemplatesDataSource()

	req := datasource.MetadataRequest{ProviderTypeName: "crusoe"}
	resp := &datasource.MetadataResponse{}
	ds.Metadata(context.Background(), req, resp)

	expected := "crusoe_instance_templates"
	if resp.TypeName != expected {
		t.Errorf("TypeName: expected %q, got %q", expected, resp.TypeName)
	}
}

// TestInstanceTemplatesDataSource_NestedFieldsComputed verifies that every field
// inside the instance_templates list object is Computed. This is a list data
// source: all returned values come from the API, not user config, so each nested
// field must be Computed for Terraform to persist it to state (CCX-2831).
func TestInstanceTemplatesDataSource_NestedFieldsComputed(t *testing.T) {
	ds := NewInstanceTemplatesDataSource()

	schemaResp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, schemaResp)

	listAttr, ok := schemaResp.Schema.Attributes["instance_templates"].(schema.ListNestedAttribute)
	if !ok {
		t.Fatal("instance_templates attribute not found or not a ListNestedAttribute")
	}

	nested := listAttr.NestedObject.Attributes
	for _, fieldName := range []string{
		"id", "name", "project_id", "type", "ssh_key", "location", "image",
		"startup_script", "shutdown_script", "subnet", "ib_partition",
		"public_ip_address_type", "disks", "placement_policy", "nvlink_domain_id",
	} {
		attr, found := nested[fieldName]
		if !found {
			t.Errorf("nested field %q not found in instance_templates", fieldName)

			continue
		}
		if !attr.IsComputed() {
			t.Errorf("nested field %q should be Computed in the data source", fieldName)
		}
	}

	disksAttr, ok := nested["disks"].(schema.ListNestedAttribute)
	if !ok {
		t.Fatal("disks attribute not found or not a ListNestedAttribute")
	}
	for _, fieldName := range []string{"size", "type"} {
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

// TestInstanceTemplatesToModel_DisksScopedPerTemplate guards against a
// regression where disks were aggregated across all templates and assigned to
// every template. Each template must carry only its own disks.
func TestInstanceTemplatesToModel_DisksScopedPerTemplate(t *testing.T) {
	items := []swagger.InstanceTemplate{
		{
			Id:   "tmpl-with-disks",
			Name: "has-disks",
			Disks: []swagger.DiskTemplate{
				{Size: "1GiB", Type_: "persistent-ssd"},
				{Size: "5GiB", Type_: "persistent-ssd"},
			},
		},
		{
			Id:    "tmpl-no-disks",
			Name:  "no-disks",
			Disks: nil,
		},
	}

	got := instanceTemplatesToModel(items)

	if len(got) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(got))
	}

	if len(got[0].Disks) != 2 {
		t.Errorf("template with disks: expected 2 disks, got %d", len(got[0].Disks))
	}

	if len(got[1].Disks) != 0 {
		t.Errorf("template without disks: expected 0 disks, got %d (disks bled across templates)", len(got[1].Disks))
	}

	if got[0].Disks[0].Size != "1GiB" || got[0].Disks[0].Type != "persistent-ssd" {
		t.Errorf("unexpected first disk: %+v", got[0].Disks[0])
	}
}

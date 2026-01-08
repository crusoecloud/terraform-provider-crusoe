package instance_group

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func TestInstanceGroupsDataSource_Metadata(t *testing.T) {
	ds := NewInstanceGroupsDataSource()

	req := datasource.MetadataRequest{
		ProviderTypeName: "crusoe",
	}
	resp := &datasource.MetadataResponse{}

	ds.Metadata(context.Background(), req, resp)

	expected := "crusoe_compute_instance_groups"
	if resp.TypeName != expected {
		t.Errorf("TypeName: expected %q, got %q", expected, resp.TypeName)
	}
}

package instance_group

import (
	"context"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
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

// TestInstanceGroupToDataSourceModel_SortsInstanceIDs verifies that the active and
// inactive instance ID lists are sorted deterministically, preventing spurious
// diffs when the API re-orders its results (CCX-4394).
func TestInstanceGroupToDataSourceModel_SortsInstanceIDs(t *testing.T) {
	item := &swagger.InstanceGroup{
		Id:                "ig-1",
		Name:              "group",
		ActiveInstances:   []string{"vm-3", "vm-1", "vm-2"},
		InactiveInstances: []string{"vm-9", "vm-5"},
	}

	got := instanceGroupToDataSourceModel(item)

	if want := []string{"vm-1", "vm-2", "vm-3"}; !slices.Equal(got.ActiveInstanceIDs, want) {
		t.Errorf("ActiveInstanceIDs = %v, want %v", got.ActiveInstanceIDs, want)
	}
	if want := []string{"vm-5", "vm-9"}; !slices.Equal(got.InactiveInstanceIDs, want) {
		t.Errorf("InactiveInstanceIDs = %v, want %v", got.InactiveInstanceIDs, want)
	}
}

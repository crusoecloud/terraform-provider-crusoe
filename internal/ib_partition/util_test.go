package ib_partition

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

// Test_ibPartitionToTerraformResourceModel verifies the transform populates
// id/name/ib_network_id from the API response, overwriting whatever the plan
// held, so Create reflects the API instead of persisting (potentially
// normalized) plan values. It also leaves project_id untouched, since the
// caller sets that from the resolved project.
func Test_ibPartitionToTerraformResourceModel(t *testing.T) {
	state := &ibPartitionResourceModel{
		Name:        types.StringValue("planned-name"),
		IBNetworkID: types.StringValue("planned-network"),
		ProjectID:   types.StringValue("project-1"),
	}
	api := &swagger.IbPartition{
		Id:          "ibp-123",
		Name:        "api-name",
		IbNetworkId: "api-network",
	}

	ibPartitionToTerraformResourceModel(api, state)

	if got := state.ID.ValueString(); got != "ibp-123" {
		t.Errorf("id = %q, want %q", got, "ibp-123")
	}
	if got := state.Name.ValueString(); got != "api-name" {
		t.Errorf("name = %q, want %q (from API, not plan)", got, "api-name")
	}
	if got := state.IBNetworkID.ValueString(); got != "api-network" {
		t.Errorf("ib_network_id = %q, want %q (from API, not plan)", got, "api-network")
	}
	if got := state.ProjectID.ValueString(); got != "project-1" {
		t.Errorf("project_id = %q, want %q (untouched by transform)", got, "project-1")
	}
}

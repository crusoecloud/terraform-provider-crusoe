package vpc_network

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

// Test_vpcNetworkToTerraformResourceModel covers the shared transform that all
// CRUD paths now use: id/name/cidr/gateway are sourced from the API and subnet
// IDs are sorted deterministically (the CCX-4394 ordering guarantee), while
// project_id is left for the caller to set.
func Test_vpcNetworkToTerraformResourceModel(t *testing.T) {
	state := &vpcNetworkResourceModel{ProjectID: types.StringValue("project-1")}
	network := &swagger.VpcNetwork{
		Id:      "vpc-1",
		Name:    "my-vpc",
		Cidr:    "10.0.0.0/16",
		Gateway: "10.0.0.1",
		Subnets: []string{"subnet-c", "subnet-a", "subnet-b"},
	}

	vpcNetworkToTerraformResourceModel(network, state)

	if got := state.ID.ValueString(); got != "vpc-1" {
		t.Errorf("id = %q, want %q", got, "vpc-1")
	}
	if got := state.Name.ValueString(); got != "my-vpc" {
		t.Errorf("name = %q, want %q", got, "my-vpc")
	}
	if got := state.CIDR.ValueString(); got != "10.0.0.0/16" {
		t.Errorf("cidr = %q, want %q", got, "10.0.0.0/16")
	}
	if got := state.Gateway.ValueString(); got != "10.0.0.1" {
		t.Errorf("gateway = %q, want %q", got, "10.0.0.1")
	}
	if got := state.ProjectID.ValueString(); got != "project-1" {
		t.Errorf("project_id = %q, want %q (untouched by transform)", got, "project-1")
	}

	var gotSubnets []string
	if diags := state.Subnets.ElementsAs(context.Background(), &gotSubnets, false); diags.HasError() {
		t.Fatalf("reading subnets: %v", diags)
	}
	wantSubnets := []string{"subnet-a", "subnet-b", "subnet-c"}
	if !reflect.DeepEqual(gotSubnets, wantSubnets) {
		t.Errorf("subnets = %v, want %v (sorted)", gotSubnets, wantSubnets)
	}
}

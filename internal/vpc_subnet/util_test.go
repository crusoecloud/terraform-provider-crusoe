package vpc_subnet

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

// Test_vpcSubnetToTerraformResourceModel covers the shared transform that all
// CRUD paths now use: scalar fields are sourced from the API, the (Computed)
// nat_gateways list is sorted by ID for deterministic ordering (CCX-4394),
// nat_gateway_enabled is derived from gateway presence, and project_id is left
// for the caller to set.
func Test_vpcSubnetToTerraformResourceModel(t *testing.T) {
	ctx := context.Background()
	state := &vpcSubnetResourceModel{ProjectID: types.StringValue("project-1")}
	subnet := &swagger.VpcSubnet{
		Id:           "subnet-1",
		Name:         "my-subnet",
		Cidr:         "10.0.1.0/24",
		Location:     "us-east1-a",
		VpcNetworkId: "vpc-1",
		NatGateways: []swagger.NatGateway{
			{Id: "nat-c", PublicIpv4Address: "1.1.1.3", PublicIpv4Id: "ip-c"},
			{Id: "nat-a", PublicIpv4Address: "1.1.1.1", PublicIpv4Id: "ip-a"},
			{Id: "nat-b", PublicIpv4Address: "1.1.1.2", PublicIpv4Id: "ip-b"},
		},
	}

	var diags diag.Diagnostics
	vpcSubnetToTerraformResourceModel(ctx, subnet, state, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got := state.ID.ValueString(); got != "subnet-1" {
		t.Errorf("id = %q, want %q", got, "subnet-1")
	}
	if got := state.Network.ValueString(); got != "vpc-1" {
		t.Errorf("network = %q, want %q", got, "vpc-1")
	}
	if !state.NATGatewayEnabled.ValueBool() {
		t.Error("nat_gateway_enabled = false, want true (gateways present)")
	}
	if got := state.ProjectID.ValueString(); got != "project-1" {
		t.Errorf("project_id = %q, want %q (untouched by transform)", got, "project-1")
	}

	var gws []vpcSubnetNatGatewayResourceModel
	if d := state.NATGateways.ElementsAs(ctx, &gws, false); d.HasError() {
		t.Fatalf("reading nat_gateways: %v", d)
	}
	gotIDs := make([]string, len(gws))
	for i, g := range gws {
		gotIDs[i] = g.ID.ValueString()
	}
	wantIDs := []string{"nat-a", "nat-b", "nat-c"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Errorf("nat gateway ids = %v, want %v (sorted)", gotIDs, wantIDs)
	}
}

// Test_vpcSubnetToTerraformResourceModel_noGateways confirms nat_gateway_enabled
// is false when the API returns no NAT gateways.
func Test_vpcSubnetToTerraformResourceModel_noGateways(t *testing.T) {
	ctx := context.Background()
	state := &vpcSubnetResourceModel{}
	subnet := &swagger.VpcSubnet{Id: "subnet-1"}

	var diags diag.Diagnostics
	vpcSubnetToTerraformResourceModel(ctx, subnet, state, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if state.NATGatewayEnabled.ValueBool() {
		t.Error("nat_gateway_enabled = true, want false (no gateways)")
	}
}

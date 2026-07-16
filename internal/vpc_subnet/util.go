package vpc_subnet

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec (VpcSubnet;
// nested NatGateway; nat_gateway_enabled from VpcSubnetPostRequest).
const (
	apiDescID                = "ID of the VPC subnet."
	apiDescName              = "Name of the VPC subnet."
	apiDescCIDR              = "Address range of the VPC subnet, in CIDR notation."
	apiDescLocation          = "Location of the VPC subnet."
	apiDescNetwork           = "ID of the VPC network that the subnet belongs to."
	apiDescNATGatewayEnabled = "Whether to create a NAT gateway for the subnet."
	apiDescNATGateways       = "NAT gateways attached to the subnet. Empty unless a NAT gateway is enabled for the subnet."

	apiDescNATGatewayID                = "ID of the NAT gateway."
	apiDescNATGatewayPublicIPv4Address = "Public IPv4 address assigned to the NAT gateway."
	apiDescNATGatewayPublicIPv4ID      = "ID of the public IPv4 address assigned to the NAT gateway."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project the VPC subnet belongs to. " + project.ProviderDescProjectIDFallback
)

var vpcSubnetNatGatewaySchema = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"id":                  types.StringType,
		"public_ipv4_address": types.StringType,
		"public_ipv4_id":      types.StringType,
	},
}

func findVpcSubnet(ctx context.Context, client *swagger.APIClient, vpcSubnetID string) (*swagger.VpcSubnet, string, error) {
	args := common.FindResourceArgs[swagger.VpcSubnet]{
		ResourceID:  vpcSubnetID,
		GetResource: client.VPCSubnetsApi.GetVPCSubnet,
		IsResource: func(subnet swagger.VpcSubnet, id string) bool {
			return subnet.Id == id
		},
	}

	return common.FindResource[swagger.VpcSubnet](ctx, client, args)
}

func vpcSubnetToTerraformResourceModel(ctx context.Context, vpcSubnet *swagger.VpcSubnet, state *vpcSubnetResourceModel, diags *diag.Diagnostics) {
	state.ID = types.StringValue(vpcSubnet.Id)
	state.Name = types.StringValue(vpcSubnet.Name)
	state.CIDR = types.StringValue(vpcSubnet.Cidr)
	state.Location = types.StringValue(vpcSubnet.Location)
	state.Network = types.StringValue(vpcSubnet.VpcNetworkId)
	natGatewaysList, natDiags := natGatewaysToTerraformResourceModel(ctx, vpcSubnet.NatGateways)
	state.NATGateways = natGatewaysList
	state.NATGatewayEnabled = types.BoolValue(len(natGatewaysList.Elements()) > 0)
	diags.Append(natDiags...)
}

func natGatewaysToTerraformResourceModel(ctx context.Context, natGateways []swagger.NatGateway) (types.List, diag.Diagnostics) {
	gateways := make([]vpcSubnetNatGatewayResourceModel, 0, len(natGateways))
	for _, gateway := range natGateways {
		gateways = append(gateways, vpcSubnetNatGatewayResourceModel{
			ID:                types.StringValue(gateway.Id),
			PublicIpv4Address: types.StringValue(gateway.PublicIpv4Address),
			PublicIpv4Id:      types.StringValue(gateway.PublicIpv4Id),
		})
	}

	// Sort by ID for deterministic ordering; the API does not guarantee a stable
	// order for the (Computed) NAT gateway list.
	common.SortByKeys(gateways, func(g vpcSubnetNatGatewayResourceModel) string { return g.ID.ValueString() })

	return types.ListValueFrom(ctx, vpcSubnetNatGatewaySchema, gateways)
}

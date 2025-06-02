package vpc_subnet

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
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

	return types.ListValueFrom(ctx, vpcSubnetNatGatewaySchema, gateways)
}

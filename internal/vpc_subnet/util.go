package vpc_subnet

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

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

func vpcSubnetToTerraformResourceModel(vpcSubnet *swagger.VpcSubnet, state *vpcSubnetResourceModel) {
	state.ID = types.StringValue(vpcSubnet.Id)
	state.Name = types.StringValue(vpcSubnet.Name)
	state.CIDR = types.StringValue(vpcSubnet.Cidr)
	state.Location = types.StringValue(vpcSubnet.Location)
	state.Network = types.StringValue(vpcSubnet.VpcNetworkId)
}

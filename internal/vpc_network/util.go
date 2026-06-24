package vpc_network

import (
	"context"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

func findVpcNetwork(ctx context.Context, client *swagger.APIClient, vpcNetworkID string) (*swagger.VpcNetwork, string, error) {
	args := common.FindResourceArgs[swagger.VpcNetwork]{
		ResourceID:  vpcNetworkID,
		GetResource: client.VPCNetworksApi.GetVPCNetwork,
		IsResource: func(network swagger.VpcNetwork, id string) bool {
			return network.Id == id
		},
	}

	return common.FindResource[swagger.VpcNetwork](ctx, client, args)
}

func vpcNetworkToTerraformResourceModel(vpcNetwork *swagger.VpcNetwork, state *vpcNetworkResourceModel) {
	state.ID = types.StringValue(vpcNetwork.Id)
	state.Name = types.StringValue(vpcNetwork.Name)
	state.CIDR = types.StringValue(vpcNetwork.Cidr)
	state.Gateway = types.StringValue(vpcNetwork.Gateway)
	// Sort subnet IDs for deterministic ordering; the API does not guarantee a stable order.
	slices.Sort(vpcNetwork.Subnets)
	subnets, _ := types.ListValueFrom(context.Background(), types.StringType, vpcNetwork.Subnets)
	state.Subnets = subnets
}

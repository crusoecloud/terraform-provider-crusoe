package vpc_network

import (
	"context"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec (VpcNetwork).
const (
	apiDescID      = "ID of the VPC network."
	apiDescName    = "Name of the VPC network."
	apiDescCIDR    = "Address range of the VPC network, in CIDR notation."
	apiDescGateway = "ID of the VPC network's gateway."
	apiDescSubnets = "IDs of the subnets that belong to the VPC network. Empty if the network has none."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project the VPC network belongs to. " + project.ProviderDescProjectIDFallback
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

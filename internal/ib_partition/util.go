package ib_partition

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec (IbPartition).
const (
	apiDescID          = "ID of the InfiniBand partition."
	apiDescName        = "Name of the InfiniBand partition."
	apiDescIBNetworkID = "ID of the InfiniBand network the partition belongs to."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project the InfiniBand partition belongs to. " + project.ProviderDescProjectIDFallback
)

func findIbPartition(ctx context.Context, client *swagger.APIClient, ibPartitionID string) (*swagger.IbPartition, string, error) {
	args := common.FindResourceArgs[swagger.IbPartition]{
		ResourceID:  ibPartitionID,
		GetResource: client.IBPartitionsApi.GetIBPartition,
		IsResource: func(ibPartition swagger.IbPartition, id string) bool {
			return ibPartition.Id == id
		},
	}

	return common.FindResource[swagger.IbPartition](ctx, client, args)
}

func ibPartitionToTerraformResourceModel(ibPartition *swagger.IbPartition, state *ibPartitionResourceModel) {
	state.ID = types.StringValue(ibPartition.Id)
	state.Name = types.StringValue(ibPartition.Name)
	state.IBNetworkID = types.StringValue(ibPartition.IbNetworkId)
}

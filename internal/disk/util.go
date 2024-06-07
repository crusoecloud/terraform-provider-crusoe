package disk

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

func findDisk(ctx context.Context, client *swagger.APIClient, diskID string) (*swagger.DiskV1Alpha5, string, error) {
	args := common.FindResourceArgs[swagger.DiskV1Alpha5]{
		ResourceID:  diskID,
		GetResource: client.DisksApi.GetDisk,
		IsResource: func(disk swagger.DiskV1Alpha5, id string) bool {
			return disk.Id == id
		},
	}

	return common.FindResource[swagger.DiskV1Alpha5](ctx, client, args)
}

func diskToTerraformResourceModel(disk *swagger.DiskV1Alpha5, state *diskResourceModel) {
	state.ID = types.StringValue(disk.Id)
	state.Name = types.StringValue(disk.Name)
	state.Location = types.StringValue(disk.Location)
	state.Type = types.StringValue(disk.Type_)
	state.Size = types.StringValue(disk.Size)
	state.SerialNumber = types.StringValue(disk.SerialNumber)
	state.BlockSize = types.Int64Value(disk.BlockSize)
}

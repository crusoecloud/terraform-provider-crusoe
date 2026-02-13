package disk

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

const gibInTib = 1024

// Shared schema descriptions for resource and data source
const (
	descID                 = "Unique identifier of the disk."
	descName               = "Name of the disk."
	descProjectID          = "ID of the project the disk belongs to."
	descProjectIDInference = "If not specified, the project ID will be inferred from the Crusoe configuration."
	descType               = "Type of the disk. Possible values: `persistent-ssd`, `shared-volume`."
	descTypeRequired       = "This field will be required in a future release."
	descSize               = "Storage capacity of the disk (e.g., `100GiB`, `1TiB`)."
	descLocation           = "Location where the disk is deployed."
	descSerialNumber       = "Serial number assigned to the disk."
	descBlockSize          = "Block size of the disk in bytes. Possible values: `512`, `4096`."
)

var errGetResourceModel = errors.New("unable to get resource model")

// tfDataGetter is implemented by tfsdk.State and tfsdk.Plan
type tfDataGetter interface {
	Get(ctx context.Context, target interface{}) diag.Diagnostics
}

// getResourceModel extracts the resource model from state or plan.
// Returns errGetResourceModel if there were errors (diagnostics already appended to respDiags).
func getResourceModel(ctx context.Context, source tfDataGetter, dest *diskResourceModel, respDiags *diag.Diagnostics) error {
	diags := source.Get(ctx, dest)
	respDiags.Append(diags...)

	if respDiags.HasError() {
		return errGetResourceModel
	}

	return nil
}

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

func diskToTerraformResourceModel(disk *swagger.DiskV1Alpha5, state *diskResourceModel, sizeFormat string) {
	state.ID = types.StringValue(disk.Id)
	state.Name = types.StringValue(disk.Name)
	state.Location = types.StringValue(disk.Location)
	state.Type = types.StringValue(disk.Type_)
	state.Size = types.StringValue(preserveSizeFormat(sizeFormat, disk.Size))
	state.SerialNumber = types.StringValue(disk.SerialNumber)
	state.BlockSize = types.Int64Value(disk.BlockSize)
}

// preserveSizeFormat converts apiSize to match the user's preferred unit (TiB vs GiB)
// when the values are semantically equivalent. Returns apiSize unchanged if userFormat
// is empty or conversion isn't possible.
func preserveSizeFormat(userFormat, apiSize string) string {
	if userFormat == "" {
		return apiSize
	}

	userUnit := strings.ToLower(userFormat[len(userFormat)-3:])
	apiUnit := strings.ToLower(apiSize[len(apiSize)-3:])

	// Already same unit
	if userUnit == apiUnit {
		return apiSize
	}

	// User wants TiB, API returned GiB → convert if evenly divisible
	if userUnit == "tib" && apiUnit == "gib" {
		if gib, err := strconv.Atoi(apiSize[:len(apiSize)-3]); err == nil &&
			gib >= gibInTib && gib%gibInTib == 0 {

			return strconv.Itoa(gib/gibInTib) + "TiB"
		}
	}

	// User wants GiB, API returned TiB → convert
	if userUnit == "gib" && apiUnit == "tib" {
		if tib, err := strconv.Atoi(apiSize[:len(apiSize)-3]); err == nil {
			return strconv.Itoa(tib*gibInTib) + "GiB"
		}
	}

	return apiSize
}

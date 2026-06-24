package disk

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
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
	descDNSName            = "DNS name used to mount the shared volume. Populated only for `shared-volume` disks; empty for other disk types."
	descVips               = "Virtual IP addresses used to mount the shared volume. Populated only for `shared-volume` disks; empty for other disk types."
)

func findDisk(ctx context.Context, client *swagger.APIClient, diskID string) (*swagger.DiskV1, string, error) {
	args := common.FindResourceArgs[swagger.DiskV1]{
		ResourceID:  diskID,
		GetResource: client.DisksApi.GetDisk,
		IsResource: func(disk swagger.DiskV1, id string) bool {
			return disk.Id == id
		},
	}

	return common.FindResource[swagger.DiskV1](ctx, client, args)
}

func diskToTerraformResourceModel(disk *swagger.DiskV1, state *diskResourceModel, sizeFormat string) {
	state.ID = types.StringValue(disk.Id)
	state.Name = types.StringValue(disk.Name)
	state.Location = types.StringValue(disk.Location)
	state.Type = types.StringValue(disk.Type_)
	state.Size = types.StringValue(preserveSizeFormat(sizeFormat, disk.Size))
	state.SerialNumber = types.StringValue(disk.SerialNumber)
	state.BlockSize = types.Int64Value(disk.BlockSize)
	state.DNSName = types.StringValue(disk.DnsName)
	// Sort VIPs for deterministic ordering; the API does not guarantee a stable order.
	slices.Sort(disk.Vips)
	state.Vips = stringSliceToList(disk.Vips)
}

// stringSliceToList converts a Go slice of strings into a Terraform list value.
// A nil or empty slice maps to an empty (non-null) list so the attribute is always known.
func stringSliceToList(s []string) types.List {
	if len(s) == 0 {
		return types.ListValueMust(types.StringType, []attr.Value{})
	}

	return types.ListValueMust(types.StringType, stringsToAttrValues(s))
}

func stringsToAttrValues(s []string) []attr.Value {
	out := make([]attr.Value, len(s))
	for i, v := range s {
		out[i] = types.StringValue(v)
	}

	return out
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

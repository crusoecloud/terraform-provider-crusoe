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
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

const gibInTib = 1024

// apiDesc* — schema descriptions derived from the client-go swagger spec (DiskV1).
const (
	apiDescID           = "ID of the disk."
	apiDescName         = "Name of the disk."
	apiDescType         = "Type of the disk. Possible values: `persistent-ssd`, `shared-volume`."
	apiDescSize         = "Storage capacity of the disk, given as a size and unit in the format `[Size][Unit]`, for example `100GiB` or `1TiB`."
	apiDescLocation     = "Location where the disk is provisioned."
	apiDescSerialNumber = "Serial number assigned to the disk."
	apiDescBlockSize    = "Block size of the disk, in bytes. Possible values: `512`, `4096`."
	apiDescDNSName      = "DNS name used to mount the disk. Populated only for `shared-volume` disks."
	apiDescVips         = "Virtual IP addresses used to mount the disk. Populated only for `shared-volume` disks."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID         = "ID of the project the disk belongs to. " + project.ProviderDescProjectIDFallback
	providerDescTypeRequired      = "This field will be required in a future release."
	providerDescSharedVolumeEmpty = "Empty for other disk types."
	providerDescDisks             = "List of disks in the project."
)

// blockSizeDeprecationMessage marks the deprecated block_size attribute on both
// the resource and the data source. All persistent disks now use a 512-byte block
// size; any value set on create is ignored. Shared by both schemas so the wording
// stays consistent.
var blockSizeDeprecationMessage = common.FormatDeprecation("v0.6.0") +
	" All persistent disks now use a 512-byte block size; any value set here is ignored."

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
	// block_size is deprecated and intentionally not sourced from the API here; callers
	// preserve the planned/prior value via preserveDeprecatedBlockSize so the deprecated
	// attribute never triggers a spurious replacement when the backend standardizes it.
	state.DNSName = types.StringValue(disk.DnsName)
	// Sort VIPs for deterministic ordering; the API does not guarantee a stable order.
	slices.Sort(disk.Vips)
	state.Vips = stringSliceToList(disk.Vips)
}

// preserveDeprecatedBlockSize keeps a user-configured block_size value in state.
// block_size is deprecated and no longer sent on create, so retaining the planned
// (Create) or prior-state (Read) value keeps Terraform's Computed-consistency check
// satisfied and prevents the RequiresReplaceIfConfigured plan modifier from spuriously
// replacing the disk if the backend reports a different block size. When the value is
// unset (e.g. omitted on create, or a freshly imported disk), it reflects what the API
// assigned.
func preserveDeprecatedBlockSize(planned types.Int64, apiBlockSize int64) types.Int64 {
	if planned.IsNull() || planned.IsUnknown() {
		return types.Int64Value(apiBlockSize)
	}

	return planned
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

package instance_template

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec (InstanceTemplate).
const (
	apiDescID                  = "ID of the instance template."
	apiDescName                = "Name of the instance template. (This is not the name of the VMs created from this instance template.)"
	apiDescType                = "Product name of the VM type we want to create from this instance template."
	apiDescSSHKey              = "SSH public key to use for all VMs created from this instance template."
	apiDescLocation            = "Location to use for all VMs created from this instance template. May be empty if we do not want to bind this template to a location."
	apiDescImage               = "OS Image to use for all VMs created from this instance template."
	apiDescStartupScript       = "Startup script to use for all VMs created from this instance template."
	apiDescShutdownScript      = "Shutdown script to use for all VMs created from this instance template."
	apiDescSubnet              = "SubnetID to use for all VMs created from this instance template. Only used if template has a location."
	apiDescPublicIPAddressType = "Public IP address type to use for all VMs created from this instance template. Must either be `static` or `dynamic`."
	apiDescPlacementPolicy     = "Placement policy controlling how VMs created from this instance template are distributed across hosts. Possible values: `spread`, `unspecified`."
	apiDescDisks               = "Disks attached to all VMs created from this instance template."
	apiDescNvlinkDomainID      = "NVLink domain assigned to all VMs created from this instance template."

	// Nested DiskTemplate attributes.
	apiDescDiskSize = "Size of the disk, including a unit suffix."
	apiDescDiskType = "Type of disk to create. Possible values: `persistent-ssd`, `shared-volume`."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project this instance template belongs to. " + project.ProviderDescProjectIDFallback

	// providerDescReservationID is provider-side deprecation/behavior text for the
	// resource-only, plan-owned reservation_id attribute. It is intentionally not
	// sourced from the spec.
	providerDescReservationID = "(Deprecated) ID of the reservation to which the VM belongs. If not provided or null, the lowest-cost reservation will be used by default. To opt out of using a reservation, set this to an empty string."
)

// instanceTemplateToResourceModel maps an API instance template onto model, with
// the API object as the source of truth. Create and Read both call it so their
// mappings cannot drift apart.
//
// Every API-backed field comes from the response: disks are read from the response
// rather than rebuilt from the create request, the nullable fields (ib_partition,
// startup_script, shutdown_script, nvlink_domain_id) are null-normalized, and an
// empty placement_policy falls back to "unspecified".
//
// The deprecated, plan-owned reservation_id is left untouched: Create handles its
// own deprecation logic and Read preserves the prior-state value.
func instanceTemplateToResourceModel(ctx context.Context, template *swagger.InstanceTemplate,
	model *instanceTemplateResourceModel, diags *diag.Diagnostics,
) {
	model.ID = types.StringValue(template.Id)
	model.ProjectID = types.StringValue(template.ProjectId)
	model.Name = types.StringValue(template.Name)
	model.Type = types.StringValue(template.Type_)
	model.Location = types.StringValue(template.Location)
	model.Image = types.StringValue(template.ImageName)
	model.SSHKey = types.StringValue(template.SshPublicKey)
	model.Subnet = types.StringValue(template.SubnetId)
	model.PublicIpAddressType = types.StringValue(template.PublicIpAddressType)

	model.IBPartition = stringOrNull(template.IbPartitionId)
	model.StartupScript = stringOrNull(template.StartupScript)
	model.ShutdownScript = stringOrNull(template.ShutdownScript)
	model.NvlinkDomainID = stringOrNull(template.NvlinkDomainId)

	if template.PlacementPolicy != "" {
		model.PlacementPolicy = types.StringValue(template.PlacementPolicy)
	} else {
		model.PlacementPolicy = types.StringValue(unspecifiedPlacementPolicy)
	}

	model.DisksToCreate = disksToSet(ctx, template.Disks, model.DisksToCreate, diags)
}

// stringOrNull maps an empty API string to a null value, matching how the
// nullable, Optional attributes are represented in Terraform state.
func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}

	return types.StringValue(s)
}

// disksToSet builds the disks Set from the API response. When the template has
// no disks it preserves the caller's null-vs-empty intent (current is the plan
// value in Create and the prior-state value in Read).
func disksToSet(ctx context.Context, apiDisks []swagger.DiskTemplate, current types.Set,
	diags *diag.Diagnostics,
) types.Set {
	if len(apiDisks) == 0 {
		if current.IsNull() {
			return types.SetNull(diskToCreateSchema)
		}

		empty, d := types.SetValueFrom(ctx, diskToCreateSchema, []diskToCreateResourceModel{})
		diags.Append(d...)

		return empty
	}

	disks := make([]diskToCreateResourceModel, 0, len(apiDisks))
	for i := range apiDisks {
		disks = append(disks, diskToCreateResourceModel{
			Size: types.StringValue(apiDisks[i].Size),
			Type: types.StringValue(apiDisks[i].Type_),
		})
	}

	set, d := types.SetValueFrom(ctx, diskToCreateSchema, disks)
	diags.Append(d...)

	return set
}

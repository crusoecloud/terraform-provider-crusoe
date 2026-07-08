package instance_template

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
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

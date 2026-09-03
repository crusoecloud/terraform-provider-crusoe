package instance_template

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec (InstanceTemplate).
const (
	apiDescID                   = "ID of the instance template."
	apiDescName                 = "Name of the instance template. (This is not the name of the VMs created from this instance template.)"
	apiDescType                 = "Product name of the VM type we want to create from this instance template."
	apiDescSSHKey               = "SSH public key to use for all VMs created from this instance template."
	apiDescLocation             = "Location to use for all VMs created from this instance template. May be empty if we do not want to bind this template to a location."
	apiDescImage                = "OS Image to use for all VMs created from this instance template."
	apiDescStartupScript        = "Startup script to use for all VMs created from this instance template."
	apiDescShutdownScript       = "Shutdown script to use for all VMs created from this instance template."
	apiDescSubnet               = "SubnetID to use for all VMs created from this instance template. Only used if template has a location."
	apiDescTransportPartitionID = "IB or RoCE partition to use for all VMs created from this instance template. Only used for transport-enabled VM types. Empty if template has no location."
	apiDescPublicIPAddressType  = "Public IP address type to use for all VMs created from this instance template. Must either be `static` or `dynamic`."
	apiDescPlacementPolicy      = "Placement policy controlling how VMs created from this instance template are distributed across hosts. Possible values: `spread`, `unspecified`."
	apiDescDisks                = "Disks attached to all VMs created from this instance template."
	apiDescNvlinkDomainID       = "NVLink domain assigned to all VMs created from this instance template."

	// Nested DiskTemplate attributes.
	apiDescDiskSize                = "Size of the disk, including a unit suffix."
	apiDescDiskType                = "Type of disk to create. Possible values: `persistent-ssd`, `shared-volume`."
	apiDescSharedVolumeAttachments = "IDs of existing shared disks to attach to every VM created from this instance template. Attached read-write."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project this instance template belongs to. " + project.ProviderDescProjectIDFallback

	// providerDescReservationID is provider-side deprecation/behavior text for the
	// resource-only, plan-owned reservation_id attribute. It is intentionally not
	// sourced from the spec.
	providerDescReservationID = "(Deprecated) ID of the reservation to which the VM belongs. If not provided or null, the lowest-cost reservation will be used by default. To opt out of using a reservation, set this to an empty string."
)

// providerDescIBPartitionDeprecated marks ib_partition as replaced by
// transport_partition_id. The spec does not describe ib_partition, so this
// text is provider-side only.
var providerDescIBPartitionDeprecated = common.FormatDeprecationWithReplacement("v1.3.0", "transport_partition_id")

// The two names of the partition alias pair. Both the schema and Update refer to these,
// so the pair is described in one place.
const (
	attrIBPartition          = "ib_partition"
	attrTransportPartitionID = "transport_partition_id"
)

const aliasPairReplaceDescription = "Recreates the instance template when the partition changes. " +
	"Renaming ib_partition to transport_partition_id is not a change."

// aliasPairReplaceIfModifier builds the plan modifier both halves of the partition alias
// pair share.
func aliasPairReplaceIfModifier() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(
		common.AliasPairRequiresReplaceIf(attrIBPartition, attrTransportPartitionID),
		aliasPairReplaceDescription,
		aliasPairReplaceDescription,
	)
}

// instanceTemplateToResourceModel maps an API instance template onto model, with
// the API object as the source of truth. Create and Read both call it so their
// mappings cannot drift apart.
//
// Every API-backed field comes from the response: disks are read from the response
// rather than rebuilt from the create request, the nullable fields (startup_script,
// shutdown_script, nvlink_domain_id) are null-normalized, and an empty
// placement_policy falls back to "unspecified".
//
// Two groups of attributes are plan-owned and left untouched here. The deprecated
// reservation_id is handled by Create, and Read preserves the prior-state value. The
// ib_partition and transport_partition_id alias pair names one partition under two
// names, so the half the configuration uses has to survive: both are Optional and not
// Computed, which makes their planned value a known null, and writing an API value over
// that null on create fails Terraform's post-apply consistency check. Read calls
// hydrateAliasPairForImport to cover the one case with no planned value.
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

	model.StartupScript = stringOrNull(template.StartupScript)
	model.ShutdownScript = stringOrNull(template.ShutdownScript)
	model.NvlinkDomainID = stringOrNull(template.NvlinkDomainId)

	if template.PlacementPolicy != "" {
		model.PlacementPolicy = types.StringValue(template.PlacementPolicy)
	} else {
		model.PlacementPolicy = types.StringValue(unspecifiedPlacementPolicy)
	}

	model.DisksToCreate = disksToSet(ctx, template.Disks, model.DisksToCreate, diags)
	model.SharedVolumes = stringsToSet(ctx, template.SharedVolumeAttachments, model.SharedVolumes, diags)
}

// validateAliasPairConfig rejects a configuration that gives the two names of the
// partition alias pair different values, which does not say which partition to use.
//
// Setting both to the same value stays legal, so a configuration written during the
// migration keeps working. Only a conflict is an error.
func validateAliasPairConfig(deprecated, replacement types.String, diags *diag.Diagnostics) {
	if !common.AliasPairConflicts(deprecated, replacement) {
		return
	}

	diags.AddAttributeError(
		path.Root(attrTransportPartitionID),
		"Conflicting partition configuration",
		fmt.Sprintf("%s is %q and %s is %q. The two attributes name the same partition, so they "+
			"cannot be set to different values. Remove %s and keep %s.",
			attrIBPartition, deprecated.ValueString(),
			attrTransportPartitionID, replacement.ValueString(),
			attrIBPartition, attrTransportPartitionID),
	)
}

// ErrAliasOnlyChange reports that aliasOnlyChange could not compare the plan and state.
// The diagnostics carry the reason.
var ErrAliasOnlyChange = errors.New("unable to compare plan and state")

// aliasOnlyChange reports whether plan and state are identical once the partition alias
// pair is masked out of both. It compares the raw values rather than the resource models,
// because instanceTemplateResourceModel holds Set fields and is not comparable.
func aliasOnlyChange(ctx context.Context, plan *tfsdk.Plan, state *tfsdk.State,
	diags *diag.Diagnostics,
) (bool, error) {
	maskedPlan := *plan
	maskedState := *state

	for _, name := range []string{attrIBPartition, attrTransportPartitionID} {
		attrPath := path.Root(name)
		diags.Append(maskedPlan.SetAttribute(ctx, attrPath, (*string)(nil))...)
		diags.Append(maskedState.SetAttribute(ctx, attrPath, (*string)(nil))...)
	}

	if diags.HasError() {
		return false, ErrAliasOnlyChange
	}

	return maskedPlan.Raw.Equal(maskedState.Raw), nil
}

// hydrateAliasPairForImport fills transport_partition_id from the API when neither half of
// the alias pair is in state. That happens only for a freshly imported template, which has
// no configuration to own the value.
//
// Otherwise the pair is left alone. It is plan-owned, so re-reading it from the API would
// fight the alias plan modifier and leave a permanent diff for a configuration still on
// ib_partition. Only the replacement name is filled in, so an import never puts a value on
// the deprecated attribute.
//
// This belongs in Read and not in Create: Terraform does not consistency-check a refresh.
func hydrateAliasPairForImport(model *instanceTemplateResourceModel, template *swagger.InstanceTemplate) {
	if !model.IBPartition.IsNull() || !model.TransportPartitionID.IsNull() {
		return
	}

	model.TransportPartitionID = stringOrNull(
		common.EffectiveAliasString(template.IbPartitionId, template.TransportPartitionId))
}

// stringsToSet builds a string Set from the API response, preserving the caller's
// null-vs-empty intent when the list is empty.
func stringsToSet(ctx context.Context, values []string, current types.Set,
	diags *diag.Diagnostics,
) types.Set {
	if len(values) == 0 && current.IsNull() {
		return types.SetNull(types.StringType)
	}

	set, d := types.SetValueFrom(ctx, types.StringType, values)
	diags.Append(d...)

	return set
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

	formats := configuredSizeFormats(ctx, current)

	disks := make([]diskToCreateResourceModel, 0, len(apiDisks))
	for i := range apiDisks {
		// Preserve the user's size format (e.g., TiB, GiB) to avoid Terraform
		// treating it as a change when the API normalizes to GiB.
		size := common.PreserveSizeFormat(formats.take(apiDisks[i].Size), apiDisks[i].Size)
		disks = append(disks, diskToCreateResourceModel{
			Size: types.StringValue(size),
			Type: types.StringValue(apiDisks[i].Type_),
		})
	}

	set, d := types.SetValueFrom(ctx, diskToCreateSchema, disks)
	diags.Append(d...)

	return set
}

type sizeFormats []string

// take returns the configured size matching apiSize's capacity and consumes it, or "" if none.
func (f sizeFormats) take(apiSize string) string {
	want, ok := common.StorageSizeInGiB(apiSize)
	if !ok {
		return ""
	}

	for i, configured := range f {
		if configured == "" {
			continue
		}
		if got, ok := common.StorageSizeInGiB(configured); ok && got == want {
			f[i] = ""

			return configured
		}
	}

	return ""
}

// configuredSizeFormats extracts the user's disk sizes (plan in Create, prior state in Read)
func configuredSizeFormats(ctx context.Context, current types.Set) sizeFormats {
	if current.IsNull() || current.IsUnknown() {
		return nil
	}

	configured := make([]diskToCreateResourceModel, 0, len(current.Elements()))
	if d := current.ElementsAs(ctx, &configured, true); d.HasError() {
		return nil
	}

	formats := make(sizeFormats, 0, len(configured))
	for i := range configured {
		if configured[i].Size.IsNull() || configured[i].Size.IsUnknown() {
			formats = append(formats, "")

			continue
		}
		formats = append(formats, configured[i].Size.ValueString())
	}

	return formats
}

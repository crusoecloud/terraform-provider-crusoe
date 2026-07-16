package instance_group

import (
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec (InstanceGroup).
const (
	apiDescID                   = "ID of the instance group."
	apiDescName                 = "Name of the instance group."
	apiDescInstanceTemplateID   = "ID of the instance template currently associated with the instance group."
	apiDescRunningInstanceCount = "Number of running instances currently in the instance group."
	apiDescDesiredCount         = "Desired number of instances for the instance group."
	apiDescState                = "Current state of the instance group."
	apiDescActiveInstanceIDs    = "List of IDs of running instances in the instance group."
	apiDescInactiveInstanceIDs  = "List of IDs of non-running instances in the instance group."
	apiDescCreatedAt            = "Creation timestamp of the instance group, in RFC3339 format."
	apiDescUpdatedAt            = "Last update timestamp of the instance group, in RFC3339 format."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID      = "ID of the project that owns the instance group. " + project.ProviderDescProjectIDFallback
	providerDescState          = "Possible values: `HEALTHY` (matches desired count), `UPDATING` (scaling in progress), `UNHEALTHY` (cannot reach desired count)."
	providerDescInstanceGroups = "List of instance groups in the project."
)

func instanceGroupToResourceModel(instanceGroup *swagger.InstanceGroup, state *instanceGroupResourceModel, diags *diag.Diagnostics) {
	state.ID = types.StringValue(instanceGroup.Id)
	state.ProjectID = types.StringValue(instanceGroup.ProjectId)
	state.Name = types.StringValue(instanceGroup.Name)
	state.InstanceTemplateID = types.StringValue(instanceGroup.TemplateId)
	state.RunningInstanceCount = types.Int64Value(instanceGroup.RunningInstanceCount)
	state.State = types.StringValue(instanceGroup.State)
	state.DesiredCount = types.Int64Value(instanceGroup.DesiredCount)
	state.CreatedAt = types.StringValue(instanceGroup.CreatedAt)
	state.UpdatedAt = types.StringValue(instanceGroup.UpdatedAt)

	// Sort instance ID lists for deterministic ordering; the API does not guarantee a stable order.
	slices.Sort(instanceGroup.ActiveInstances)
	slices.Sort(instanceGroup.InactiveInstances)

	var tfListDiags diag.Diagnostics
	state.ActiveInstanceIDs, tfListDiags = common.StringSliceToTFList(instanceGroup.ActiveInstances)
	diags.Append(tfListDiags...)
	state.InactiveInstanceIDs, tfListDiags = common.StringSliceToTFList(instanceGroup.InactiveInstances)
	diags.Append(tfListDiags...)
}

func instanceGroupToDataSourceModel(item *swagger.InstanceGroup) instanceGroupDataSourceModel {
	// Sort instance ID lists for deterministic ordering; the API does not guarantee a stable order.
	slices.Sort(item.ActiveInstances)
	slices.Sort(item.InactiveInstances)

	return instanceGroupDataSourceModel{
		ID:                   item.Id,
		ProjectID:            item.ProjectId,
		Name:                 item.Name,
		InstanceTemplateID:   item.TemplateId,
		RunningInstanceCount: item.RunningInstanceCount,
		State:                item.State,
		DesiredCount:         item.DesiredCount,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
		ActiveInstanceIDs:    item.ActiveInstances,
		InactiveInstanceIDs:  item.InactiveInstances,
	}
}

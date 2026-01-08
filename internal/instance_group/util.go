package instance_group

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

var errGetResourceModel = errors.New("unable to get resource model")

// Shared schema descriptions for resource and data source
const (
	descID                   = "The unique identifier of the instance group."
	descName                 = "The name of the instance group."
	descProjectID            = "The ID of the project this instance group belongs to."
	descProjectIDInference   = "If not specified, the project ID will be inferred from the Crusoe configuration."
	descInstanceTemplateID   = "The ID of the instance template used for creating instances in this group."
	descRunningInstanceCount = "The number of running instances currently in the instance group."
	descDesiredCount         = "The desired number of VMs for the instance group."
	descState                = "The current state of the instance group. Possible values: `HEALTHY` (matches desired count), `UPDATING` (scaling in progress), `UNHEALTHY` (cannot reach desired count)."
	descActiveInstanceIDs    = "A list of IDs of running instances in the instance group."
	descInactiveInstanceIDs  = "A list of IDs of non-running instances in the instance group."
	descCreatedAt            = "The timestamp when the instance group was created."
	descUpdatedAt            = "The timestamp when the instance group was most recently updated."
)

// tfDataGetter is implemented by tfsdk.State and tfsdk.Plan
type tfDataGetter interface {
	Get(ctx context.Context, target interface{}) diag.Diagnostics
}

// getResourceModel extracts the resource model from state or plan.
// Returns errGetResourceModel if there were errors (diagnostics already appended to respDiags).
func getResourceModel(ctx context.Context, source tfDataGetter, dest *instanceGroupResourceModel, respDiags *diag.Diagnostics) error {
	diags := source.Get(ctx, dest)
	respDiags.Append(diags...)

	if respDiags.HasError() {
		return errGetResourceModel
	}

	return nil
}

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

	var tfListDiags diag.Diagnostics
	state.ActiveInstanceIDs, tfListDiags = common.StringSliceToTFList(instanceGroup.ActiveInstances)
	diags.Append(tfListDiags...)
	state.InactiveInstanceIDs, tfListDiags = common.StringSliceToTFList(instanceGroup.InactiveInstances)
	diags.Append(tfListDiags...)
}

func instanceGroupToDataSourceModel(item *swagger.InstanceGroup) instanceGroupDataSourceModel {
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

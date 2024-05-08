package instance_group

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

func findInstanceGroup(ctx context.Context, client *swagger.APIClient, instanceGroupID string) (*swagger.InstanceGroup, string, error) {
	args := common.FindResourceArgs[swagger.InstanceGroup]{
		ResourceID:  instanceGroupID,
		GetResource: client.InstanceGroupsApi.GetInstanceGroup,
		IsResource: func(group swagger.InstanceGroup, id string) bool {
			return group.Id == id
		},
	}

	return common.FindResource[swagger.InstanceGroup](ctx, client, args)
}

func instanceGroupToTerraformResourceModel(instanceGroup *swagger.InstanceGroup, state *instanceGroupResourceModel) {
	state.ID = types.StringValue(instanceGroup.Id)
	state.Name = types.StringValue(instanceGroup.Name)
	state.TemplateID = types.StringValue(instanceGroup.TemplateId)
	state.RunningInstanceCount = types.Int64Value(instanceGroup.RunningInstanceCount)
	state.ProjectID = types.StringValue(instanceGroup.ProjectId)

	instancesInGroup, _ := types.ListValueFrom(context.Background(), types.StringType, instanceGroup.Instances)
	state.Instances = instancesInGroup
}

func addInstancesToGroup(ctx context.Context, client *swagger.APIClient,
	namePrefix, groupID, templateID, projectID string,
	numInstances int64,
) error {
	bulkResp, bulkHttpResp, bulkErr := client.VMsApi.BulkCreateInstance(ctx, swagger.BulkInstancePostRequestV1Alpha5{
		NamePrefix:         namePrefix,
		Count:              numInstances,
		InstanceTemplateId: templateID,
		InstanceGroupId:    groupID,
	}, projectID)
	if bulkErr != nil {
		return common.UnpackAPIError(bulkErr)
	}
	defer bulkHttpResp.Body.Close()

	instances, _, waitErr := common.AwaitOperationAndResolve[[]swagger.InstanceV1Alpha5](
		ctx, bulkResp.Operation, projectID, client.VMOperationsApi.GetComputeVMsInstancesOperation)
	if waitErr != nil {
		return common.UnpackAPIError(waitErr)
	}
	instancesList := *instances
	if len(instancesList) < 1 {
		return errors.New("failed to create instance: no instance was created")
	}

	return nil
}

func removeInstancesFromGroup(ctx context.Context, client *swagger.APIClient,
	projectID string, numInstances int64, currInstances []string,
) ([]string, error) {
	numInstancesToRemove := int64(len(currInstances)) - numInstances
	remainingInstances := make([]string, 0, numInstances)
	for i := int64(0); i < numInstancesToRemove; i++ {
		if i < numInstancesToRemove {
			instanceToDelete := currInstances[i]
			delDataResp, delHttpResp, err := client.VMsApi.DeleteInstance(ctx, projectID, instanceToDelete)
			delHttpResp.Body.Close()
			if err != nil {
				return nil, common.UnpackAPIError(err)
			}

			_, _, err = common.AwaitOperationAndResolve[interface{}](ctx, delDataResp.Operation, projectID,
				client.VMOperationsApi.GetComputeVMsInstancesOperation)
			if err != nil {
				return nil, common.UnpackAPIError(err)
			}
		} else {
			remainingInstances = append(remainingInstances, currInstances[i])
		}
	}

	return remainingInstances, nil
}

package kubernetes_node_pool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

var (
	ErrFailedDemarshal = errors.New("failed to demarshal node pool operation into either node pool or node pool response")
	ErrNodePoolBothNil = errors.New("neither node pool nor node pool response found")
)

func ParseOpResultStrict[T any](opResult interface{}) (*T, error) {
	b, err := json.Marshal(opResult)
	if err != nil {
		return nil, common.ErrUnableToGetOpRes
	}

	var result T
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()

	err = json.Unmarshal(b, &result)
	if err != nil {
		return nil, common.ErrUnableToGetOpRes
	}

	return &result, nil
}

func AwaitNodePoolOrNodePoolResponse(ctx context.Context, asyncOperation *swagger.Operation, projectID string, client *swagger.APIClient) (*swagger.KubernetesNodePool, *swagger.KubernetesNodePoolResponse, error) {
	var err error
	var secondErr error
	var finalOp *swagger.Operation
	var nodePool *swagger.KubernetesNodePool
	var nodePoolResponse *swagger.KubernetesNodePoolResponse

	finalOp, err = common.AwaitOperation(ctx, asyncOperation, projectID, client.KubernetesNodePoolOperationsApi.GetKubernetesNodePoolsOperation)
	if err != nil {
		return nil, nil, err
	}

	// Try new node pool response
	nodePoolResponse, err = ParseOpResultStrict[swagger.KubernetesNodePoolResponse](finalOp.Result)
	if err != nil || nodePoolResponse.NodePool == nil {
		// Handle old node pool response
		nodePool, secondErr = ParseOpResultStrict[swagger.KubernetesNodePool](finalOp.Result)
		if secondErr != nil {
			// Both demarshal attempts failed
			return nil, nil, fmt.Errorf("%w: %w and %w", ErrFailedDemarshal, err, secondErr)
		}
	}

	return nodePool, nodePoolResponse, err
}

func AwaitNodePoolOperation(ctx context.Context, asyncOperation *swagger.Operation, projectID string, client *swagger.APIClient) (*swagger.KubernetesNodePoolResponse, error) {
	nodePool, nodePoolResponse, err := AwaitNodePoolOrNodePoolResponse(ctx, asyncOperation, projectID, client)
	if err != nil {
		return nil, err
	}

	// Handle new node pool response
	if nodePoolResponse != nil && nodePoolResponse.NodePool != nil {
		return nodePoolResponse, err
	}

	// Try old node pool response
	if nodePool != nil {
		return &swagger.KubernetesNodePoolResponse{
			NodePool: nodePool,
			Details: &swagger.OperationDetails{
				Error_:        "",
				NumVmsCreated: int32(nodePool.Count),
			},
		}, err
	}

	return nil, ErrNodePoolBothNil
}

// nodePoolNeedsRotation checks if plan and state differences require rotation
func nodePoolNeedsRotation(plan, state *kubernetesNodePoolResourceModel) bool {
	if !plan.Version.Equal(state.Version) {
		return true
	}
	if !plan.RequestedNodeLabels.Equal(state.RequestedNodeLabels) {
		return true
	}
	if !plan.EphemeralStorageForContainerd.Equal(state.EphemeralStorageForContainerd) {
		return true
	}

	return false
}

func UpdateNodePoolNeedsRotation(ctx context.Context, req *resource.UpdateRequest, resp *resource.UpdateResponse) bool {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return false
	}

	var plan, state kubernetesNodePoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return false
	}

	return nodePoolNeedsRotation(&plan, &state)
}

func ModifyPlanNodePoolNeedsRotation(ctx context.Context, req *resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) bool {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return false
	}

	var plan, state kubernetesNodePoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return false
	}

	return nodePoolNeedsRotation(&plan, &state)
}

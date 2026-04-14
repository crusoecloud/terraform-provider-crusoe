package kubernetes_node_pool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

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

// nodePoolNeedsRollout checks if plan and state differences require rollout of changes
func nodePoolNeedsRollout(plan, state *kubernetesNodePoolResourceModel) bool {
	return !plan.Version.Equal(state.Version) ||
		!plan.RequestedNodeLabels.Equal(state.RequestedNodeLabels) ||
		!plan.NodeTaints.Equal(state.NodeTaints) ||
		!plan.EphemeralStorageForContainerd.Equal(state.EphemeralStorageForContainerd) ||
		!plan.BatchPercentage.Equal(state.BatchPercentage) ||
		!plan.BatchSize.Equal(state.BatchSize)
}

func UpdateNodePoolNeedsRollout(ctx context.Context, req *resource.UpdateRequest, resp *resource.UpdateResponse) bool {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return false
	}

	var plan, state kubernetesNodePoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return false
	}

	return nodePoolNeedsRollout(&plan, &state)
}

func ModifyPlanNodePoolNeedsRollout(ctx context.Context, req *resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) bool {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return false
	}

	var plan, state kubernetesNodePoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return false
	}

	return nodePoolNeedsRollout(&plan, &state)
}

type nodeTaintModel struct {
	Key    types.String `tfsdk:"key"`
	Value  types.String `tfsdk:"value"`
	Effect types.String `tfsdk:"effect"`
}

// tfListToNodeTaints converts a Terraform List to swagger node taints.
func tfListToNodeTaints(ctx context.Context, tfList types.List) ([]swagger.KubernetesNodeTaint, error) {
	if tfList.IsNull() || tfList.IsUnknown() {
		return nil, nil
	}
	var models []nodeTaintModel
	diags := tfList.ElementsAs(ctx, &models, false)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to parse node taints")
	}
	taints := make([]swagger.KubernetesNodeTaint, 0, len(models))
	for _, m := range models {
		taints = append(taints, swagger.KubernetesNodeTaint{
			Key:    m.Key.ValueString(),
			Value:  m.Value.ValueString(),
			Effect: m.Effect.ValueString(),
		})
	}

	return taints, nil
}

// nodeTaintsToTFList converts swagger node taints to a Terraform List
func nodeTaintsToTFList(ctx context.Context, taints []swagger.KubernetesNodeTaint) (types.List, diag.Diagnostics) {
	if len(taints) == 0 {
		return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: nodeTaintAttrTypes()}, []nodeTaintModel{})
	}
	models := make([]nodeTaintModel, 0, len(taints))
	for _, t := range taints {
		models = append(models, nodeTaintModel{
			Key:    types.StringValue(t.Key),
			Value:  types.StringValue(t.Value),
			Effect: types.StringValue(t.Effect),
		})
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: nodeTaintAttrTypes()}, models)
}

func nodeTaintAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"key":    types.StringType,
		"value":  types.StringType,
		"effect": types.StringType,
	}
}

func validateNodeTaintDuplicates(taints []swagger.KubernetesNodeTaint) error {
	seen := make(map[string]struct{})
	for _, t := range taints {
		uniqueKey := t.Key + ":" + t.Effect
		if _, exists := seen[uniqueKey]; exists {
			return fmt.Errorf("duplicate taint: key %q with effect %q is specified more than once", t.Key, t.Effect)
		}
		seen[uniqueKey] = struct{}{}
	}

	return nil
}

package kubernetes_node_pool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

var (
	ErrFailedDemarshal = errors.New("failed to demarshal node pool operation into either node pool or node pool response")
	ErrNodePoolBothNil = errors.New("neither node pool nor node pool response found")
)

// apiDesc* — schema descriptions derived from the client-go swagger spec
// (KubernetesNodePool / KubernetesNodeTaint definitions; version,
// requested_node_labels, and ssh_key from KubernetesNodePoolPostRequest).
const (
	apiDescID                            = "ID of the node pool."
	apiDescVersion                       = "Version of the Kubernetes node pool."
	apiDescImageID                       = "ID of the image used for the node pool."
	apiDescType                          = "VM type of the node pool."
	apiDescInstanceCount                 = "Number of nodes in the node pool."
	apiDescClusterID                     = "ID of the Kubernetes cluster the node pool belongs to."
	apiDescSubnetID                      = "ID of the subnet the node pool belongs to."
	apiDescRequestedNodeLabels           = "Labels to assign to nodes in the new node pool."
	apiDescNodeLabels                    = "Labels assigned to nodes in the node pool."
	apiDescInstanceIDs                   = "IDs of the instances within the node pool."
	apiDescSSHKey                        = "SSH public key to use for all VMs created from the new node pool."
	apiDescState                         = "Current state of the node pool."
	apiDescName                          = "Name of the node pool."
	apiDescEphemeralStorageForContainerd = "Whether the first local ephemeral NVMe disk is used for containerd storage."
	apiDescNvlinkDomainID                = "NVLink domain ID assigned to the node pool."
	apiDescPublicIPType                  = "Public IP type for the node pool's nodes. Possible values: `dynamic`, `static`, `none`."
	apiDescNodeTaints                    = "Taints applied to nodes in the node pool."

	apiDescTaintKey    = "Taint key. Follows the Kubernetes qualified-name format: an optional DNS subdomain prefix (up to 253 characters) followed by a `/`, then a name segment (up to 63 characters). Allowed characters: alphanumerics, `-`, `_`, and `.`. Must start and end with an alphanumeric character. Keys beginning with `crusoe.ai/` are reserved for internal use."
	apiDescTaintValue  = "Taint value. May be empty. Follows the same format rules as a Kubernetes label value: up to 63 characters, alphanumerics and `-`, `_`, `.`."
	apiDescTaintEffect = "Taint effect, controlling how pods are treated on matching nodes. `NoSchedule`: new pods are not scheduled unless they tolerate. `PreferNoSchedule`: new pods avoid the node if possible. `NoExecute`: new pods are not scheduled and existing non-tolerating pods are evicted."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project that owns the node pool. " + project.ProviderDescProjectIDFallback

	providerDescBatchSize = common.DevelopmentMessage + " " +
		"Number of nodes to update at a time during rollout (minimum 1, maximum 10). " +
		"Mutually exclusive with batch_percentage. " +
		"If both this and batch_percentage are omitted, existing nodes will not be updated, " +
		"but new nodes will use the new configuration."
	providerDescBatchPercentage = common.DevelopmentMessage + " " +
		"Percentage of nodes to update concurrently during rollout. " +
		"The calculated number will not exceed 10 nodes. Mutually exclusive with batch_size. " +
		"If both this and batch_size are omitted, existing nodes will not be updated, " +
		"but new nodes will use the new configuration."
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

// tfSetToNodeTaints converts a Terraform Set to swagger node taints.
func tfSetToNodeTaints(ctx context.Context, tfSet types.Set) ([]swagger.KubernetesNodeTaint, error) {
	if tfSet.IsNull() || tfSet.IsUnknown() {
		return nil, nil
	}
	var models []nodeTaintModel
	diags := tfSet.ElementsAs(ctx, &models, false)
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

// nodeTaintsToTFSet converts swagger node taints to a Terraform Set.
// Returns an empty set (not null) when the server has no taints, matching
// how Terraform represents an absent SetNestedBlock.
func nodeTaintsToTFSet(ctx context.Context, taints []swagger.KubernetesNodeTaint) (types.Set, diag.Diagnostics) {
	if len(taints) == 0 {
		return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: nodeTaintAttrTypes()}, []nodeTaintModel{})
	}
	models := make([]nodeTaintModel, 0, len(taints))
	for _, t := range taints {
		models = append(models, nodeTaintModel{
			Key:    types.StringValue(t.Key),
			Value:  types.StringValue(t.Value),
			Effect: types.StringValue(t.Effect),
		})
	}

	return types.SetValueFrom(ctx, types.ObjectType{AttrTypes: nodeTaintAttrTypes()}, models)
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
	var dups []string
	for _, t := range taints {
		uniqueKey := t.Key + ":" + t.Effect
		if _, exists := seen[uniqueKey]; exists {
			dups = append(dups, fmt.Sprintf("key %q with effect %q", t.Key, t.Effect))
		} else {
			seen[uniqueKey] = struct{}{}
		}
	}
	switch len(dups) {
	case 0:
		return nil
	case 1:
		return fmt.Errorf("duplicate taint: %s is specified more than once", dups[0])
	default:
		return fmt.Errorf("duplicate taints:\n  - %s", strings.Join(dups, "\n  - "))
	}
}

// nodePoolToResourceModel maps an API node pool onto model, following the CCX-4492
// convention that the API object is the source of truth. It is called from Create,
// Read, and Update so the three previously-duplicated mappings stay in sync — in
// particular the instance_ids sort (CCX-4394) and the nvlink_domain_id
// empty-to-null normalization now live in exactly one place.
//
// The Terraform-only fields the API does not return (ib_partition_id, ssh_key,
// requested_node_labels, batch_size, batch_percentage) are taken from ref: the
// plan in Create/Update, the prior state in Read.
func nodePoolToResourceModel(ctx context.Context, nodePool *swagger.KubernetesNodePool,
	ref, model *kubernetesNodePoolResourceModel, diags *diag.Diagnostics,
) {
	model.ID = types.StringValue(nodePool.Id)
	model.ProjectID = types.StringValue(nodePool.ProjectId)
	model.InstanceCount = types.Int64Value(nodePool.Count)
	model.Version = types.StringValue(nodePool.ImageId)
	model.Type = types.StringValue(nodePool.Type_)
	model.ClusterID = types.StringValue(nodePool.ClusterId)
	model.SubnetID = types.StringValue(nodePool.SubnetId)
	model.State = types.StringValue(nodePool.State)
	model.Name = types.StringValue(nodePool.Name)
	model.EphemeralStorageForContainerd = types.BoolValue(nodePool.EphemeralStorageForContainerd)
	model.PublicIPType = types.StringValue(nodePool.PublicIpType)
	model.NvlinkDomainID = stringOrNull(nodePool.NvlinkDomainId)

	allNodeLabels, d := common.StringMapToTFMap(nodePool.NodeLabels)
	diags.Append(d...)
	model.AllNodeLabels = allNodeLabels

	nodeTaints, d := nodeTaintsToTFSet(ctx, nodePool.NodeTaints)
	diags.Append(d...)
	model.NodeTaints = nodeTaints

	instanceIDs, d := common.StringSliceToTFList(sortedInstanceIDs(nodePool.InstanceIds))
	diags.Append(d...)
	model.InstanceIDs = instanceIDs

	// Terraform-only fields (not returned by the API) come from the reference model.
	model.IBPartitionID = ref.IBPartitionID
	model.SSHKey = ref.SSHKey
	model.BatchSize = ref.BatchSize
	model.BatchPercentage = ref.BatchPercentage
	if ref.RequestedNodeLabels.IsUnknown() {
		model.RequestedNodeLabels = types.MapNull(types.StringType)
	} else {
		model.RequestedNodeLabels = ref.RequestedNodeLabels
	}
}

// stringOrNull maps an empty API string to a null value, matching how the nullable
// nvlink_domain_id attribute is represented in Terraform state.
func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}

	return types.StringValue(s)
}

// sortedInstanceIDs returns the instance IDs in a deterministic (lexical) order.
// instance_ids is a Computed, API-ordered list of opaque IDs and the API does not
// guarantee a stable order, so sorting prevents CCX-4394-class spurious diffs. The
// input slice is not mutated.
func sortedInstanceIDs(instanceIDs []string) []string {
	sorted := append([]string(nil), instanceIDs...)
	slices.Sort(sorted)

	return sorted
}

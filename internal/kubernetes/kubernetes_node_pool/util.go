package kubernetes_node_pool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

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
// requested_node_labels, ssh_key, and transport_partition_id from
// KubernetesNodePoolPostRequest).
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
	apiDescTransportPartitionID          = "ID of the Infiniband or RoCE partition to create node pool in. Must be in the location of the cluster if specified."
	apiDescCurrent                       = "Number of the pool's nodes that have joined the cluster and passed readiness: registered with the API server and ready to take workloads."
	apiDescUpdateSettings                = "Settings controlling how update operations may act on the node pool's existing nodes."
	apiDescAllowScaleDown                = "Whether an update may scale the node pool below its current node count, draining and deleting existing nodes. When false (the default), an update that lowers the count only records the new target and reports a health issue; no nodes are removed. Only supported on CMK v2 clusters."
	apiDescHealth                        = "Issues currently detected on the node pool."
	apiDescHealthIssues                  = "Current issues detected on the node pool."
	apiDescIssueCode                     = "Machine-readable code for the issue, e.g. `INSUFFICIENT_CAPACITY`, `INSUFFICIENT_QUOTA`, `NODE_NOT_READY` or `INTERNAL_ERROR`. New codes may be added; treat unknown values as display-only. A code persists until the node deficit behind it is resolved."
	apiDescIssueMessage                  = "Human-readable description of the issue."
	apiDescIssueSince                    = "Time the issue started, in RFC3339 format."
	apiDescAffectedCount                 = "Number of nodes affected by the issue."
	apiDescAffectedNodeIDs               = "IDs of the affected nodes, when known. Node IDs are the IDs of the VMs backing the nodes."

	apiDescConsentMode = "Remediation consent posture for the node pool. Possible values: `auto`, `propose`, `off`."

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

	// providerDescV2Only marks the fields only a node pool served by the v2
	// backend carries. Setting one on a pool served by v1 is refused by the API,
	// and reads omit them, so they are null rather than defaulted.
	providerDescV2Only = "Only available for node pools on CMK v2 clusters; null for others."

	providerDescConsentModeDefault = "Newly created node pools default to `propose`. " +
		"A node pool migrated from CMK v1 starts at `off`, so that remediation does not begin on a " +
		"pool whose owner was never asked."
)

// providerDescIBPartitionIDDeprecated marks ib_partition_id as replaced by
// transport_partition_id. The spec does not describe ib_partition_id, so this
// text is provider-side only.
var providerDescIBPartitionIDDeprecated = common.FormatDeprecationWithReplacement("v1.3.0", "transport_partition_id")

// The two names of the partition alias pair. Both the schema and ValidateConfig refer to
// these, so the pair is described in one place.
const (
	attrIBPartitionID        = "ib_partition_id"
	attrTransportPartitionID = "transport_partition_id"
)

const aliasPairReplaceDescription = "Recreates the node pool when the partition changes. " +
	"Renaming ib_partition_id to transport_partition_id is not a change."

// aliasPairReplaceIfModifier builds the plan modifier both halves of the partition alias
// pair share.
func aliasPairReplaceIfModifier() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(
		common.AliasPairRequiresReplaceIf(attrIBPartitionID, attrTransportPartitionID),
		aliasPairReplaceDescription,
		aliasPairReplaceDescription,
	)
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
			attrIBPartitionID, deprecated.ValueString(),
			attrTransportPartitionID, replacement.ValueString(),
			attrIBPartitionID, attrTransportPartitionID),
	)
}

// parseNodePoolOpResult decodes an operation result document into T, ignoring
// fields T does not model.
//
// The leniency is deliberate and load-bearing, which is why it is stated here:
// the API adds response fields over the life of a released provider, and a
// provider that refused a document carrying one would break against a newer API
// until its pinned client-go caught up — backwards for a client, whose job is to
// keep working. A node pool served by the v2 backend already carries fields
// (health, current, update_settings, consent_mode) that older client-go versions
// do not model.
//
// This function used to construct a json.Decoder and call DisallowUnknownFields
// on it, then decode with json.Unmarshal instead — so the strictness never took
// effect and the name claimed the opposite of the behavior. Making it real would
// have broken every create and update against a v2-backed pool.
//
// AwaitNodePoolOrNodePoolResponse's fallback does not depend on strictness: it
// discriminates on NodePool being nil, which is what a bare KubernetesNodePool
// document (no "node_pool" key) decodes to here.
func parseNodePoolOpResult[T any](opResult interface{}) (*T, error) {
	b, err := json.Marshal(opResult)
	if err != nil {
		return nil, common.ErrUnableToGetOpRes
	}

	var result T
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, common.ErrUnableToGetOpRes
	}

	return &result, nil
}

// nodePoolOpResult and operationDetails mirror swagger.KubernetesNodePoolResponse and
// swagger.OperationDetails, which the generated client no longer exposes.
// TODO: CCX-5707 - drop and revert to the swagger types once the SDK exposes them again.
type operationDetails struct {
	Error_        string `json:"error,omitempty"`
	NumVmsCreated int32  `json:"num_vms_created,omitempty"`
}

type nodePoolOpResult struct {
	Details  *operationDetails           `json:"details,omitempty"`
	NodePool *swagger.KubernetesNodePool `json:"node_pool"`
}

func AwaitNodePoolOrNodePoolResponse(ctx context.Context, asyncOperation *swagger.Operation, projectID string, client *swagger.APIClient) (*swagger.KubernetesNodePool, *nodePoolOpResult, error) {
	var err error
	var secondErr error
	var finalOp *swagger.Operation
	var nodePool *swagger.KubernetesNodePool
	var nodePoolResponse *nodePoolOpResult

	finalOp, err = common.AwaitOperation(ctx, asyncOperation, projectID, client.KubernetesNodePoolOperationsApi.GetKubernetesNodePoolsOperation)
	if err != nil {
		return nil, nil, err
	}

	// Try new node pool response
	nodePoolResponse, err = parseNodePoolOpResult[nodePoolOpResult](finalOp.Result)
	if err != nil || nodePoolResponse.NodePool == nil {
		// Handle old node pool response
		nodePool, secondErr = parseNodePoolOpResult[swagger.KubernetesNodePool](finalOp.Result)
		if secondErr != nil {
			// Both demarshal attempts failed
			return nil, nil, fmt.Errorf("%w: %w and %w", ErrFailedDemarshal, err, secondErr)
		}
	}

	return nodePool, nodePoolResponse, err
}

func AwaitNodePoolOperation(ctx context.Context, asyncOperation *swagger.Operation, projectID string, client *swagger.APIClient) (*nodePoolOpResult, error) {
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
		return &nodePoolOpResult{
			NodePool: nodePool,
			Details: &operationDetails{
				Error_:        "",
				NumVmsCreated: int32(nodePool.Count),
			},
		}, err
	}

	return nil, ErrNodePoolBothNil
}

// nodePoolNeedsRollout checks if plan and state differences require rollout of changes
func nodePoolNeedsRollout(plan, state *kubernetesNodePoolResourceModel) bool {
	return !common.SameK8sVersion(plan.Version.ValueString(), state.Version.ValueString()) ||
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
// The Terraform-only fields the API does not return (ib_partition_id,
// transport_partition_id, ssh_key, requested_node_labels, batch_size,
// batch_percentage) are taken from ref: the
// plan in Create/Update, the prior state in Read.
func nodePoolToResourceModel(ctx context.Context, nodePool *swagger.KubernetesNodePool,
	ref, model *kubernetesNodePoolResourceModel, diags *diag.Diagnostics,
) {
	model.ID = types.StringValue(nodePool.Id)
	model.ProjectID = types.StringValue(nodePool.ProjectId)
	model.InstanceCount = types.Int64Value(nodePool.Count)
	model.Version = common.NewK8sVersionValue(nodePool.ImageId)
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

	// The v2-only group. All four are absent on a node pool served by v1, and
	// absence is carried through rather than defaulted — see the API contract
	// notes on each helper.
	model.ConsentMode = stringOrNull(nodePool.ConsentMode)

	updateSettings, d := updateSettingsToTFObject(nodePool.UpdateSettings)
	diags.Append(d...)
	model.UpdateSettings = updateSettings

	health, d := healthToTFObject(ctx, nodePool.Health)
	diags.Append(d...)
	model.Health = health

	model.Current = currentOrNull(nodePool)

	// Terraform-only fields (not returned by the API) come from the reference model.
	model.IBPartitionID = ref.IBPartitionID
	model.TransportPartitionID = ref.TransportPartitionID
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

// Attribute names for the two nested blocks, referenced by both schemas and by
// the conversions below.
const (
	attrAllowScaleDown  = "allow_scale_down"
	attrHealthIssues    = "issues"
	attrIssueCode       = "code"
	attrIssueMessage    = "message"
	attrIssueSince      = "since"
	attrAffectedCount   = "affected_count"
	attrAffectedNodeIDs = "affected_node_ids"
)

// Consent modes the API accepts on a write. detect_only is deliberately absent:
// it is a real mode the backend rejects as "not yet available", so accepting it
// here would only let a configuration fail on every apply.
var consentModeValues = []string{"auto", "propose", "off"}

func updateSettingsAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		attrAllowScaleDown: types.BoolType,
	}
}

func healthIssueAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		attrIssueCode:       types.StringType,
		attrIssueMessage:    types.StringType,
		attrIssueSince:      types.StringType,
		attrAffectedCount:   types.Int64Type,
		attrAffectedNodeIDs: types.ListType{ElemType: types.StringType},
	}
}

func healthAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		attrHealthIssues: types.ListType{ElemType: types.ObjectType{AttrTypes: healthIssueAttrTypes()}},
	}
}

// updateSettingsToTFObject renders the pool's update settings, preserving
// absence: only a backend that carries them serves the block, and a null object
// says "this pool has no update settings" rather than "scale-down is off".
func updateSettingsToTFObject(settings *swagger.KubernetesNodePoolUpdateSettings) (types.Object, diag.Diagnostics) {
	if settings == nil {
		return types.ObjectNull(updateSettingsAttrTypes()), nil
	}

	return types.ObjectValue(updateSettingsAttrTypes(), map[string]attr.Value{
		attrAllowScaleDown: types.BoolValue(settings.AllowScaleDown),
	})
}

// tfObjectToUpdateSettings is the request direction. A null or unknown object
// yields nil, and nil is meaningful: the API leaves the stored settings alone
// when the block is absent and treats a present block as an assertion, so
// presence must ride on the pointer rather than on the bool inside it. The
// bool's own omitempty means a present block carrying false marshals as {},
// which the API still reads as a present block.
func tfObjectToUpdateSettings(ctx context.Context, obj types.Object) (
	*swagger.KubernetesNodePoolUpdateSettings, diag.Diagnostics,
) {
	if obj.IsNull() || obj.IsUnknown() {
		return nil, nil
	}

	var model updateSettingsModel
	diags := obj.As(ctx, &model, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return &swagger.KubernetesNodePoolUpdateSettings{
		AllowScaleDown: model.AllowScaleDown.ValueBool(),
	}, diags
}

type updateSettingsModel struct {
	AllowScaleDown types.Bool `tfsdk:"allow_scale_down"`
}

// healthToTFObject renders the derived health block.
//
// Absence is preserved for the same reason the API preserves it: a pool with
// nothing to report carries no block at all, so "healthy" and "not reported"
// look the same on the wire — and inventing an empty issues list here would
// claim the former on a backend that only meant the latter.
//
// Issues are sorted by code, and each issue's node IDs lexically: both are
// API-ordered collections with no guaranteed order, and an unsorted Computed
// list re-orders between reads and shows up as a spurious diff (CCX-4394).
func healthToTFObject(ctx context.Context, health *swagger.KubernetesNodePoolHealth) (
	types.Object, diag.Diagnostics,
) {
	var diags diag.Diagnostics

	if health == nil {
		return types.ObjectNull(healthAttrTypes()), diags
	}

	issues := append([]swagger.KubernetesNodePoolHealthIssue(nil), health.Issues...)
	slices.SortFunc(issues, func(a, b swagger.KubernetesNodePoolHealthIssue) int {
		return strings.Compare(a.Code, b.Code)
	})

	issueValues := make([]attr.Value, 0, len(issues))
	for _, issue := range issues {
		nodeIDs, d := common.StringSliceToTFList(sortedInstanceIDs(issue.AffectedNodeIds))
		diags.Append(d...)

		issueValue, d := types.ObjectValue(healthIssueAttrTypes(), map[string]attr.Value{
			attrIssueCode:       types.StringValue(issue.Code),
			attrIssueMessage:    types.StringValue(issue.Message),
			attrIssueSince:      stringOrNull(issue.Since),
			attrAffectedCount:   types.Int64Value(issue.AffectedCount),
			attrAffectedNodeIDs: nodeIDs,
		})
		diags.Append(d...)
		issueValues = append(issueValues, issueValue)
	}

	issueList, d := types.ListValue(types.ObjectType{AttrTypes: healthIssueAttrTypes()}, issueValues)
	diags.Append(d...)

	healthObject, d := types.ObjectValue(healthAttrTypes(), map[string]attr.Value{
		attrHealthIssues: issueList,
	})
	diags.Append(d...)

	return healthObject, diags
}

// currentOrNull renders the pool's ready node count, preserving the absence the
// API contract depends on: the spec says an omitted `current` means the serving
// backend does not compute it, while zero "means the pool genuinely has no ready
// nodes". The generated client models the field as a plain int64, so decoding
// collapses those two into 0 and the distinction has to be recovered here.
//
// update_settings is the discriminator, because it is modelled as a pointer and
// so survives decoding: a backend that derives read-time fields serves it
// always, and one that does not never serves it. The narrow claim this rests on
// is not "the pool is v2" but "this response carried derived read fields at
// all" — if a backend ever computes `current` without also serving
// update_settings, this would null a real count, and the fix then is to have the
// spec model `current` as nullable rather than to widen the guess.
func currentOrNull(nodePool *swagger.KubernetesNodePool) types.Int64 {
	if nodePool.UpdateSettings == nil {
		return types.Int64Null()
	}

	return types.Int64Value(nodePool.Current)
}

// planAllowsScaleDown reports whether the planned update_settings turn
// scale-down on. An absent, unknown, or unreadable block is "off", matching the
// API's own default — a plan-time warning must not claim nodes will be deleted
// on a guess.
func planAllowsScaleDown(ctx context.Context, updateSettings types.Object) bool {
	settings, diags := tfObjectToUpdateSettings(ctx, updateSettings)
	if diags.HasError() || settings == nil {
		return false
	}

	return settings.AllowScaleDown
}

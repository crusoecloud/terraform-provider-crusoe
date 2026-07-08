package kubernetes_node_pool

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                   = &kubernetesNodePoolResource{}
	_ resource.ResourceWithValidateConfig = &kubernetesNodePoolResource{}
)

type kubernetesNodePoolResource struct {
	client *common.CrusoeClient
}

func NewKubernetesNodePoolResource() resource.Resource {
	return &kubernetesNodePoolResource{}
}

type kubernetesNodePoolResourceModel struct {
	ID                            types.String `tfsdk:"id"`
	ProjectID                     types.String `tfsdk:"project_id"`
	Version                       types.String `tfsdk:"version"`
	Type                          types.String `tfsdk:"type"`
	InstanceCount                 types.Int64  `tfsdk:"instance_count"`
	ClusterID                     types.String `tfsdk:"cluster_id"`
	SubnetID                      types.String `tfsdk:"subnet_id"`
	IBPartitionID                 types.String `tfsdk:"ib_partition_id"`
	RequestedNodeLabels           types.Map    `tfsdk:"requested_node_labels"`
	AllNodeLabels                 types.Map    `tfsdk:"all_node_labels"`
	NodeTaints                    types.Set    `tfsdk:"node_taints"`
	InstanceIDs                   types.List   `tfsdk:"instance_ids"`
	SSHKey                        types.String `tfsdk:"ssh_key"`
	State                         types.String `tfsdk:"state"`
	Name                          types.String `tfsdk:"name"`
	EphemeralStorageForContainerd types.Bool   `tfsdk:"ephemeral_storage_for_containerd"`
	BatchSize                     types.Int64  `tfsdk:"batch_size"`
	BatchPercentage               types.Int64  `tfsdk:"batch_percentage"`
	NvlinkDomainID                types.String `tfsdk:"nvlink_domain_id"`
	PublicIPType                  types.String `tfsdk:"public_ip_type"`
}

func (r *kubernetesNodePoolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*common.CrusoeClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", common.ErrorMsgProviderInitFailed)

		return
	}

	r.client = client
}

func (r *kubernetesNodePoolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_node_pool"
}

// ValidateConfig runs at plan time and surfaces config-only violations
// (no remote state needed) before any API call. Schema validators handle
// per-field shape; this catches list-level rules like duplicate taints.
func (r *kubernetesNodePoolResource) ValidateConfig(
	ctx context.Context,
	req resource.ValidateConfigRequest,
	resp *resource.ValidateConfigResponse,
) {
	var data kubernetesNodePoolResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	nodeTaints, err := tfSetToNodeTaints(ctx, data.NodeTaints)
	if err != nil {
		// Unresolved/unknown values; let plan/apply re-evaluate.
		return
	}
	if vErr := validateNodeTaintDuplicates(nodeTaints); vErr != nil {
		resp.Diagnostics.AddError("Invalid node_taints", vErr.Error())
	}
}

func (r *kubernetesNodePoolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"project_id": schema.StringAttribute{
				Computed:      true,
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplaceIfConfigured()}, // cannot be updated in place by user
			},
			"version": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
				Validators: []validator.String{stringvalidator.RegexMatches(
					regexp.MustCompile(`\d+\.\d+\.\d+-cmk\.\d+.*`), "must be in the format MAJOR.MINOR.BUGFIX-cmk.NUM (e.g 1.2.3-cmk.4)",
				)},
			},
			"type": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"instance_count": schema.Int64Attribute{
				Required: true,
			},
			"cluster_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"subnet_id": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplaceIfConfigured()}, // cannot be updated in place by user
			},
			"ib_partition_id": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"requested_node_labels": schema.MapAttribute{
				ElementType:   types.StringType,
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"all_node_labels": schema.MapAttribute{
				ElementType: types.StringType,
				Computed:    true,
			},
			"instance_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
			},
			"ssh_key": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"state": schema.StringAttribute{
				Computed: true,
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"ephemeral_storage_for_containerd": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"batch_size": schema.Int64Attribute{
				Optional: true,
				MarkdownDescription: common.DevelopmentMessage + " " +
					"Number of nodes to update at a time during rollout (minimum 1, maximum 10). " +
					"Mutually exclusive with batch_percentage. " +
					"If both this and batch_percentage are omitted, existing nodes will not be updated, " +
					"but new nodes will use the new configuration.",
				Validators: []validator.Int64{
					int64validator.Between(1, 10),
				},
				PlanModifiers: []planmodifier.Int64{
					common.NewDevelopmentWarningInt64Modifier("", "Node pool rollout is currently in development. "+common.DevelopmentSupportMessage),
				},
			},
			"batch_percentage": schema.Int64Attribute{
				Optional: true,
				MarkdownDescription: common.DevelopmentMessage + " " +
					"Percentage of nodes to update concurrently during rollout. " +
					"The calculated number will not exceed 10 nodes. Mutually exclusive with batch_size. " +
					"If both this and batch_size are omitted, existing nodes will not be updated, " +
					"but new nodes will use the new configuration.",
				Validators: []validator.Int64{
					int64validator.Between(1, 100),
				},
				PlanModifiers: []planmodifier.Int64{
					common.NewDevelopmentWarningInt64Modifier("", "Node pool rollout is currently in development. "+common.DevelopmentSupportMessage),
				},
			},
			"nvlink_domain_id": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place & maintain across updates
			},
			"public_ip_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("dynamic"), // Default to dynamic
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					common.NewPrivateNodePoolsWarningModifier(),
				}, // maintain across updates
			},
		},
		Blocks: map[string]schema.Block{
			"node_taints": schema.SetNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Taint key. Follows the Kubernetes qualified-name format (optional DNS subdomain prefix, name segment up to 63 chars). Must be non-empty.",
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
						},
						"value": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString(""),
							MarkdownDescription: "Taint value. Defaults to empty string if omitted. Must be at most 63 characters.",
							Validators: []validator.String{
								stringvalidator.LengthAtMost(63),
							},
						},
						"effect": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Taint effect. Possible values: `NoSchedule`, `PreferNoSchedule`, `NoExecute`.",
							Validators: []validator.String{
								stringvalidator.OneOf("NoSchedule", "PreferNoSchedule", "NoExecute"),
							},
						},
					},
				},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesNodePoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesNodePoolResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())

	var nodeLabels map[string]string
	if !plan.RequestedNodeLabels.IsNull() && !plan.RequestedNodeLabels.IsUnknown() {
		var err error
		nodeLabels, err = common.TFMapToStringMap(plan.RequestedNodeLabels)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create node pool", fmt.Sprintf("error when parsing requested_node_labels as string map: %s", err))

			return
		}
	}

	nodeTaints, err := tfSetToNodeTaints(ctx, plan.NodeTaints)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create node pool", fmt.Sprintf("error when parsing node taints: %s", err))

		return
	}

	asyncOperation, _, err := r.client.APIClient.KubernetesNodePoolsApi.CreateNodePool(ctx, swagger.KubernetesNodePoolPostRequest{
		ClusterId:                     plan.ClusterID.ValueString(),
		Count:                         plan.InstanceCount.ValueInt64(),
		IbPartitionId:                 plan.IBPartitionID.ValueString(),
		Name:                          plan.Name.ValueString(),
		NodeLabels:                    nodeLabels,
		NodeTaints:                    nodeTaints,
		NodePoolVersion:               plan.Version.ValueString(),
		ProductName:                   plan.Type.ValueString(),
		SshPublicKey:                  plan.SSHKey.ValueString(),
		SubnetId:                      plan.SubnetID.ValueString(),
		EphemeralStorageForContainerd: plan.EphemeralStorageForContainerd.ValueBool(),
		NvlinkDomainId:                plan.NvlinkDomainID.ValueString(),
		PublicIpType:                  plan.PublicIPType.ValueString(),
	}, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create node pool",
			fmt.Sprintf("Error starting a create node pool operation: %s", common.UnpackAPIError(err)))

		return
	}

	// Wait for operation to complete
	kubernetesNodePoolResponse, err := AwaitNodePoolOperation(ctx, asyncOperation.Operation, projectID, r.client.APIClient)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create node pool",
			fmt.Sprintf("Error creating a node pool: %s", common.UnpackAPIError(err)))

		return
	}

	if kubernetesNodePoolResponse.Details != nil && kubernetesNodePoolResponse.Details.Error_ != "" {
		// TODO: Return created count once NumVmsCreated is populated
		resp.Diagnostics.AddWarning("Unable to create desired number of VMs",
			fmt.Sprintf("Warning -- Unable to create desired number of VMs due to the following error: %s",
				kubernetesNodePoolResponse.Details.Error_))
	}

	var state kubernetesNodePoolResourceModel
	nodePoolToResourceModel(ctx, kubernetesNodePoolResponse.NodePool, &plan, &state, &resp.Diagnostics)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesNodePoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var stored kubernetesNodePoolResourceModel

	diags := req.State.Get(ctx, &stored)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, stored.ProjectID.ValueString())

	kubernetesNodePool, httpResp, err := r.client.APIClient.KubernetesNodePoolsApi.GetNodePool(ctx, projectID, stored.ID.ValueString())
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to get Kubernetes Node Pool", fmt.Sprintf("Failed to get node pool: %s.",
			common.UnpackAPIError(err)))

		return
	}

	var state kubernetesNodePoolResourceModel
	nodePoolToResourceModel(ctx, &kubernetesNodePool, &stored, &state, &resp.Diagnostics)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

// Handle validation at the resource level to prevent duplicate errors/warnings
// nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesNodePoolResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Only check during updates (skip creates and deletes)
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var plan kubernetesNodePoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state kubernetesNodePoolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Warn if instance count is decreasing
	if !plan.InstanceCount.IsNull() && !state.InstanceCount.IsNull() &&
		plan.InstanceCount.ValueInt64() < state.InstanceCount.ValueInt64() {

		resp.Diagnostics.AddAttributeWarning(
			path.Root("instance_count"),
			"Node pool instance count decreased",
			"Decreasing node pool instance count will not delete node pool VMs. Manual deletion is required.",
		)
	}

	// Check for mutual exclusivity
	if !plan.BatchSize.IsNull() && !plan.BatchPercentage.IsNull() {
		resp.Diagnostics.AddError(
			"Conflicting Configuration",
			"batch_size and batch_percentage are mutually exclusive. "+
				"Please specify only one.",
		)

		return
	}

	needsRollout := ModifyPlanNodePoolNeedsRollout(ctx, &req, resp)

	if !needsRollout {
		return
	}

	// Check if both batch_size and batch_percentage are null
	if plan.BatchSize.IsNull() && plan.BatchPercentage.IsNull() {
		resp.Diagnostics.AddWarning(
			"Existing nodes will not be updated",
			"Existing nodes will not be updated to the new configuration, "+
				"but new nodes will use the new configuration.",
		)

		return
	}

	// Build warning message based on which parameter is set
	warningMsg := "Using batch_size or batch_percentage during updates will cause nodes to be deleted and recreated in batches. " +
		"This will result in temporary capacity reduction during the rollout process. "

	if !plan.BatchPercentage.IsNull() {
		warningMsg += "The calculated number of nodes based on the percentage cannot exceed 10 nodes per batch. "
	}

	resp.Diagnostics.AddWarning(
		"Batch update will delete and recreate nodes",
		warningMsg,
	)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesNodePoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan kubernetesNodePoolResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var stored kubernetesNodePoolResourceModel
	diags = req.State.Get(ctx, &stored)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, stored.ProjectID.ValueString())

	var nodeLabels map[string]string
	if !plan.RequestedNodeLabels.IsNull() && !plan.RequestedNodeLabels.IsUnknown() {
		var err error
		nodeLabels, err = common.TFMapToStringMap(plan.RequestedNodeLabels)
		if err != nil {
			resp.Diagnostics.AddError("Failed to update node pool", fmt.Sprintf("error when parsing requested_node_labels as string map: %s", err))

			return
		}
	}

	nodeTaints, err := tfSetToNodeTaints(ctx, plan.NodeTaints)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update node pool", fmt.Sprintf("error when parsing node_taints: %s", err))

		return
	}

	// If node_taints block is removed from config but state has taints, send empty list to clear all taints.
	if nodeTaints == nil && !stored.NodeTaints.IsNull() && len(stored.NodeTaints.Elements()) > 0 {
		nodeTaints = []swagger.KubernetesNodeTaint{}
	}

	patchRequest := swagger.KubernetesNodePoolPatchRequest{
		Count:                         plan.InstanceCount.ValueInt64(),
		NodeLabels:                    nodeLabels,
		NodeTaints:                    nodeTaints,
		EphemeralStorageForContainerd: plan.EphemeralStorageForContainerd.ValueBool(),
		NodePoolVersion:               plan.Version.ValueString(),
	}

	updateAsyncOperation, _, err := r.client.APIClient.KubernetesNodePoolsApi.UpdateNodePool(
		ctx,
		patchRequest,
		projectID,
		stored.ID.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update node pool",
			fmt.Sprintf("Error starting an update node pool operation: %s", common.UnpackAPIError(err)))

		return
	}

	// Wait for update operation to complete
	kubernetesNodePoolResponse, err := AwaitNodePoolOperation(ctx, updateAsyncOperation.Operation, projectID, r.client.APIClient)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update node pool",
			fmt.Sprintf("Error updating a node pool: %s", common.UnpackAPIError(err)))

		return
	}

	if kubernetesNodePoolResponse.Details != nil && kubernetesNodePoolResponse.Details.Error_ != "" {
		// TODO: Return created count once NumVmsCreated is populated
		resp.Diagnostics.AddWarning("Unable to provision all instances",
			fmt.Sprintf("Warning -- Unable to create desired number of VMs due to the following error: %s",
				kubernetesNodePoolResponse.Details.Error_))
	}

	// Detect if changes require node rollout
	needsRollout := UpdateNodePoolNeedsRollout(ctx, &req, resp)

	// run rollout if batch_size or batch_percentage is specified
	batchSizeSet := !plan.BatchSize.IsNull() && plan.BatchSize.ValueInt64() > 0
	batchPercentageSet := !plan.BatchPercentage.IsNull() && plan.BatchPercentage.ValueInt64() > 0

	if needsRollout && (batchSizeSet || batchPercentageSet) {
		rolloutRequest := swagger.KubernetesNodePoolRotateStartRequest{}
		if batchSizeSet {
			rolloutRequest.Count = plan.BatchSize.ValueInt64()
			rolloutRequest.Strategy = "COUNT"
		} else {
			rolloutRequest.Percentage = plan.BatchPercentage.ValueInt64()
			rolloutRequest.Strategy = "PERCENTAGE"
		}

		_, _, rolloutAsyncOperationErr := r.client.APIClient.KubernetesNodePoolsApi.RotateNodePool(
			ctx,
			rolloutRequest,
			projectID,
			stored.ID.ValueString(),
		)
		if rolloutAsyncOperationErr != nil {
			resp.Diagnostics.AddError("Failed to initiate rollout of nodes",
				fmt.Sprintf("Unable to rollout nodes changes: %s", common.UnpackAPIError(rolloutAsyncOperationErr)))
		} else {
			resp.Diagnostics.AddWarning(
				"Rollout initiated",
				"A new rollout has been initiated to replace nodes with the latest config.",
			)
		}
	}

	// the async operation is returning the previous version of the node pool. query for the latest node pool.
	updatedNodePool, httpResp, err := r.client.APIClient.KubernetesNodePoolsApi.GetNodePool(ctx, projectID, kubernetesNodePoolResponse.NodePool.Id)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to get Kubernetes Node Pool", fmt.Sprintf("Failed to get node pool: %s.",
			common.UnpackAPIError(err)))

		return
	}

	var state kubernetesNodePoolResourceModel
	nodePoolToResourceModel(ctx, &updatedNodePool, &plan, &state, &resp.Diagnostics)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesNodePoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var stored kubernetesNodePoolResourceModel

	diags := req.State.Get(ctx, &stored)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, stored.ProjectID.ValueString())

	asyncOperation, _, err := r.client.APIClient.KubernetesNodePoolsApi.DeleteNodePool(ctx, projectID, stored.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete node pool",
			fmt.Sprintf("Error starting a delete node pool operation: %s", common.UnpackAPIError(err)))

		return
	}

	_, _, err = common.AwaitOperationAndResolve[swagger.KubernetesNodePool](ctx, asyncOperation.Operation, projectID, r.client.APIClient.KubernetesNodePoolOperationsApi.GetKubernetesNodePoolsOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete node pool",
			fmt.Sprintf("Error deleting a node pool: %s", common.UnpackAPIError(err)))

		return
	}
}

func (r *kubernetesNodePoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	nodePoolID, projectID, err := common.ParseResourceIdentifiers(req, r.client, "node_pool_id")

	if err != "" {
		resp.Diagnostics.AddError("Invalid resource identifier", err)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), nodePoolID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
}

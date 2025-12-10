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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource = &kubernetesNodePoolResource{}
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
	InstanceIDs                   types.List   `tfsdk:"instance_ids"`
	SSHKey                        types.String `tfsdk:"ssh_key"`
	State                         types.String `tfsdk:"state"`
	Name                          types.String `tfsdk:"name"`
	EphemeralStorageForContainerd types.Bool   `tfsdk:"ephemeral_storage_for_containerd"`
	BatchSize                     types.Int64  `tfsdk:"batch_size"`
	BatchPercentage               types.Int64  `tfsdk:"batch_percentage"`
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
				Description: "Number of nodes to update at a time during rollout (minimum 1, maximum 10). " +
					"Mutually exclusive with batch_percentage. " +
					"If both this and batch_percentage are omitted, existing nodes will not be updated, " +
					"but new nodes will use the new configuration.",
				Validators: []validator.Int64{
					int64validator.Between(1, 10),
				},
			},
			"batch_percentage": schema.Int64Attribute{
				Optional: true,
				Description: "Percentage of nodes to update concurrently during rollout. " +
					"The calculated number will not exceed 10 nodes. Mutually exclusive with batch_size. " +
					"If both this and batch_size are omitted, existing nodes will not be updated, " +
					"but new nodes will use the new configuration.",
				Validators: []validator.Int64{
					int64validator.Between(1, 100),
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

	asyncOperation, _, err := r.client.APIClient.KubernetesNodePoolsApi.CreateNodePool(ctx, swagger.KubernetesNodePoolPostRequest{
		ClusterId:                     plan.ClusterID.ValueString(),
		Count:                         plan.InstanceCount.ValueInt64(),
		IbPartitionId:                 plan.IBPartitionID.ValueString(),
		Name:                          plan.Name.ValueString(),
		NodeLabels:                    nodeLabels,
		NodePoolVersion:               plan.Version.ValueString(),
		ProductName:                   plan.Type.ValueString(),
		SshPublicKey:                  plan.SSHKey.ValueString(),
		SubnetId:                      plan.SubnetID.ValueString(),
		EphemeralStorageForContainerd: plan.EphemeralStorageForContainerd.ValueBool(),
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

	state.ID = types.StringValue(kubernetesNodePoolResponse.NodePool.Id)
	state.ProjectID = types.StringValue(kubernetesNodePoolResponse.NodePool.ProjectId)
	state.InstanceCount = types.Int64Value(kubernetesNodePoolResponse.NodePool.Count)
	state.Version = types.StringValue(kubernetesNodePoolResponse.NodePool.ImageId)
	state.Type = types.StringValue(kubernetesNodePoolResponse.NodePool.Type_)
	state.ClusterID = types.StringValue(kubernetesNodePoolResponse.NodePool.ClusterId)
	state.SubnetID = types.StringValue(kubernetesNodePoolResponse.NodePool.SubnetId)
	state.IBPartitionID = plan.IBPartitionID
	if plan.RequestedNodeLabels.IsUnknown() {
		state.RequestedNodeLabels = types.MapNull(types.StringType)
	} else {
		state.RequestedNodeLabels = plan.RequestedNodeLabels
	}
	state.AllNodeLabels, diags = common.StringMapToTFMap(kubernetesNodePoolResponse.NodePool.NodeLabels)
	resp.Diagnostics.Append(diags...)
	state.InstanceIDs, diags = common.StringSliceToTFList(kubernetesNodePoolResponse.NodePool.InstanceIds)
	resp.Diagnostics.Append(diags...)
	state.State = types.StringValue(kubernetesNodePoolResponse.NodePool.State)
	state.Name = types.StringValue(kubernetesNodePoolResponse.NodePool.Name)
	state.EphemeralStorageForContainerd = types.BoolValue(kubernetesNodePoolResponse.NodePool.EphemeralStorageForContainerd)
	state.SSHKey = plan.SSHKey
	if plan.BatchSize.IsNull() {
		state.BatchSize = types.Int64Null()
	} else {
		state.BatchSize = plan.BatchSize
	}
	if plan.BatchPercentage.IsNull() {
		state.BatchPercentage = types.Int64Null()
	} else {
		state.BatchPercentage = plan.BatchPercentage
	}

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

	// Update state from API response
	state.ID = types.StringValue(kubernetesNodePool.Id)
	state.ProjectID = types.StringValue(kubernetesNodePool.ProjectId)
	state.Version = types.StringValue(kubernetesNodePool.ImageId)
	state.Type = types.StringValue(kubernetesNodePool.Type_)
	state.InstanceCount = types.Int64Value(kubernetesNodePool.Count)
	state.ClusterID = types.StringValue(kubernetesNodePool.ClusterId)
	state.SubnetID = types.StringValue(kubernetesNodePool.SubnetId)
	state.AllNodeLabels, diags = common.StringMapToTFMap(kubernetesNodePool.NodeLabels)
	resp.Diagnostics.Append(diags...)
	state.InstanceIDs, diags = common.StringSliceToTFList(kubernetesNodePool.InstanceIds)
	resp.Diagnostics.Append(diags...)
	state.State = types.StringValue(kubernetesNodePool.State)
	state.Name = types.StringValue(kubernetesNodePool.Name)
	state.EphemeralStorageForContainerd = types.BoolValue(kubernetesNodePool.EphemeralStorageForContainerd)

	// Preserve Terraform-only fields from prior state (not in API)
	state.IBPartitionID = stored.IBPartitionID
	state.RequestedNodeLabels = stored.RequestedNodeLabels
	state.SSHKey = stored.SSHKey
	if stored.BatchSize.IsNull() {
		state.BatchSize = types.Int64Null()
	} else {
		state.BatchSize = stored.BatchSize
	}
	if stored.BatchPercentage.IsNull() {
		state.BatchPercentage = types.Int64Null()
	} else {
		state.BatchPercentage = stored.BatchPercentage
	}

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

	needsRotation := ModifyPlanNodePoolNeedsRotation(ctx, &req, resp)

	if !needsRotation {
		return
	}

	// Check if both batch_size and batch_percentage are null
	if plan.BatchSize.IsNull() && plan.BatchPercentage.IsNull() {
		resp.Diagnostics.AddWarning(
			"Existing nodes will not be updated",
			"Neither batch_size nor batch_percentage is specified. "+
				"Existing nodes will not be updated to the new configuration, "+
				"but new nodes will use the new configuration.",
		)

		return
	}

	// Build warning message based on which parameter is set
	warningMsg := "Using batch_size or batch_percentage during updates will cause nodes to be deleted and recreated in batches. " +
		"This will result in temporary capacity reduction during the rotate process. "

	if !plan.BatchPercentage.IsNull() {
		warningMsg += "The calculated number of nodes based on the percentage will not exceed 10 nodes per batch. "
	}

	warningMsg += "\n\n" +
		"Node rotation can take 5+ minutes and depends on the total number of nodes that need to be rotated." +
		"\n\n" +
		"⚠️ DO NOT INTERRUPT: Stopping this Terraform operation (Ctrl+C) will halt the rotation, " +
		"potentially leaving nodes in a partially updated state. Let the operation complete."

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

	patchRequest := swagger.KubernetesNodePoolPatchRequest{
		Count:                         plan.InstanceCount.ValueInt64(),
		NodeLabels:                    nodeLabels,
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

	// Detect if changes require node rotation
	needsRotation := UpdateNodePoolNeedsRotation(ctx, &req, resp)

	// run rollout if batch_size or batch_percentage is specified
	batchSizeSet := !plan.BatchSize.IsNull() && plan.BatchSize.ValueInt64() > 0
	batchPercentageSet := !plan.BatchPercentage.IsNull() && plan.BatchPercentage.ValueInt64() > 0

	if needsRotation && (batchSizeSet || batchPercentageSet) {
		rolloutRequest := swagger.KubernetesNodePoolRotateStartRequest{}
		if batchSizeSet {
			rolloutRequest.Count = plan.BatchSize.ValueInt64()
			rolloutRequest.Strategy = "COUNT"
		} else {
			rolloutRequest.Percentage = plan.BatchPercentage.ValueInt64()
			rolloutRequest.Strategy = "PERCENTAGE"
		}

		rotateAsyncOperation, _, rotateAsyncOperationErr := r.client.APIClient.KubernetesNodePoolsApi.RotateNodePool(
			ctx,
			rolloutRequest,
			projectID,
			stored.ID.ValueString(),
		)
		if rotateAsyncOperationErr != nil {
			resp.Diagnostics.AddError("Failed to rotate nodes",
				fmt.Sprintf("Unable to rotate nodes: %s", common.UnpackAPIError(rotateAsyncOperationErr)))

			return
		}

		// Wait for rotate operation to complete
		_, err = AwaitNodePoolOperation(ctx, rotateAsyncOperation.Operation, projectID, r.client.APIClient)
		if err != nil {
			resp.Diagnostics.AddError("Failed to rotate nodes",
				fmt.Sprintf("Unable to rotate nodes: %s", common.UnpackAPIError(err)))

			return
		}
	}

	// the async operation is returning the previous version of the node pool. query for the latest node pool.
	updatedNodePool, httpResp, err := r.client.APIClient.KubernetesNodePoolsApi.GetNodePool(ctx, projectID, kubernetesNodePoolResponse.NodePool.Id)
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

	// Populate state with values from API response
	state.ID = types.StringValue(updatedNodePool.Id)
	state.ProjectID = types.StringValue(updatedNodePool.ProjectId)
	state.InstanceCount = types.Int64Value(updatedNodePool.Count)
	state.Version = types.StringValue(updatedNodePool.ImageId)
	state.Type = types.StringValue(updatedNodePool.Type_)
	state.ClusterID = types.StringValue(updatedNodePool.ClusterId)
	state.SubnetID = types.StringValue(updatedNodePool.SubnetId)
	state.AllNodeLabels, diags = common.StringMapToTFMap(updatedNodePool.NodeLabels)
	resp.Diagnostics.Append(diags...)
	state.InstanceIDs, diags = common.StringSliceToTFList(updatedNodePool.InstanceIds)
	resp.Diagnostics.Append(diags...)
	state.State = types.StringValue(updatedNodePool.State)
	state.Name = types.StringValue(updatedNodePool.Name)
	state.EphemeralStorageForContainerd = types.BoolValue(updatedNodePool.EphemeralStorageForContainerd)

	// Preserve fields not returned by API
	state.RequestedNodeLabels = plan.RequestedNodeLabels
	state.IBPartitionID = plan.IBPartitionID
	state.SSHKey = plan.SSHKey
	state.BatchSize = plan.BatchSize
	state.BatchPercentage = plan.BatchPercentage

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

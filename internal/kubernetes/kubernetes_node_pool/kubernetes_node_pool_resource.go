package kubernetes_node_pool

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
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
	client *swagger.APIClient
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
}

func (r *kubernetesNodePoolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*swagger.APIClient)
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
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplaceIfConfigured()}, // cannot be updated in place
			},
			"version": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
				Validators: []validator.String{stringvalidator.RegexMatches(
					regexp.MustCompile(`\d+\.\d+\.\d+-cmk\.\d+.*`), "must be in the format MAJOR.MINOR.BUGFIX-cmk.NUM (e.g 1.2.3-cmk.4)",
				)},
			},
			"type": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"instance_count": schema.Int64Attribute{
				Required:      true,
				PlanModifiers: []planmodifier.Int64{},
			},
			"cluster_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"subnet_id": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplaceIfConfigured()}, // cannot be updated in place
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
				ElementType:   types.StringType,
				Computed:      true,
				PlanModifiers: []planmodifier.Map{}, // cannot be updated in place
			},
			"instance_ids": schema.ListAttribute{
				ElementType:   types.StringType,
				Computed:      true,
				PlanModifiers: []planmodifier.List{}, // cannot be updated in place
			},
			"ssh_key": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"state": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{},
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"ephemeral_storage_for_containerd": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesNodePoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesNodePoolResourceModel
	diags := req.Config.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID, err := common.GetProjectIDOrFallback(ctx, r.client, &resp.Diagnostics, plan.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch project ID",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	nodeLabels, err := common.TFMapToStringMap(plan.RequestedNodeLabels)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create node pool", fmt.Sprintf("error when parsing requested_node_labels as string map: %s", err))

		return
	}

	asyncOperation, _, err := r.client.KubernetesNodePoolsApi.CreateNodePool(ctx, swagger.KubernetesNodePoolPostRequest{
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
	kubernetesNodePoolResponse, err := AwaitNodePoolOperation(ctx, asyncOperation.Operation, projectID, r.client)
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
	state.RequestedNodeLabels = plan.RequestedNodeLabels
	state.AllNodeLabels, diags = common.StringMapToTFMap(kubernetesNodePoolResponse.NodePool.NodeLabels)
	resp.Diagnostics.Append(diags...)
	state.InstanceIDs, diags = common.StringSliceToTFList(kubernetesNodePoolResponse.NodePool.InstanceIds)
	resp.Diagnostics.Append(diags...)
	state.State = types.StringValue(kubernetesNodePoolResponse.NodePool.State)
	state.Name = types.StringValue(kubernetesNodePoolResponse.NodePool.Name)
	state.EphemeralStorageForContainerd = types.BoolValue(kubernetesNodePoolResponse.NodePool.EphemeralStorageForContainerd)

	state.SSHKey = plan.SSHKey

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

	projectID, err := common.GetProjectIDOrFallback(ctx, r.client, &resp.Diagnostics, stored.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch project ID",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	kubernetesNodePool, httpResp, err := r.client.KubernetesNodePoolsApi.GetNodePool(ctx, projectID, stored.ID.ValueString())
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

	diags = resp.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue(kubernetesNodePool.Id)
	state.ProjectID = types.StringValue(kubernetesNodePool.ProjectId)
	state.Version = types.StringValue(kubernetesNodePool.ImageId)
	state.Type = types.StringValue(kubernetesNodePool.Type_)
	state.InstanceCount = types.Int64Value(kubernetesNodePool.Count)
	state.ClusterID = types.StringValue(kubernetesNodePool.ClusterId)
	state.SubnetID = types.StringValue(kubernetesNodePool.SubnetId)
	state.IBPartitionID = stored.IBPartitionID
	state.RequestedNodeLabels = stored.RequestedNodeLabels
	state.AllNodeLabels, diags = common.StringMapToTFMap(kubernetesNodePool.NodeLabels)
	resp.Diagnostics.Append(diags...)
	state.InstanceIDs, diags = common.StringSliceToTFList(kubernetesNodePool.InstanceIds)
	resp.Diagnostics.Append(diags...)
	state.SSHKey = stored.SSHKey
	state.State = types.StringValue(kubernetesNodePool.State)
	state.Name = types.StringValue(kubernetesNodePool.Name)
	state.EphemeralStorageForContainerd = types.BoolValue(kubernetesNodePool.EphemeralStorageForContainerd)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
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
	if plan.InstanceCount.ValueInt64() < stored.InstanceCount.ValueInt64() {
		resp.Diagnostics.AddAttributeWarning(path.Root("instance_count"), "Node pool instance count decreased", "Decreasing node pool instance count will not delete node pool VMs. Manual deletion is required.")
	}

	projectID, err := common.GetProjectIDOrFallback(ctx, r.client, &resp.Diagnostics, stored.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch project ID",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	nodeLabels, err := common.TFMapToStringMap(plan.RequestedNodeLabels)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update node pool", fmt.Sprintf("error when parsing requested_node_labels as string map: %s", err))

		return
	}

	patchRequest := swagger.KubernetesNodePoolPatchRequest{
		Count:                         plan.InstanceCount.ValueInt64(),
		NodeLabels:                    nodeLabels,
		EphemeralStorageForContainerd: plan.EphemeralStorageForContainerd.ValueBool(),
	}

	asyncOperation, _, err := r.client.KubernetesNodePoolsApi.UpdateNodePool(
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

	// Wait for operation to complete
	kubernetesNodePoolResponse, err := AwaitNodePoolOperation(ctx, asyncOperation.Operation, projectID, r.client)
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

	var state kubernetesNodePoolResourceModel

	// Update state
	state.ID = types.StringValue(kubernetesNodePoolResponse.NodePool.Id)
	state.ProjectID = types.StringValue(kubernetesNodePoolResponse.NodePool.ProjectId)
	state.InstanceCount = types.Int64Value(kubernetesNodePoolResponse.NodePool.Count) // For now, this is the only field that we expect to change
	state.Version = types.StringValue(kubernetesNodePoolResponse.NodePool.ImageId)
	state.Type = types.StringValue(kubernetesNodePoolResponse.NodePool.Type_)
	state.ClusterID = types.StringValue(kubernetesNodePoolResponse.NodePool.ClusterId)
	state.SubnetID = types.StringValue(kubernetesNodePoolResponse.NodePool.SubnetId)
	state.IBPartitionID = plan.IBPartitionID
	state.RequestedNodeLabels = plan.RequestedNodeLabels
	state.AllNodeLabels, diags = common.StringMapToTFMap(kubernetesNodePoolResponse.NodePool.NodeLabels)
	resp.Diagnostics.Append(diags...)
	state.InstanceIDs, diags = common.StringSliceToTFList(kubernetesNodePoolResponse.NodePool.InstanceIds)
	resp.Diagnostics.Append(diags...)
	state.State = types.StringValue(kubernetesNodePoolResponse.NodePool.State)
	state.Name = types.StringValue(kubernetesNodePoolResponse.NodePool.Name)
	state.EphemeralStorageForContainerd = types.BoolValue(kubernetesNodePoolResponse.NodePool.EphemeralStorageForContainerd)

	state.SSHKey = plan.SSHKey

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

	projectID, err := common.GetProjectIDOrFallback(ctx, r.client, &resp.Diagnostics, stored.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch project ID",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	asyncOperation, _, err := r.client.KubernetesNodePoolsApi.DeleteNodePool(ctx, projectID, stored.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete node pool",
			fmt.Sprintf("Error starting a delete node pool operation: %s", common.UnpackAPIError(err)))

		return
	}

	_, _, err = common.AwaitOperationAndResolve[swagger.KubernetesNodePool](ctx, asyncOperation.Operation, projectID, r.client.KubernetesNodePoolOperationsApi.GetKubernetesNodePoolsOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete node pool",
			fmt.Sprintf("Error deleting a node pool: %s", common.UnpackAPIError(err)))

		return
	}
}

func (r *kubernetesNodePoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceIdentifiers := strings.Split(req.ID, ",")

	// We allow "node_pool_id" (project_id is implicitly defined via env variable) or "node_pool_id,project_id" (explicit project_id)
	if len(resourceIdentifiers) != 1 && len(resourceIdentifiers) != 2 {
		resp.Diagnostics.AddError("Invalid resource identifier", fmt.Sprintf("Expected format node_pool_id,project_id, got %q", req.ID))

		return
	}

	nodePoolID := resourceIdentifiers[0]
	var projectID string

	if len(resourceIdentifiers) == 2 {
		projectID = resourceIdentifiers[1]
	}

	projectID, err := common.GetProjectIDOrFallback(ctx, r.client, &resp.Diagnostics, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch project ID",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	if _, parseErr := uuid.Parse(nodePoolID); parseErr != nil {
		resp.Diagnostics.AddError("Invalid resource identifier", fmt.Sprintf("Failed to parse node pool ID: %v", parseErr))

		return
	}

	if _, parseErr := uuid.Parse(projectID); parseErr != nil {
		resp.Diagnostics.AddError("Invalid resource identifier", fmt.Sprintf("Failed to parse project ID: %v", parseErr))

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), nodePoolID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
}

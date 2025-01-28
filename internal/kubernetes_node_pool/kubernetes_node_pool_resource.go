package kubernetes_node_pool

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type kubernetesNodePoolResource struct {
	client *swagger.APIClient
}

func NewKubernetesNodePoolResource() resource.Resource {
	return &kubernetesNodePoolResource{}
}

type kubernetesNodePoolResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	ProjectID           types.String `tfsdk:"project_id"`
	Version             types.String `tfsdk:"version"`
	Type                types.String `tfsdk:"type"`
	InstanceCount       types.Int64  `tfsdk:"instance_count"`
	ClusterID           types.String `tfsdk:"cluster_id"`
	SubnetID            types.String `tfsdk:"subnet_id"`
	IBPartitionID       types.String `tfsdk:"ib_partition_id"`
	RequestedNodeLabels types.Map    `tfsdk:"requested_node_labels"`
	AllNodeLabels       types.Map    `tfsdk:"all_node_labels"`
	InstanceIDs         types.List   `tfsdk:"instance_ids"`
	SSHKey              types.String `tfsdk:"ssh_key"`
	State               types.String `tfsdk:"state"`
	Name                types.String `tfsdk:"name"`
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
	//nolint:gocritic // regex intentionally uses [0-9] to not match non ascii digits
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"project_id": schema.StringAttribute{
				Computed:      true,
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"version": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
				Validators: []validator.String{stringvalidator.RegexMatches(
					regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+-cmk\.[0-9]+.*`), "must be in the format MAJOR.MINOR.BUGFIX-cmk.NUM (e.g 1.2.3-cmk.4)",
				)},
			},
			"type": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"instance_count": schema.Int64Attribute{
				Required:      true,
				PlanModifiers: []planmodifier.Int64{}, // TODO: implement update
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
				PlanModifiers: []planmodifier.Map{mapplanmodifier.UseStateForUnknown(), mapplanmodifier.RequiresReplace()}, // cannot be updated in place
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

	var state kubernetesNodePoolResourceModel

	projectID := plan.ProjectID.ValueString()

	if projectID == "" {
		fallbackProjectID, fallbackErr := common.GetFallbackProject(ctx, r.client, &resp.Diagnostics)
		if fallbackErr != nil {
			resp.Diagnostics.AddError("Failed to fetch Node Pools",
				fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", fallbackErr))

			return
		}
		projectID = fallbackProjectID
	}

	nodeLabels, err := common.TFMapToStringMap(plan.RequestedNodeLabels)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create node pool", fmt.Sprintf("error when parsing requested_node_labels as string map: %s", err))

		return
	}

	asyncOperation, _, err := r.client.KubernetesNodePoolsApi.CreateNodePool(ctx, swagger.KubernetesNodePoolPostRequest{
		ClusterId:       plan.ClusterID.ValueString(),
		Count:           plan.InstanceCount.ValueInt64(),
		IbPartitionId:   plan.IBPartitionID.ValueString(),
		Name:            plan.Name.ValueString(),
		NodeLabels:      nodeLabels,
		NodePoolVersion: plan.Version.ValueString(),
		ProductName:     plan.Type.ValueString(),
		SshPublicKey:    plan.SSHKey.ValueString(),
		SubnetId:        plan.SubnetID.ValueString(),
	}, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create node pool",
			fmt.Sprintf("Error starting a create node pool operation: %s", common.UnpackAPIError(err)))

		return
	}

	kubernetesNodePool, _, err := common.AwaitOperationAndResolve[swagger.KubernetesNodePool](ctx, asyncOperation.Operation, projectID, r.client.KubernetesNodePoolOperationsApi.GetKubernetesNodePoolsOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create node pool",
			fmt.Sprintf("Error creating a node pool: %s", common.UnpackAPIError(err)))

		return
	}

	state.ID = types.StringValue(kubernetesNodePool.Id)
	state.ProjectID = types.StringValue(kubernetesNodePool.ProjectId)
	state.State = types.StringValue(kubernetesNodePool.State)
	state.InstanceCount = types.Int64Value(kubernetesNodePool.Count)
	state.Version = types.StringValue(kubernetesNodePool.ImageId)
	state.Type = types.StringValue(kubernetesNodePool.Type_)
	state.ClusterID = types.StringValue(kubernetesNodePool.ClusterId)
	state.SubnetID = types.StringValue(kubernetesNodePool.SubnetId)
	state.IBPartitionID = plan.IBPartitionID
	state.RequestedNodeLabels = plan.RequestedNodeLabels
	state.AllNodeLabels, diags = common.StringMapToTFMap(kubernetesNodePool.NodeLabels)
	resp.Diagnostics.Append(diags...)
	state.InstanceIDs, diags = common.StringSliceToTFList(kubernetesNodePool.InstanceIds)
	resp.Diagnostics.Append(diags...)
	state.Name = types.StringValue(kubernetesNodePool.Name)

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

	var state kubernetesNodePoolResourceModel

	diags = resp.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	projectID, err := common.GetProjectIDOrFallback(ctx, r.client, &resp.Diagnostics, state.ProjectID.ValueString())
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

	state.ID = types.StringValue(kubernetesNodePool.Id)
	state.ProjectID = types.StringValue(kubernetesNodePool.ProjectId)
	state.State = types.StringValue(kubernetesNodePool.State)
	state.InstanceCount = types.Int64Value(kubernetesNodePool.Count)
	state.Version = types.StringValue(kubernetesNodePool.ImageId)
	state.Type = types.StringValue(kubernetesNodePool.Type_)
	state.ClusterID = types.StringValue(kubernetesNodePool.ClusterId)
	state.SubnetID = types.StringValue(kubernetesNodePool.SubnetId)
	state.AllNodeLabels, diags = common.StringMapToTFMap(kubernetesNodePool.NodeLabels)
	resp.Diagnostics.Append(diags...)
	state.InstanceIDs, diags = common.StringSliceToTFList(kubernetesNodePool.InstanceIds)
	resp.Diagnostics.Append(diags...)
	state.SSHKey = stored.SSHKey
	state.State = types.StringValue(kubernetesNodePool.State)
	state.Name = types.StringValue(kubernetesNodePool.Name)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesNodePoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Updating node pool instance count is currently an imperative operation
	panic("Updating nodepool instance count is not currently supported")
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

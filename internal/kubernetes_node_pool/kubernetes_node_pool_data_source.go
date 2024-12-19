package kubernetes_node_pool

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type kubernetesNodePoolDataSource struct {
	client *swagger.APIClient
}

func NewKubernetesNodePoolDataSource() datasource.DataSource {
	return &kubernetesNodePoolDataSource{}
}

type kubernetesNodePoolDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	ProjectID     types.String `tfsdk:"project_id"`
	ImageID       types.String `tfsdk:"image_id"`
	Type          types.String `tfsdk:"type"`
	InstanceCount types.Int64  `tfsdk:"instance_count"`
	ClusterID     types.String `tfsdk:"cluster_id"`
	SubnetID      types.String `tfsdk:"subnet_id"`
	NodeLabels    types.Map    `tfsdk:"node_labels"`
	InstanceIDs   types.List   `tfsdk:"instance_ids"`
	State         types.String `tfsdk:"state"`
	Name          types.String `tfsdk:"name"`
}

func (e *kubernetesNodePoolDataSource) Metadata(_ context.Context,
	req datasource.MetadataRequest,
	resp *datasource.MetadataResponse,
) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_node_pool"
}

func (e *kubernetesNodePoolDataSource) Schema(_ context.Context,
	req datasource.SchemaRequest,
	resp *datasource.SchemaResponse,
) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required: true,
			},
			"project_id": schema.StringAttribute{
				Optional: true,
			},
			"image_id": schema.StringAttribute{
				Optional: true,
			},
			"type": schema.StringAttribute{
				Optional: true,
			},
			"instance_count": schema.Int64Attribute{
				Optional: true,
			},
			"cluster_id": schema.StringAttribute{
				Optional: true,
			},
			"subnet_id": schema.StringAttribute{
				Optional: true,
			},
			"node_labels": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
			},
			"instance_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
			},
			"state": schema.StringAttribute{
				Optional: true,
			},
			"name": schema.StringAttribute{
				Optional: true,
			},
		},
	}
}

func (d *kubernetesNodePoolDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*swagger.APIClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", common.ErrorMsgProviderInitFailed)

		return
	}

	d.client = client
}

//nolint:gocritic // Implements Terraform defined interface
func (d *kubernetesNodePoolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config kubernetesNodePoolDataSourceModel

	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state kubernetesNodePoolDataSourceModel

	diags = resp.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID, err := common.GetProjectIDOrFallback(ctx, d.client, &resp.Diagnostics, state.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch project ID",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	kubernetesNodePool, _, err := d.client.KubernetesNodePoolsApi.GetNodePool(ctx, projectID, config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Kubernetes Node Pool", fmt.Sprintf("Failed to get node pool: %s.",
			common.UnpackAPIError(err)))

		return
	}

	state.ID = types.StringValue(kubernetesNodePool.Id)
	state.ProjectID = types.StringValue(projectID)
	state.ImageID = types.StringValue(kubernetesNodePool.ImageId)
	state.Type = types.StringValue(kubernetesNodePool.Type_)
	state.InstanceCount = types.Int64Value(kubernetesNodePool.Count)
	state.ClusterID = types.StringValue(kubernetesNodePool.ClusterId)
	state.SubnetID = types.StringValue(kubernetesNodePool.SubnetId)
	state.NodeLabels, diags = common.StringMapToTFMap(kubernetesNodePool.NodeLabels)
	resp.Diagnostics.Append(diags...)
	state.InstanceIDs, diags = common.StringSliceToTFList(kubernetesNodePool.InstanceIds)
	resp.Diagnostics.Append(diags...)
	state.State = types.StringValue(kubernetesNodePool.State)
	state.Name = types.StringValue(kubernetesNodePool.Name)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

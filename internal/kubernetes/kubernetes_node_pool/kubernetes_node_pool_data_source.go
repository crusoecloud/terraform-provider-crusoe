package kubernetes_node_pool

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource = &kubernetesNodePoolDataSource{}
)

type kubernetesNodePoolDataSource struct {
	client *common.CrusoeClient
}

func NewKubernetesNodePoolDataSource() datasource.DataSource {
	return &kubernetesNodePoolDataSource{}
}

type kubernetesNodePoolDataSourceModel struct {
	ID                            types.String `tfsdk:"id"`
	ProjectID                     types.String `tfsdk:"project_id"`
	Version                       types.String `tfsdk:"version"`
	Type                          types.String `tfsdk:"type"`
	InstanceCount                 types.Int64  `tfsdk:"instance_count"`
	ClusterID                     types.String `tfsdk:"cluster_id"`
	SubnetID                      types.String `tfsdk:"subnet_id"`
	NodeLabels                    types.Map    `tfsdk:"node_labels"`
	InstanceIDs                   types.List   `tfsdk:"instance_ids"`
	State                         types.String `tfsdk:"state"`
	Name                          types.String `tfsdk:"name"`
	EphemeralStorageForContainerd types.Bool   `tfsdk:"ephemeral_storage_for_containerd"`
	NvlinkDomainID                types.String `tfsdk:"nvlink_domain_id"`
	PublicIPType                  types.String `tfsdk:"public_ip_type"`
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
			"ephemeral_storage_for_containerd": schema.BoolAttribute{
				Optional: true,
			},
			"nvlink_domain_id": schema.StringAttribute{
				Optional: true,
			},
			"public_ip_type": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (ds *kubernetesNodePoolDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*common.CrusoeClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", common.ErrorMsgProviderInitFailed)

		return
	}

	ds.client = client
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *kubernetesNodePoolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config kubernetesNodePoolDataSourceModel

	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(ds.client, config.ProjectID.ValueString())

	kubernetesNodePool, _, err := ds.client.APIClient.KubernetesNodePoolsApi.GetNodePool(ctx, projectID, config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Kubernetes Node Pool", fmt.Sprintf("Failed to get node pool: %s.",
			common.UnpackAPIError(err)))

		return
	}

	var state kubernetesNodePoolDataSourceModel

	diags = resp.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue(kubernetesNodePool.Id)
	state.ProjectID = types.StringValue(projectID)
	state.Version = types.StringValue(kubernetesNodePool.ImageId)
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
	state.EphemeralStorageForContainerd = types.BoolValue(kubernetesNodePool.EphemeralStorageForContainerd)
	state.NvlinkDomainID = types.StringValue(kubernetesNodePool.NvlinkDomainId)
	state.PublicIPType = types.StringValue(kubernetesNodePool.PublicIpType)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

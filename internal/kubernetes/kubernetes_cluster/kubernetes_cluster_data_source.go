package kubernetes_cluster

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
	_ datasource.DataSource = &kubernetesClusterDataSource{}
)

// NewKubernetesClusterDataSource is a helper function to simplify the provider implementation.
func NewKubernetesClusterDataSource() datasource.DataSource {
	return &kubernetesClusterDataSource{}
}

// kubernetesClusterDataSource is the data source implementation.
type kubernetesClusterDataSource struct {
	client *common.CrusoeClient
}

type kubernetesClusterDataSourceModel struct {
	ID                    types.String `tfsdk:"id"`
	ProjectID             types.String `tfsdk:"project_id"`
	Name                  types.String `tfsdk:"name"`
	Version               types.String `tfsdk:"version"`
	SubnetID              types.String `tfsdk:"subnet_id"`
	ClusterCidr           types.String `tfsdk:"cluster_cidr"`
	NodeCidrMaskSize      types.Int64  `tfsdk:"node_cidr_mask_size"`
	ServiceClusterIpRange types.String `tfsdk:"service_cluster_ip_range"`
	AddOns                types.List   `tfsdk:"add_ons"`
	Location              types.String `tfsdk:"location"`
	DNSName               types.String `tfsdk:"dns_name"`
	NodePoolIds           types.List   `tfsdk:"nodepool_ids"`
	Private               types.Bool   `tfsdk:"private"`
}

func (ds *kubernetesClusterDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

// Metadata returns the data source type name.
func (ds *kubernetesClusterDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_cluster"
}

// Schema defines the schema for the data source.
func (ds *kubernetesClusterDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required: true,
			},
			"project_id": schema.StringAttribute{
				Optional: true,
			},
			"name": schema.StringAttribute{
				Optional: true,
			},
			"version": schema.StringAttribute{
				Optional: true,
			},
			"subnet_id": schema.StringAttribute{
				Optional: true,
			},
			"cluster_cidr": schema.StringAttribute{
				Optional: true,
			},
			"node_cidr_mask_size": schema.Int64Attribute{
				Optional: true,
			},
			"service_cluster_ip_range": schema.StringAttribute{
				Optional: true,
			},
			"add_ons": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
			},
			"location": schema.StringAttribute{
				Optional: true,
			},
			"dns_name": schema.StringAttribute{
				Computed: true,
			},
			"nodepool_ids": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
			},
			"private": schema.BoolAttribute{
				Computed: true,
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *kubernetesClusterDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config kubernetesClusterDataSourceModel

	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(ds.client, config.ProjectID.ValueString())

	kubernetesCluster, _, err := ds.client.APIClient.KubernetesClustersApi.GetCluster(ctx, projectID, config.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read Kubernetes Cluster",
			fmt.Sprintf("Error reading the Kubernetes Cluster: %s", common.UnpackAPIError(err)))

		return
	}

	var state kubernetesClusterDataSourceModel

	diags = resp.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue(kubernetesCluster.Id)
	state.ProjectID = types.StringValue(kubernetesCluster.ProjectId)
	state.Name = types.StringValue(kubernetesCluster.Name)
	state.Version = types.StringValue(kubernetesCluster.Version)
	state.SubnetID = types.StringValue(kubernetesCluster.SubnetId)
	state.NodeCidrMaskSize = types.Int64Value(int64(kubernetesCluster.NodeCidrMaskSize))
	state.ClusterCidr = types.StringValue(kubernetesCluster.ClusterCidr)
	state.ServiceClusterIpRange = types.StringValue(kubernetesCluster.ServiceClusterIpRange)
	state.AddOns, diags = common.StringSliceToTFList(kubernetesCluster.AddOns)
	resp.Diagnostics.Append(diags...)
	state.Location = types.StringValue(kubernetesCluster.Location)
	state.DNSName = types.StringValue(kubernetesCluster.DnsName)
	state.NodePoolIds, diags = common.StringSliceToTFList(kubernetesCluster.NodePools)
	state.Private = types.BoolValue(kubernetesCluster.Private)
	resp.Diagnostics.Append(diags...)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

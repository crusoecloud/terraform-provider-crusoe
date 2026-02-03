package vpc_network

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type vpcNetworksDataSource struct {
	client *common.CrusoeClient
}

type vpcNetworksDataSourceModel struct {
	ProjectID   types.String       `tfsdk:"project_id"`
	VPCNetworks []vpcNetworksModel `tfsdk:"vpc_networks"`
}

type vpcNetworksModel struct {
	ID      string   `tfsdk:"id"`
	Name    string   `tfsdk:"name"`
	CIDR    string   `tfsdk:"cidr"`
	Gateway string   `tfsdk:"gateway"`
	Subnets []string `tfsdk:"subnets"`
}

func NewVPCNetworksDataSource() datasource.DataSource {
	return &vpcNetworksDataSource{}
}

// Configure adds the provider configured client to the data source.
func (ds *vpcNetworksDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (ds *vpcNetworksDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_vpc_networks"
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *vpcNetworksDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"vpc_networks": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed: true,
					},
					"name": schema.StringAttribute{
						Required: true,
					},
					"cidr": schema.StringAttribute{
						Required: true,
					},
					"gateway": schema.StringAttribute{
						Required: true,
					},
					"subnets": schema.ListAttribute{
						ElementType: types.StringType,
						Optional:    true,
					},
				},
			},
		},
		"project_id": schema.StringAttribute{
			Optional: true,
		},
	}}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *vpcNetworksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config vpcNetworksDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(ds.client, config.ProjectID.ValueString())

	dataResp, httpResp, err := ds.client.APIClient.VPCNetworksApi.ListVPCNetworks(ctx, projectID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to Fetch VPC Networks", "Could not fetch VPC Network data at this time.")

		return
	}

	var state vpcNetworksDataSourceModel
	for i := range dataResp.Items {
		state.VPCNetworks = append(state.VPCNetworks, vpcNetworksModel{
			ID:      dataResp.Items[i].Id,
			Name:    dataResp.Items[i].Name,
			CIDR:    dataResp.Items[i].Cidr,
			Gateway: dataResp.Items[i].Gateway,
			Subnets: dataResp.Items[i].Subnets,
		})
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

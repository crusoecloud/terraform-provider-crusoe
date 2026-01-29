package vpc_subnet

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type vpcSubnetsDataSource struct {
	client *common.CrusoeClient
}

type vpcSubnetsDataSourceModel struct {
	ProjectID  *string           `tfsdk:"project_id"`
	VPCSubnets []vpcSubnetsModel `tfsdk:"vpc_subnets"`
}

type vpcSubnetsModel struct {
	ID       string `tfsdk:"id"`
	Name     string `tfsdk:"name"`
	CIDR     string `tfsdk:"cidr"`
	Location string `tfsdk:"location"`
	Network  string `tfsdk:"network"`
}

func NewVPCSubnetsDataSource() datasource.DataSource {
	return &vpcSubnetsDataSource{}
}

// Configure adds the provider configured client to the data source.
func (ds *vpcSubnetsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (ds *vpcSubnetsDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_vpc_subnets"
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *vpcSubnetsDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"vpc_subnets": schema.ListNestedAttribute{
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
					"location": schema.StringAttribute{
						Computed: true,
					},
					"network": schema.StringAttribute{
						Computed: true,
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
func (ds *vpcSubnetsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config vpcSubnetsDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDFromPointerOrFallback(ds.client, config.ProjectID)

	dataResp, httpResp, err := ds.client.APIClient.VPCSubnetsApi.ListVPCSubnets(ctx, projectID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to Fetch VPC Subnets", "Could not fetch VPC Subnet data at this time.")

		return
	}

	var state vpcSubnetsDataSourceModel
	for i := range dataResp.Items {
		state.VPCSubnets = append(state.VPCSubnets, vpcSubnetsModel{
			ID:       dataResp.Items[i].Id,
			Name:     dataResp.Items[i].Name,
			CIDR:     dataResp.Items[i].Cidr,
			Location: dataResp.Items[i].Location,
			Network:  dataResp.Items[i].VpcNetworkId,
		})
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

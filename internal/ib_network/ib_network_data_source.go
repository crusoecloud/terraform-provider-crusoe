//nolint:gocritic // Implements Terraform defined interface
package ib_network

import (
	"context"
	"fmt"


	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha4"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type ibNetworksDataSource struct {
	client *swagger.APIClient
}

type ibNetworksDataSourceModel struct {
	IBNetworks []ibNetworkModel `tfsdk:"ib_networks"`
}

type ibNetworkModel struct {
	ID       string `tfsdk:"id"`
	Name     string `tfsdk:"name"`
	Location string `tfsdk:"location"`
}

func NewIBNetworkDataSource() datasource.DataSource {
	return &ibNetworksDataSource{}
}

// Configure adds the provider configured client to the data source.
func (ds *ibNetworksDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*swagger.APIClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", common.ErrorMsgProviderInitFailed)

		return
	}

	ds.client = client
}

func (ds *ibNetworksDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_ib_networks"
}

func (ds *ibNetworksDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"ib_networks": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed: true,
					},
					"name": schema.StringAttribute{
						Computed: true,
					},
					"location": schema.StringAttribute{
						Computed: true,
					},
				},
			},
		},
	}}
}

func (ds *ibNetworksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	dataResp, httpResp, err := ds.client.IBNetworksApi.GetIBNetworks(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to Fetch IB Networks",
			fmt.Sprintf("Could not fetch Infiniband network data at this time: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	var state ibNetworksDataSourceModel
	for i := range dataResp.IbNetworks {
		state.IBNetworks = append(state.IBNetworks, ibNetworkModel{
			ID:       dataResp.IbNetworks[i].Id,
			Name:     dataResp.IbNetworks[i].Name,
			Location: dataResp.IbNetworks[i].Location,
		})
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

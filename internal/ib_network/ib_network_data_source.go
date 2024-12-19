//nolint:gocritic // Implements Terraform defined interface
package ib_network

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type ibNetworksDataSource struct {
	client *swagger.APIClient
}

type ibNetworksDataSourceModel struct {
	IBNetworks []ibNetworkModel `tfsdk:"ib_networks"`
}

type ibNetworkCapacityModel struct {
	Quantity  int64  `tfsdk:"quantity"`
	SliceType string `tfsdk:"slice_type"`
}

type ibNetworkModel struct {
	ID         string                   `tfsdk:"id"`
	Name       string                   `tfsdk:"name"`
	Location   string                   `tfsdk:"location"`
	Capacities []ibNetworkCapacityModel `tfsdk:"capacities"`
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
					"capacities": schema.ListNestedAttribute{
						Computed: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"quantity": schema.Int64Attribute{
									Computed: true,
								},
								"slice_type": schema.StringAttribute{
									Computed: true,
								},
							},
						},
					},
				},
			},
		},
	}}
}

func (ds *ibNetworksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	projectID, err := common.GetFallbackProject(ctx, ds.client, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch IB networks",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	dataResp, httpResp, err := ds.client.IBNetworksApi.ListIBNetworks(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to Fetch IB Networks",
			fmt.Sprintf("Could not fetch Infiniband network data at this time: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	var state ibNetworksDataSourceModel
	for i := range dataResp.Items {
		capacities := make([]ibNetworkCapacityModel, 0, len(dataResp.Items[i].Capacities))
		for _, c := range dataResp.Items[i].Capacities {
			capacities = append(capacities, ibNetworkCapacityModel{
				Quantity:  int64(c.Quantity),
				SliceType: c.SliceType,
			})
		}
		state.IBNetworks = append(state.IBNetworks, ibNetworkModel{
			ID:         dataResp.Items[i].Id,
			Name:       dataResp.Items[i].Name,
			Location:   dataResp.Items[i].Location,
			Capacities: capacities,
		})
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

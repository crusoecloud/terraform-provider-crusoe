//nolint:gocritic // Implements Terraform defined interface
package transport_network

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type transportNetworksDataSource struct {
	client *common.CrusoeClient
}

type transportNetworksDataSourceModel struct {
	ProjectID         types.String            `tfsdk:"project_id"`
	TransportNetworks []transportNetworkModel `tfsdk:"transport_networks"`
}

type transportNetworkCapacityModel struct {
	Quantity  int64  `tfsdk:"quantity"`
	SliceType string `tfsdk:"slice_type"`
}

type transportNetworkModel struct {
	ID         string                          `tfsdk:"id"`
	Name       string                          `tfsdk:"name"`
	Location   string                          `tfsdk:"location"`
	Capacities []transportNetworkCapacityModel `tfsdk:"capacities"`
}

func NewTransportNetworkDataSource() datasource.DataSource {
	return &transportNetworksDataSource{}
}

// Configure adds the provider configured client to the data source.
func (ds *transportNetworksDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (ds *transportNetworksDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_transport_networks"
}

func (ds *transportNetworksDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"transport_networks": schema.ListNestedAttribute{
			Computed:    true,
			Description: providerDescTransportNetworks,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed:    true,
						Description: apiDescID,
					},
					"name": schema.StringAttribute{
						Computed:    true,
						Description: apiDescName,
					},
					"location": schema.StringAttribute{
						Computed:    true,
						Description: apiDescLocation,
					},
					"capacities": schema.ListNestedAttribute{
						Computed:    true,
						Description: apiDescCapacities,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"quantity": schema.Int64Attribute{
									Computed:    true,
									Description: apiDescCapacityQuantity,
								},
								"slice_type": schema.StringAttribute{
									Computed:    true,
									Description: apiDescCapacitySliceType,
								},
							},
						},
					},
				},
			},
		},
		"project_id": schema.StringAttribute{
			Optional:    true,
			Description: providerDescProjectID,
		},
	}}
}

func (ds *transportNetworksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config transportNetworksDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(ds.client, config.ProjectID.ValueString())

	dataResp, httpResp, err := ds.client.APIClient.IBNetworksApi.ListIBNetworks(ctx, projectID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to Fetch Transport Networks",
			fmt.Sprintf("Could not fetch transport network data at this time: %s", common.UnpackAPIError(err)))

		return
	}

	var state transportNetworksDataSourceModel
	state.TransportNetworks = transportNetworksToModel(dataResp.Items)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// transportNetworksToModel maps API transport networks to the Terraform model.
// Capacities are built per network so each network carries only its own.
func transportNetworksToModel(items []swagger.IbNetwork) []transportNetworkModel {
	networks := make([]transportNetworkModel, 0, len(items))
	for i := range items {
		capacities := make([]transportNetworkCapacityModel, 0, len(items[i].Capacities))
		for _, c := range items[i].Capacities {
			capacities = append(capacities, transportNetworkCapacityModel{
				Quantity:  int64(c.Quantity),
				SliceType: c.SliceType,
			})
		}

		networks = append(networks, transportNetworkModel{
			ID:         items[i].Id,
			Name:       items[i].Name,
			Location:   items[i].Location,
			Capacities: capacities,
		})
	}

	// Sort networks deterministically so repeated reads produce a stable ordering.
	common.SortByKeys(networks,
		func(n transportNetworkModel) string { return n.Name },
		func(n transportNetworkModel) string { return n.ID },
	)

	return networks
}

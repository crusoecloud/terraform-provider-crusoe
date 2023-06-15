package ib_network

import (
	"context"
	swagger "github.com/crusoecloud/client-go/swagger/v1alpha4"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/crusoecloud/terraform-provider-crusoe/internal"
)

type ibNetworksDataSource struct {
	client *swagger.APIClient
}

type ibNetworksDataSourceModel struct {
	IBNetworks []ibNetworkModel `tfsdk:"ib_networks"`
}

type ibNetworkModel struct {
	ID           string `tfsdk:"id"`
	Name         string `tfsdk:"name"`
	Location     string `tfsdk:"location"`
	Type         string `tfsdk:"type"`
	Size         string `tfsdk:"size"`
	SerialNumber string `tfsdk:"serial_number"`
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
		resp.Diagnostics.AddError("Failed to initialize provider", internal.ErrorMsgProviderInitFailed)

		return
	}

	ds.client = client
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *ibNetworksDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_ib_networks"
}

//nolint:gocritic // Implements Terraform defined interface
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

//nolint:gocritic // Implements Terraform defined interface
func (ds *ibNetworksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	dataResp, httpResp, err := ds.client.DisksApi.GetDisks(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to Fetch Disks", "Could not fetch Disk data at this time.")

		return
	}
	defer httpResp.Body.Close()

	//var state ibNetworksDataSourceModel
	//for i := range dataResp.Disks {
	//state.Disks = append(state.Disks, diskModel{
	//	ID:           dataResp.Disks[i].Id,
	//	Name:         dataResp.Disks[i].Name,
	//	Location:     dataResp.Disks[i].Location,
	//	Type:         dataResp.Disks[i].Type_,
	//	Size:         dataResp.Disks[i].Size,
	//	SerialNumber: dataResp.Disks[i].SerialNumber,
	//})
	//}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

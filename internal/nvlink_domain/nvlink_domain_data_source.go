//nolint:gocritic // Implements Terraform defined interface
package nvlink_domain

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type NVLinkDomainsDataSource struct {
	client *common.CrusoeClient
}

type NVLinkDomainsDataSourceModel struct {
	NVLinkDomains []NVLinkDomainModel `tfsdk:"nvlink_domains"`
}

type NVLinkDomainModel struct {
	ID             string `tfsdk:"id"`
	Name           string `tfsdk:"name"`
	Location       string `tfsdk:"location"`
	TotalNodes     int64  `tfsdk:"total_nodes"`
	AvailableNodes int64  `tfsdk:"available_nodes"`
}

func NewNvlinkDomainsDataSource() datasource.DataSource {
	return &NVLinkDomainsDataSource{}
}

// Configure adds the provider configured client to the data source.
func (ds *NVLinkDomainsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (ds *NVLinkDomainsDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_nvlink_domains"
}

func (ds *NVLinkDomainsDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"nvlink_domains": schema.ListNestedAttribute{
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
					"total_nodes": schema.Int64Attribute{
						Computed: true,
					},
					"available_nodes": schema.Int64Attribute{
						Computed: true,
					},
				},
			},
		},
	}}
}

func (ds *NVLinkDomainsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	dataResp, httpResp, err := ds.client.APIClient.NVLinkDomainsApi.ListNvlinkDomains(ctx, ds.client.ProjectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch NVLink domains",
			fmt.Sprintf("Could not fetch NVLink domains data at this time: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	var state NVLinkDomainsDataSourceModel
	for i := range dataResp.NvlinkDomains {
		state.NVLinkDomains = append(state.NVLinkDomains, NVLinkDomainModel{
			ID:             dataResp.NvlinkDomains[i].Id,
			Name:           dataResp.NvlinkDomains[i].Name,
			Location:       dataResp.NvlinkDomains[i].Location,
			TotalNodes:     int64(dataResp.NvlinkDomains[i].TotalNodes),
			AvailableNodes: int64(dataResp.NvlinkDomains[i].AvailableNodes),
		})
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

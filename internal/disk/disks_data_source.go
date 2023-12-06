package disk

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type disksDataSource struct {
	client *swagger.APIClient
}

type disksDataSourceModel struct {
	Disks []diskModel `tfsdk:"disks"`
}

type disksDataSourceFilter struct {
	ProjectID *string `tfsdk:"project_id"`
}

type diskModel struct {
	ID           string `tfsdk:"id"`
	Name         string `tfsdk:"name"`
	Location     string `tfsdk:"location"`
	Type         string `tfsdk:"type"`
	Size         string `tfsdk:"size"`
	SerialNumber string `tfsdk:"serial_number"`
}

// TODO: let's also implement a singular DiskDataSource for fetching one disk with filtering
func NewDisksDataSource() datasource.DataSource {
	return &disksDataSource{}
}

// Configure adds the provider configured client to the data source.
func (ds *disksDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

//nolint:gocritic // Implements Terraform defined interface
func (ds *disksDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_storage_disks"
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *disksDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"disks": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed: true,
					},
					"name": schema.StringAttribute{
						Required: true,
					},
					"location": schema.StringAttribute{
						Required: true,
					},
					"type": schema.StringAttribute{
						Required: true,
					},
					"size": schema.StringAttribute{
						Required: true,
					},
					"serial_number": schema.StringAttribute{
						Computed: true,
					},
				},
			},
		},
	}}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *disksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	projectID, err := common.GetFallbackProject(ctx, ds.client, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch disks",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	dataResp, httpResp, err := ds.client.DisksApi.ListDisks(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to Fetch Disks", "Could not fetch Disk data at this time.")

		return
	}
	defer httpResp.Body.Close()

	var state disksDataSourceModel
	for i := range dataResp.Items {
		state.Disks = append(state.Disks, diskModel{
			ID:           dataResp.Items[i].Id,
			Name:         dataResp.Items[i].Name,
			Location:     dataResp.Items[i].Location,
			Type:         dataResp.Items[i].Type_,
			Size:         dataResp.Items[i].Size,
			SerialNumber: dataResp.Items[i].SerialNumber,
		})
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

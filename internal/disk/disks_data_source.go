package disk

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type disksDataSource struct {
	client *common.CrusoeClient
}

type disksDataSourceModel struct {
	ProjectID types.String `tfsdk:"project_id"`
	Disks     []diskModel  `tfsdk:"disks"`
}

type diskModel struct {
	ID           string `tfsdk:"id"`
	Name         string `tfsdk:"name"`
	Location     string `tfsdk:"location"`
	Type         string `tfsdk:"type"`
	Size         string `tfsdk:"size"`
	SerialNumber string `tfsdk:"serial_number"`
	BlockSize    int64  `tfsdk:"block_size"`
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

	client, ok := req.ProviderData.(*common.CrusoeClient)
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
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Optional: true,
			},
			"disks": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of disks in the project.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descID,
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descName,
						},
						"location": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descLocation,
						},
						"type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descType,
						},
						"size": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descSize,
						},
						"serial_number": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descSerialNumber,
						},
						"block_size": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: descBlockSize,
						},
					},
				},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *disksDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config disksDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(ds.client, config.ProjectID.ValueString())

	dataResp, httpResp, err := ds.client.APIClient.DisksApi.ListDisks(ctx, projectID, &swagger.DisksApiListDisksOpts{})
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to Fetch Disks", "Could not fetch Disk data at this time.")

		return
	}

	var state disksDataSourceModel
	for i := range dataResp.Items {
		state.Disks = append(state.Disks, diskModel{
			ID:           dataResp.Items[i].Id,
			Name:         dataResp.Items[i].Name,
			Location:     dataResp.Items[i].Location,
			Type:         dataResp.Items[i].Type_,
			Size:         dataResp.Items[i].Size,
			SerialNumber: dataResp.Items[i].SerialNumber,
			BlockSize:    dataResp.Items[i].BlockSize,
		})
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

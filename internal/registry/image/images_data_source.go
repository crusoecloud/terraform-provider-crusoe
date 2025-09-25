package image

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type imagesDataSource struct {
	client *swagger.APIClient
}

type imagesDataSourceModel struct {
	Images    []imageDataSourceModel `tfsdk:"images"`
	ProjectID types.String           `tfsdk:"project_id"`
	RepoName  types.String           `tfsdk:"repo_name"`
	Location  types.String           `tfsdk:"location"`
}

type imageDataSourceModel struct {
	ManifestCount types.Int64  `tfsdk:"manifest_count"`
	Name          types.String `tfsdk:"name"`
	PullCount     types.Int64  `tfsdk:"pull_count"`
	Url           types.String `tfsdk:"url"`
}

func NewRegistryImagesDataSource() datasource.DataSource {
	return &imagesDataSource{}
}

func (i *imagesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*swagger.APIClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected ProviderData type",
			fmt.Sprintf("Expected *swagger.APIClient, got: %T", req.ProviderData),
		)

		return
	}
	i.client = client
}

//nolint:gocritic // Implements Terraform defined interface
func (i *imagesDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_registry_images"
}

//nolint:gocritic // Implements Terraform defined interface
func (i *imagesDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Optional: true,
			},
			"repo_name": schema.StringAttribute{
				Required: true,
			},
			"location": schema.StringAttribute{
				Required: true,
			},
			"images": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"manifest_count": schema.Int64Attribute{
							Computed: true,
						},
						"name": schema.StringAttribute{
							Computed: true,
						},
						"pull_count": schema.Int64Attribute{
							Computed: true,
						},
						"url": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (i *imagesDataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	var state imagesDataSourceModel
	diags := request.Config.Get(ctx, &state)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}
	projectID, err := common.GetProjectIDOrFallback(ctx, i.client, &response.Diagnostics, state.ProjectID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Failed to fetch project ID", fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}
	images, httpResp, err := i.client.CcrApi.ListCcrImages(ctx, projectID, state.RepoName.ValueString(), state.Location.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Failed to list images", fmt.Sprintf("Error listing images: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()
	for _, image := range images.Items {
		state.Images = append(state.Images, imageDataSourceModel{
			ManifestCount: types.Int64Value(image.ManifestCount),
			Name:          types.StringValue(image.Name),
			PullCount:     types.Int64Value(image.PullCount),
			Url:           types.StringValue(image.Url),
		})
	}

	diags = response.State.Set(ctx, &state)
	response.Diagnostics.Append(diags...)
}

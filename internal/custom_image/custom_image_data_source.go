package custom_image

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type customImageDataSource struct {
	client *common.CrusoeClient
}

type customImageDataSourceModel struct {
	ProjectID    types.String       `tfsdk:"project_id"`
	Name         *string            `tfsdk:"name"`
	NamePrefix   *string            `tfsdk:"name_prefix"`
	CustomImages []customImageModel `tfsdk:"custom_images"`
	NewestImage  *customImageModel  `tfsdk:"newest_image"`
}

type customImageModel struct {
	ID          string   `tfsdk:"id" json:"id"`
	Name        string   `tfsdk:"name" json:"name"`
	Description string   `tfsdk:"description" json:"description"`
	Locations   []string `tfsdk:"locations" json:"locations"`
	Tags        []string `tfsdk:"tags" json:"tags"`
	CreatedAt   string   `tfsdk:"created_at" json:"created_at"`
}

func NewCustomImageDataSource() datasource.DataSource {
	return &customImageDataSource{}
}

// Configure adds the provider configured client to the data source.
func (ds *customImageDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (ds *customImageDataSource) Metadata(_ context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_compute_custom_image"
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *customImageDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"project_id": schema.StringAttribute{
			Optional:    true,
			Description: providerDescProjectID,
		},
		"name": schema.StringAttribute{
			Optional:    true,
			Description: providerDescNameFilter,
		},
		"name_prefix": schema.StringAttribute{
			Optional:    true,
			Description: providerDescNamePrefixFilter,
		},
		"custom_images": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id":          schema.StringAttribute{Computed: true, Description: apiDescID},
					"name":        schema.StringAttribute{Computed: true, Description: apiDescName},
					"description": schema.StringAttribute{Computed: true, Description: apiDescDescription},
					"locations":   schema.ListAttribute{ElementType: types.StringType, Computed: true, Description: apiDescLocations},
					"tags":        schema.ListAttribute{ElementType: types.StringType, Computed: true, Description: apiDescTags},
					"created_at":  schema.StringAttribute{Computed: true, Description: apiDescCreatedAt},
				},
			},
		},
		"newest_image": schema.SingleNestedAttribute{
			Computed: true,
			Attributes: map[string]schema.Attribute{
				"id":          schema.StringAttribute{Computed: true, Description: apiDescID},
				"name":        schema.StringAttribute{Computed: true, Description: apiDescName},
				"description": schema.StringAttribute{Computed: true, Description: apiDescDescription},
				"locations":   schema.ListAttribute{ElementType: types.StringType, Computed: true, Description: apiDescLocations},
				"tags":        schema.ListAttribute{ElementType: types.StringType, Computed: true, Description: apiDescTags},
				"created_at":  schema.StringAttribute{Computed: true, Description: apiDescCreatedAt},
			},
		},
	}}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *customImageDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config customImageDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(ds.client, config.ProjectID.ValueString())

	apiResp, httpResp, err := ds.client.APIClient.CustomImagesApi.ListCustomImages(ctx, projectID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to fetch custom images",
			fmt.Sprintf("Could not fetch custom images: %s", common.UnpackAPIError(err)),
		)

		return
	}

	filteredImages := filterCustomImagesListResponse(&apiResp, config)

	// Sort images deterministically so repeated reads produce a stable ordering.
	// newest_image is computed independently below via its own copy-sort.
	common.SortByKeys(filteredImages,
		func(img customImageModel) string { return img.Name },
		func(img customImageModel) string { return img.CreatedAt },
		func(img customImageModel) string { return img.ID },
	)

	var state customImageDataSourceModel
	state.Name = config.Name
	state.NamePrefix = config.NamePrefix
	state.CustomImages = filteredImages
	state.NewestImage = findNewestImage(filteredImages)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

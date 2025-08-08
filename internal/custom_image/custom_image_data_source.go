package custom_image

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type customImageDataSource struct {
	client *swagger.APIClient
}

type customImageDataSourceModel struct {
	ProjectID    *string            `tfsdk:"project_id"`
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
	client, ok := req.ProviderData.(*swagger.APIClient)
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
			Optional: true,
		},
		"name": schema.StringAttribute{
			Optional:    true,
			Description: "Filter custom images by name. This is a case-sensitive exact match.",
		},
		"name_prefix": schema.StringAttribute{
			Optional:    true,
			Description: "Filter custom images by name prefix. This is case-sensitive and does not require trailing dashes.",
		},
		"custom_images": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id":          schema.StringAttribute{Computed: true},
					"name":        schema.StringAttribute{Computed: true},
					"description": schema.StringAttribute{Computed: true},
					"locations":   schema.ListAttribute{ElementType: types.StringType, Computed: true},
					"tags":        schema.ListAttribute{ElementType: types.StringType, Computed: true},
					"created_at":  schema.StringAttribute{Computed: true},
				},
			},
		},
		"newest_image": schema.SingleNestedAttribute{
			Computed: true,
			Attributes: map[string]schema.Attribute{
				"id":          schema.StringAttribute{Computed: true},
				"name":        schema.StringAttribute{Computed: true},
				"description": schema.StringAttribute{Computed: true},
				"locations":   schema.ListAttribute{ElementType: types.StringType, Computed: true},
				"tags":        schema.ListAttribute{ElementType: types.StringType, Computed: true},
				"created_at":  schema.StringAttribute{Computed: true},
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

	var projectID string
	if config.ProjectID != nil {
		projectID = *config.ProjectID
	} else {
		fallbackProjectID, err := common.GetFallbackProject(ctx, ds.client, &resp.Diagnostics)
		if err != nil {
			resp.Diagnostics.AddError("Failed to fetch custom images",
				fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

			return
		}
		projectID = fallbackProjectID
	}

	apiResp, httpResp, err := ds.client.CustomImagesApi.ListCustomImages(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch custom images", err.Error())

		return
	}
	defer httpResp.Body.Close()

	filteredImages := filterCustomImagesListResponse(&apiResp, config)

	var state customImageDataSourceModel
	state.Name = config.Name
	state.NamePrefix = config.NamePrefix
	state.CustomImages = filteredImages
	state.NewestImage = findNewestImage(filteredImages)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

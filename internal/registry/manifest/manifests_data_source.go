package manifest

import (
	"context"
	"fmt"
	"github.com/antihax/optional"
	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type manifestsDataSource struct {
	client *swagger.APIClient
}

type manifestsDataSourceModel struct {
	Manifests   []manifestDataSourceModel `tfsdk:"manifests"`
	ProjectID   types.String              `tfsdk:"project_id"`
	RepoName    types.String              `tfsdk:"repo_name"`
	ImageName   types.String              `tfsdk:"image_name"`
	Location    types.String              `tfsdk:"location"`
	TagContains types.String              `tfsdk:"tag_contains"`
}

type manifestDataSourceModel struct {
	Digest types.String   `tfsdk:"digest"`
	Size   types.String   `tfsdk:"size"`
	Tags   []types.String `tfsdk:"tags"`
}

func NewRegistryManifestsDataSource() datasource.DataSource {
	return &manifestsDataSource{}
}

func (m *manifestsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	m.client = client
}

func (m *manifestsDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_registry_manifests"
}

func (m *manifestsDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Optional: true,
			},
			"repo_name": schema.StringAttribute{
				Required: true,
			},
			"image_name": schema.StringAttribute{
				Required: true,
			},
			"location": schema.StringAttribute{
				Required: true,
			},
			"tag_contains": schema.StringAttribute{
				Optional: true,
			},
			"manifests": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"digest": schema.StringAttribute{
							Computed: true,
						},
						"size": schema.StringAttribute{
							Computed: true,
						},
						"tags": schema.ListAttribute{
							Computed:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
		},
	}
}

func (m *manifestsDataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	var state manifestsDataSourceModel
	diags := request.Config.Get(ctx, &state)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	projectID, err := common.GetProjectIDOrFallback(ctx, m.client, &response.Diagnostics, state.ProjectID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Failed to fetch project ID", fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))
		return
	}
	opts := &swagger.CcrApiListCcrManifestsOpts{}
	if !state.TagContains.IsNull() {
		tagSearchQuery := state.TagContains.ValueString()
		opts.TagContains = optional.NewString(tagSearchQuery)
	}

	manifests, httpResp, err := m.client.CcrApi.ListCcrManifests(ctx, projectID, state.RepoName.ValueString(), state.ImageName.ValueString(), state.Location.ValueString(), opts)
	if err != nil {
		response.Diagnostics.AddError("Failed to list manifests", fmt.Sprintf("Error listing manifests: %s", common.UnpackAPIError(err)))
		return
	}
	defer httpResp.Body.Close()

	for _, manifest := range manifests.Items {
		// Convert tags slice to types.String slice
		var tags []types.String
		for _, tag := range manifest.Tags {
			tags = append(tags, types.StringValue(tag))
		}

		state.Manifests = append(state.Manifests, manifestDataSourceModel{
			Digest: types.StringValue(manifest.Digest),
			Size:   types.StringValue(manifest.SizeBytes),
			Tags:   tags,
		})
	}

	diags = response.State.Set(ctx, &state)
	response.Diagnostics.Append(diags...)
}

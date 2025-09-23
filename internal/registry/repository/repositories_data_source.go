package repository

import (
	"context"
	"fmt"
	"github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type registryRepositoriesDataSource struct {
	client *swagger.APIClient
}

type registryRepositoriesDataSourceModel struct {
	Repositories []registryRepositoryDataSourceModel `tfsdk:"repositories"`
	ProjectID    types.String                        `tfsdk:"project_id"`
}

type registryRepositoryDataSourceModel struct {
	Name     types.String `tfsdk:"name"`
	Location types.String `tfsdk:"location"`
	Mode     types.String `tfsdk:"mode"`
	State    types.String `tfsdk:"state"`
	URL      types.String `tfsdk:"url"`
}

func NewRegistryRepositoriesDataSource() datasource.DataSource {
	return &registryRepositoriesDataSource{}
}

func (d *registryRepositoriesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.client = client
}

func (d *registryRepositoriesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_registry_repositories"
}

func (d *registryRepositoriesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Optional: true,
			},
			"repositories": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":     schema.StringAttribute{Computed: true},
						"location": schema.StringAttribute{Computed: true},
						"mode":     schema.StringAttribute{Computed: true},
						"state":    schema.StringAttribute{Computed: true},
						"url":      schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *registryRepositoriesDataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	var state registryRepositoriesDataSourceModel
	diags := request.Config.Get(ctx, &state)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}
	projectID, err := common.GetProjectIDOrFallback(ctx, d.client, &response.Diagnostics, state.ProjectID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Failed to fetch project ID", fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	repos, httpResp, err := d.client.CcrApi.ListCcrRepositories(ctx, projectID)
	if err != nil {
		response.Diagnostics.AddError("Failed to list repositories", fmt.Sprintf("Error listing repositories: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	for _, repo := range repos.Items {
		state.Repositories = append(state.Repositories, registryRepositoryDataSourceModel{
			Name:     types.StringValue(repo.Name),
			Location: types.StringValue(repo.Location),
			Mode:     types.StringValue(repo.Mode),
			State:    types.StringValue(repo.State),
			URL:      types.StringValue(repo.Url),
		})
	}

	diags = response.State.Set(ctx, &state)
	response.Diagnostics.Append(diags...)
}

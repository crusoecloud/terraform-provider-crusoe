package project

import (
	"context"
	"fmt"

	"github.com/antihax/optional"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type projectsDataSource struct {
	client *swagger.APIClient
}

type projectsDataSourceModel struct {
	Projects []projectModel `tfsdk:"projects"`
}

type projectModel struct {
	ID   string `tfsdk:"id"`
	Name string `tfsdk:"name"`
}

func NewProjectsDataSource() datasource.DataSource {
	return &projectsDataSource{}
}

// Configure adds the provider configured client to the data source.
func (ds *projectsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (ds *projectsDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_projects"
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *projectsDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"projects": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed: true,
					},
					"name": schema.StringAttribute{
						Computed: true,
					},
				},
			},
		},
	}}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *projectsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	opts := &swagger.ProjectsApiListProjectsOpts{
		OrgId: optional.EmptyString(),
	}

	dataResp, httpResp, err := ds.client.ProjectsApi.ListProjects(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Failed to Fetch Projects",
			fmt.Sprintf("Could not fetch Project data at this time: %v.", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	var state projectsDataSourceModel
	for i := range dataResp.Items {
		state.Projects = append(state.Projects, projectModel{
			ID:   dataResp.Items[i].Id,
			Name: dataResp.Items[i].Name,
		})
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

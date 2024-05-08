package instance_group

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type instanceGroupsDataSource struct {
	client *swagger.APIClient
}

type instanceGroupsDataSourceModel struct {
	ProjectID      *string               `tfsdk:"project_id"`
	InstanceGroups []instanceGroupsModel `tfsdk:"instance_groups"`
}

type instanceGroupsModel struct {
	ID                   string   `tfsdk:"id"`
	Name                 string   `tfsdk:"name"`
	InstanceTemplate     string   `tfsdk:"instance_template"`
	RunningInstanceCount int64    `tfsdk:"running_instance_count"`
	Instances            []string `tfsdk:"instances"`
}

func NewInstanceGroupsDataSource() datasource.DataSource {
	return &instanceGroupsDataSource{}
}

// Configure adds the provider configured client to the data source.
func (ds *instanceGroupsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (ds *instanceGroupsDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_compute_instance_groups"
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *instanceGroupsDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"instance_groups": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed: true,
					},
					"name": schema.StringAttribute{
						Required: true,
					},
					"cidr": schema.StringAttribute{
						Required: true,
					},
					"location": schema.StringAttribute{
						Computed: true,
					},
					"network": schema.StringAttribute{
						Computed: true,
					},
				},
			},
		},
		"project_id": schema.StringAttribute{
			Optional: true,
		},
	}}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *instanceGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config instanceGroupsDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectID := ""
	if config.ProjectID != nil {
		projectID = *config.ProjectID
	} else {
		fallbackProjectID, err := common.GetFallbackProject(ctx, ds.client, &resp.Diagnostics)
		if err != nil {
			resp.Diagnostics.AddError("Failed to fetch Instance Groups",
				fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

			return
		}
		projectID = fallbackProjectID
	}

	dataResp, httpResp, err := ds.client.InstanceGroupsApi.ListInstanceGroups(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to Fetch Instance Groups", "Could not fetch Instance Group data at this time.")

		return
	}
	defer httpResp.Body.Close()

	var state instanceGroupsDataSourceModel
	for i := range dataResp.Items {
		state.InstanceGroups = append(state.InstanceGroups, instanceGroupsModel{
			ID:                   dataResp.Items[i].Id,
			Name:                 dataResp.Items[i].Name,
			InstanceTemplate:     dataResp.Items[i].TemplateId,
			RunningInstanceCount: dataResp.Items[i].RunningInstanceCount,
			Instances:            dataResp.Items[i].Instances,
		})
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

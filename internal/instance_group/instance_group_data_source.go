package instance_group

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type instanceGroupsDataSource struct {
	client *common.CrusoeClient
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

	client, ok := req.ProviderData.(*common.CrusoeClient)
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
	response.Schema = schema.Schema{
		MarkdownDescription: common.DevelopmentMessage,
		Attributes: map[string]schema.Attribute{
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
						"instance_template": schema.StringAttribute{
							Required: true,
						},
						"running_instance_count": schema.Int64Attribute{
							Computed: true,
						},
						"instances": schema.ListAttribute{
							ElementType: types.StringType,
							Computed:    true,
						},
					},
				},
			},
			"project_id": schema.StringAttribute{
				Optional: true,
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *instanceGroupsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config instanceGroupsDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDFromPointerOrFallback(ds.client, config.ProjectID)

	dataResp, httpResp, err := ds.client.APIClient.InstanceGroupsApi.ListInstanceGroups(ctx, projectID)
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

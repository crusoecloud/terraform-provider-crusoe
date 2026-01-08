package instance_group

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type instanceGroupsDataSource struct {
	client *common.CrusoeClient
}

type instanceGroupsDataSourceModel struct {
	ProjectID      *string                        `tfsdk:"project_id"`
	InstanceGroups []instanceGroupDataSourceModel `tfsdk:"instance_groups"`
}

type instanceGroupDataSourceModel struct {
	ID                   string   `tfsdk:"id"`
	Name                 string   `tfsdk:"name"`
	InstanceTemplateID   string   `tfsdk:"instance_template_id"`
	RunningInstanceCount int64    `tfsdk:"running_instance_count"`
	DesiredCount         int64    `tfsdk:"desired_count"`
	State                string   `tfsdk:"state"`
	ProjectID            string   `tfsdk:"project_id"`
	ActiveInstanceIDs    []string `tfsdk:"active_instance_ids"`
	InactiveInstanceIDs  []string `tfsdk:"inactive_instance_ids"`
	CreatedAt            string   `tfsdk:"created_at"`
	UpdatedAt            string   `tfsdk:"updated_at"`
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
		MarkdownDescription: common.DevelopmentMessage + "\n\nFetches a list of instance groups within a project.",
		Attributes: map[string]schema.Attribute{
			"instance_groups": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of instance groups in the project.",
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
						"instance_template_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descInstanceTemplateID,
						},
						"running_instance_count": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: descRunningInstanceCount,
						},
						"desired_count": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: descDesiredCount,
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descState,
						},
						"project_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descProjectID,
						},
						"active_instance_ids": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: descActiveInstanceIDs,
						},
						"inactive_instance_ids": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: descInactiveInstanceIDs,
						},
						"created_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descCreatedAt,
						},
						"updated_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descUpdatedAt,
						},
					},
				},
			},
			"project_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: descProjectID + " " + descProjectIDInference,
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
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to Fetch Instance Groups",
			fmt.Sprintf("Could not fetch instance groups: %s", common.UnpackAPIError(err)),
		)

		return
	}

	if !common.ValidateHTTPStatus(&resp.Diagnostics, httpResp, "fetch Instance Groups", http.StatusOK) {
		return
	}

	var state instanceGroupsDataSourceModel
	state.ProjectID = &projectID
	state.InstanceGroups = make([]instanceGroupDataSourceModel, 0, len(dataResp.Items))

	for i := range dataResp.Items {
		state.InstanceGroups = append(state.InstanceGroups, instanceGroupToDataSourceModel(&dataResp.Items[i]))
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

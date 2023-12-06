package project

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type projectResource struct {
	client *swagger.APIClient
}

type projectResourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func NewProjectResource() resource.Resource {
	return &projectResource{}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *projectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*swagger.APIClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", common.ErrorMsgProviderInitFailed)

		return
	}

	r.client = client
}

//nolint:gocritic // Implements Terraform defined interface
func (r *projectResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

//nolint:gocritic // Implements Terraform defined interface
func (r *projectResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"name": schema.StringAttribute{
				Required: true,
			},
		},
	}
}

func (r *projectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *projectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgID, err := getUserOrg(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create project",
			fmt.Sprintf("There was an error fetching the user's organization: %s", err))

		return
	}
	dataResp, httpResp, err := r.client.ProjectsApi.CreateProject(ctx, swagger.ProjectsPostRequest{
		Name:           plan.Name.ValueString(),
		OrganizationId: orgID,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create project",
			fmt.Sprintf("There was an error starting a create project operation: %s.", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	project := dataResp.Project

	plan.ID = types.StringValue(project.Id)
	plan.Name = types.StringValue(project.Name)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *projectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}


	project, httpResp, err := r.client.ProjectsApi.GetProject(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get projects",
			fmt.Sprintf("Fetching Crusoe projects failed: %s\n\nIf the problem persists, contact support@crusoecloud.com", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	state.Name = types.StringValue(project.Name)
	state.ID = types.StringValue(project.Id)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *projectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state projectResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan projectResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, httpResp, err := r.client.ProjectsApi.UpdateProject(ctx,
		swagger.ProjectsPutRequest{Name: plan.Name.ValueString()},
		plan.ID.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update project",
			fmt.Sprintf("There was an error starting an update project operation: %s.", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *projectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {

	resp.Diagnostics.AddWarning("Delete not supported",
		"Deleting projects is not currently supported. If you're seeing this message, please reach"+
			" out to support@crusoecloud.com and let us know.")
}

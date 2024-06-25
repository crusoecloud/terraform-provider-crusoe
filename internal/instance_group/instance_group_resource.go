package instance_group

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	validators "github.com/crusoecloud/terraform-provider-crusoe/internal/validators"
)

type instanceGroupResource struct {
	client *swagger.APIClient
}

type instanceGroupResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	TemplateID           types.String `tfsdk:"instance_template"`
	RunningInstanceCount types.Int64  `tfsdk:"running_instance_count"`
	InstanceNamePrefix   types.String `tfsdk:"instance_name_prefix"`
	Instances            types.List   `tfsdk:"instances"`
	ProjectID            types.String `tfsdk:"project_id"`
}

func NewInstanceGroupResource() resource.Resource {
	return &instanceGroupResource{}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *instanceGroupResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compute_instance_group"
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: common.DevelopmentMessage,
		Version:             0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"project_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"instance_name_prefix": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"instance_template": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"running_instance_count": schema.Int64Attribute{
				Required: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Validators: []validator.Int64{
					validators.RunningInstanceCountValidator{},
				},
			},
			"instances": schema.ListAttribute{
				ElementType:   types.StringType,
				Computed:      true,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
		},
	}
}

func (r *instanceGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan instanceGroupResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := ""
	if plan.ProjectID.ValueString() == "" {
		project, err := common.GetFallbackProject(ctx, r.client, &resp.Diagnostics)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create Instance Group",
				fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

			return
		}
		projectID = project
	} else {
		projectID = plan.ProjectID.ValueString()
	}

	dataResp, httpResp, err := r.client.InstanceGroupsApi.CreateInstanceGroup(ctx, swagger.InstanceGroupPostRequest{
		Name:       plan.Name.ValueString(),
		TemplateId: plan.TemplateID.ValueString(),
	}, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Instance Group",
			fmt.Sprintf("There was an error starting a create Instance Group operation (%s): %s", projectID, common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	plan.Name = types.StringValue(dataResp.Name)
	plan.ID = types.StringValue(dataResp.Id)
	plan.TemplateID = types.StringValue(dataResp.TemplateId)
	plan.RunningInstanceCount = types.Int64Value(dataResp.RunningInstanceCount)
	plan.ProjectID = types.StringValue(projectID)
	instances, _ := types.ListValueFrom(context.Background(), types.StringType, dataResp.Instances)
	plan.Instances = instances

	// if user specifies that they want a non-zero number of instances
	numInstances := plan.RunningInstanceCount.ValueInt64()
	if numInstances > 0 {
		addErr := addInstancesToGroup(ctx, r.client, plan.InstanceNamePrefix.ValueString(), dataResp.Id,
			plan.TemplateID.ValueString(), projectID, numInstances)
		if addErr != nil {
			resp.Diagnostics.AddError("Failed to add instances to Instance Group",
				fmt.Sprintf("There was an error adding instances to the Instance Group: %s.\n\n", common.UnpackAPIError(addErr)))

			return
		}
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state instanceGroupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	instanceGroup, httpResp, err := r.client.InstanceGroupsApi.GetInstanceGroup(ctx, state.ID.ValueString(), state.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Instance Group",
			fmt.Sprintf("Fetching Crusoe Instance Groups failed: %s\n\nIf the problem persists, contact support@crusoecloud.com", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		// Instance Group has most likely been deleted out of band, so we update Terraform state to match
		resp.State.RemoveResource(ctx)

		return
	}

	instanceGroupToTerraformResourceModel(&instanceGroup, &state)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state instanceGroupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan instanceGroupResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dataResp, httpResp, err := r.client.InstanceGroupsApi.PatchInstanceGroup(ctx,
		swagger.InstanceGroupPatchRequest{
			Name:       plan.Name.ValueString(),
			TemplateId: plan.TemplateID.ValueString(),
		},
		plan.ProjectID.ValueString(),
		plan.ID.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update Instance Group",
			fmt.Sprintf("There was an error updating the Instance Group: %s.\n\n", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	// if user specifies that they want a non-zero number of instances
	numInstances := plan.RunningInstanceCount.ValueInt64()
	// add instances
	if numInstances > dataResp.RunningInstanceCount {
		addErr := addInstancesToGroup(ctx, r.client, plan.InstanceNamePrefix.ValueString(), dataResp.Id,
			plan.TemplateID.ValueString(), plan.ProjectID.ValueString(), numInstances)
		if addErr != nil {
			resp.Diagnostics.AddError("Failed to add instances to Instance Group",
				fmt.Sprintf("There was an error adding instances to the Instance Group: %s.\n\n", common.UnpackAPIError(addErr)))

			return
		}
	} else if numInstances < dataResp.RunningInstanceCount {
		newInstances, removeErr := removeInstancesFromGroup(ctx, r.client, plan.ProjectID.ValueString(), numInstances, dataResp.Instances)
		if removeErr != nil {
			resp.Diagnostics.AddError("Failed to remove instances from Instance Group",
				fmt.Sprintf("There was an error removing instances from the Instance Group: %s.\n\n", common.UnpackAPIError(removeErr)))

			return
		}

		newInstancesInGroup, _ := types.ListValueFrom(context.Background(), types.StringType, newInstances)
		state.Instances = newInstancesInGroup
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state instanceGroupResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	instanceGroup, getHttpResp, err := r.client.InstanceGroupsApi.GetInstanceGroup(ctx, state.ID.ValueString(), state.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete Instance Group",
			fmt.Sprintf("Fetching Crusoe Instance Groups failed: %s\n\nIf the problem persists, contact support@crusoecloud.com", common.UnpackAPIError(err)))

		return
	}
	defer getHttpResp.Body.Close()

	_, removeErr := removeInstancesFromGroup(ctx, r.client, state.ProjectID.ValueString(), 0, instanceGroup.Instances)
	if removeErr != nil {
		resp.Diagnostics.AddError("Failed to delete Instance Group",
			fmt.Sprintf("There was an error removing instances from the Instance Group: %s.\n\n", common.UnpackAPIError(removeErr)))

		return
	}

	httpResp, err := r.client.InstanceGroupsApi.DeleteInstanceGroup(ctx, state.ID.ValueString(), state.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete Instance Group",
			fmt.Sprintf("There was an error deleting the Instance Group: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()
}

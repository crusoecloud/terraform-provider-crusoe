package instance_group

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
)

type instanceGroupResource struct {
	client *common.CrusoeClient
}

type instanceGroupResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	InstanceTemplateID   types.String `tfsdk:"instance_template_id"`
	RunningInstanceCount types.Int64  `tfsdk:"running_instance_count"`
	ActiveInstanceIDs    types.List   `tfsdk:"active_instance_ids"`
	InactiveInstanceIDs  types.List   `tfsdk:"inactive_instance_ids"`
	ProjectID            types.String `tfsdk:"project_id"`
	DesiredCount         types.Int64  `tfsdk:"desired_count"`
	State                types.String `tfsdk:"state"`
	CreatedAt            types.String `tfsdk:"created_at"`
	UpdatedAt            types.String `tfsdk:"updated_at"`
}

func NewInstanceGroupResource() resource.Resource {
	return &instanceGroupResource{}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*common.CrusoeClient)
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
		MarkdownDescription: common.DevelopmentMessage + "\n\nManages a Crusoe compute instance group resource.",
		Version:             1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: descID,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"project_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: descProjectID + " " + descProjectIDInference,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: descName,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"instance_template_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: descInstanceTemplateID,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"running_instance_count": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: descRunningInstanceCount,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"active_instance_ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: descActiveInstanceIDs,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"inactive_instance_ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: descInactiveInstanceIDs,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: descState,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"desired_count": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: descDesiredCount,
				Validators: []validator.Int64{
					int64validator.AtLeast(0),
				},
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: descCreatedAt,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: descUpdatedAt,
			},
		},
	}
}

func (r *instanceGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceID, projectID, errMsg := common.ParseResourceIdentifiers(req, r.client, "instance_group_id")
	if errMsg != "" {
		resp.Diagnostics.AddError("Failed to import Instance Group", errMsg)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), resourceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan instanceGroupResourceModel
	if err := getResourceModel(ctx, req.Plan, &plan, &resp.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())

	dataResp, httpResp, err := r.client.APIClient.InstanceGroupsApi.CreateInstanceGroup(ctx, swagger.InstanceGroupPostRequest{
		Name:         plan.Name.ValueString(),
		TemplateId:   plan.InstanceTemplateID.ValueString(),
		DesiredCount: plan.DesiredCount.ValueInt64(),
	}, projectID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to create Instance Group",
			fmt.Sprintf("There was an error starting a create instance group operation (%s): %s", projectID, common.UnpackAPIError(err)),
		)

		return
	}

	if !common.ValidateHTTPStatus(&resp.Diagnostics, httpResp, "create Instance Group", http.StatusOK, http.StatusCreated) {
		return
	}

	var state instanceGroupResourceModel
	instanceGroupToResourceModel(&dataResp, &state, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state instanceGroupResourceModel
	if err := getResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	instanceGroup, httpResp, err := r.client.APIClient.InstanceGroupsApi.GetInstanceGroup(ctx, state.ID.ValueString(), state.ProjectID.ValueString())
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to get Instance Group",
			fmt.Sprintf("Fetching Crusoe instance group failed: %s", common.UnpackAPIError(err)),
		)

		return
	}

	if httpResp.StatusCode == http.StatusNotFound {
		// Instance Group has most likely been deleted out of band, so we update Terraform state to match
		resp.State.RemoveResource(ctx)

		return
	}

	instanceGroupToResourceModel(&instanceGroup, &state, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state instanceGroupResourceModel
	if err := getResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	var plan instanceGroupResourceModel
	if err := getResourceModel(ctx, req.Plan, &plan, &resp.Diagnostics); err != nil {
		return
	}

	desiredCount := swagger.DesiredCount{
		Value: plan.DesiredCount.ValueInt64(),
	}

	dataResp, httpResp, err := r.client.APIClient.InstanceGroupsApi.PatchInstanceGroup(ctx,
		swagger.InstanceGroupPatchRequest{
			Name:         plan.Name.ValueString(),
			TemplateId:   plan.InstanceTemplateID.ValueString(),
			DesiredCount: &desiredCount,
		},
		plan.ID.ValueString(),
		plan.ProjectID.ValueString(),
	)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to update Instance Group",
			fmt.Sprintf("There was an error updating the instance group: %s", common.UnpackAPIError(err)),
		)

		return
	}

	if !common.ValidateHTTPStatus(&resp.Diagnostics, httpResp, "update Instance Group", http.StatusOK) {
		return
	}

	instanceGroupToResourceModel(&dataResp, &state, &resp.Diagnostics)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state instanceGroupResourceModel
	if err := getResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	projectID := state.ProjectID.ValueString()
	instanceGroupID := state.ID.ValueString()

	// Step 1: Update instance group desired_count to 0 to prevent new VMs from starting
	desiredCount := swagger.DesiredCount{Value: 0}
	_, patchHttpResp, err := r.client.APIClient.InstanceGroupsApi.PatchInstanceGroup(ctx,
		swagger.InstanceGroupPatchRequest{
			DesiredCount: &desiredCount,
		},
		instanceGroupID,
		projectID,
	)
	if patchHttpResp != nil {
		defer patchHttpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to update Instance Group before deletion",
			fmt.Sprintf("Could not set desired_count to 0: %s", common.UnpackAPIError(err)),
		)

		return
	}

	if !common.ValidateHTTPStatus(&resp.Diagnostics, patchHttpResp, "update Instance Group desired_count to 0", http.StatusOK) {
		return
	}

	// Step 2: Get current instance group state to retrieve active instance IDs
	instanceGroup, getHttpResp, err := r.client.APIClient.InstanceGroupsApi.GetInstanceGroup(ctx, instanceGroupID, projectID)
	if getHttpResp != nil {
		defer getHttpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to get Instance Group",
			fmt.Sprintf("Could not retrieve instance group for deletion: %s", common.UnpackAPIError(err)),
		)

		return
	}

	if !common.ValidateHTTPStatus(&resp.Diagnostics, getHttpResp, "get Instance Group for deletion", http.StatusOK) {
		return
	}

	// Step 3: Delete active instances
	instanceErrors := make(map[string]string) // instanceID -> error message
	for _, instanceID := range instanceGroup.ActiveInstances {
		delDataResp, delHttpResp, delErr := r.client.APIClient.VMsApi.DeleteInstance(ctx, projectID, instanceID)
		if delErr != nil {
			if delHttpResp != nil {
				delHttpResp.Body.Close()
			}
			instanceErrors[instanceID] = common.UnpackAPIError(delErr).Error()

			continue
		}

		// Wait for the delete operation to complete
		_, _, waitErr := common.AwaitOperationAndResolve[interface{}](ctx, delDataResp.Operation, projectID,
			r.client.APIClient.VMOperationsApi.GetComputeVMsInstancesOperation)
		if delHttpResp != nil {
			delHttpResp.Body.Close()
		}
		if waitErr != nil {
			instanceErrors[instanceID] = common.UnpackAPIError(waitErr).Error()

			continue
		}
	}

	if len(instanceErrors) > 0 {
		// Check if all errors are the same
		var firstErr string
		allSame := true
		var failedIDs []string
		for id, errMsg := range instanceErrors {
			failedIDs = append(failedIDs, id)
			if firstErr == "" {
				firstErr = errMsg
			} else if errMsg != firstErr {
				allSame = false

				break
			}
		}

		var errDetail string
		if allSame {
			errDetail = fmt.Sprintf("Could not delete instances %v: %s", failedIDs, firstErr)
		} else {
			errDetail = "Could not delete instances:"
			for id, errMsg := range instanceErrors {
				errDetail += fmt.Sprintf("\n  %s: %s", id, errMsg)
			}
		}

		resp.Diagnostics.AddError("Failed to delete instances", errDetail)

		return
	}

	// Step 4: Delete the instance group
	httpResp, err := r.client.APIClient.InstanceGroupsApi.DeleteInstanceGroup(ctx, instanceGroupID, projectID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to delete Instance Group",
			fmt.Sprintf("There was an error deleting the instance group: %s", common.UnpackAPIError(err)),
		)

		return
	}

	if !common.ValidateHTTPStatus(&resp.Diagnostics, httpResp, "delete Instance Group", http.StatusOK, http.StatusNoContent, http.StatusNotFound) {
		return
	}
}

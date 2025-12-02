//nolint:gocritic // Implements Terraform defined interface
package ib_partition

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

const notFoundMessage = "404 Not Found"

type ibPartitionResource struct {
	client *common.CrusoeClient
}

type ibPartitionResourceModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	IBNetworkID types.String `tfsdk:"ib_network_id"`
}

func NewIBPartitionResource() resource.Resource {
	return &ibPartitionResource{}
}

func (r *ibPartitionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ibPartitionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ib_partition"
}

func (r *ibPartitionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"ib_network_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"project_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				// cannot be updated in place
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *ibPartitionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ibPartitionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ibPartitionResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())

	dataResp, httpResp, err := r.client.APIClient.IBPartitionsApi.CreateIBPartition(ctx, swagger.IbPartitionsPostRequestV1Alpha5{
		Name:        plan.Name.ValueString(),
		IbNetworkId: plan.IBNetworkID.ValueString(),
	}, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create partition",
			fmt.Sprintf("There was an error creating an Infiniband partition: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	plan.ID = types.StringValue(dataResp.Id)
	plan.ProjectID = types.StringValue(projectID)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ibPartitionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ibPartitionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// We only have this parsing for transitioning from v1alpha4 to v1alpha5 because old tf state files will not
	// have project ID stored. So we will try to get a fallback project to pass to the API.
	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())

	partition, httpResp, err := r.client.APIClient.IBPartitionsApi.GetIBPartition(ctx, projectID, state.ID.ValueString())
	if err != nil {
		if err.Error() == notFoundMessage {
			// partition has most likely been deleted out of band, so we update Terraform state to match
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError("Failed to get IB partition",
			fmt.Sprintf("Fetching Crusoe Infiniband partition failed: %s\n\nIf the problem persists, contact support@crusoecloud.com", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	state.ProjectID = types.StringValue(projectID)
	ibPartitionToTerraformResourceModel(&partition, &state)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *ibPartitionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// This should be unreachable, since all properties are marked as needing replacement on update.
	resp.Diagnostics.AddWarning("In-place updates not supported",
		"Updating IB partitions in place is not currently supported. If you're seeing this message, please"+
			" reach out to support@crusoecloud.com and let us know. In the meantime, you should be able to update your"+
			" partition by deleting it and then creating a new one.")
}

func (r *ibPartitionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ibPartitionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.APIClient.IBPartitionsApi.DeleteIBPartition(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete partition",
			fmt.Sprintf("There was an error deleting an Infiniband partition: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()
}

//nolint:gocritic // Implements Terraform defined interface
package transport_partition

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

const notFoundMessage = "404 Not Found"

type transportPartitionResource struct {
	client *common.CrusoeClient
}

type transportPartitionResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	ProjectID          types.String `tfsdk:"project_id"`
	Name               types.String `tfsdk:"name"`
	TransportNetworkID types.String `tfsdk:"transport_network_id"`
}

func NewTransportPartitionResource() resource.Resource {
	return &transportPartitionResource{}
}

func (r *transportPartitionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *transportPartitionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_transport_partition"
}

func (r *transportPartitionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   apiDescID,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   apiDescName,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"transport_network_id": schema.StringAttribute{
				Required:      true,
				Description:   apiDescTransportNetworkID,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"project_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: providerDescProjectID,
				// cannot be updated in place
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *transportPartitionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceID, projectID, errMsg := common.ParseResourceIdentifiers(req, r.client, "transport_partition_id")
	if errMsg != "" {
		resp.Diagnostics.AddError("Failed to import transport partition", errMsg)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), resourceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
}

func (r *transportPartitionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan transportPartitionResourceModel
	if err := common.GetResourceModel(ctx, req.Plan, &plan, &resp.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())

	dataResp, httpResp, err := r.client.APIClient.IBPartitionsApi.CreateIBPartition(ctx, swagger.IbPartitionsPostRequestV1{
		Name:        plan.Name.ValueString(),
		IbNetworkId: plan.TransportNetworkID.ValueString(),
	}, projectID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to create partition",
			fmt.Sprintf("There was an error creating a transport partition: %s", common.UnpackAPIError(err)))

		return
	}

	transportPartitionToTerraformResourceModel(&dataResp, &plan)
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *transportPartitionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state transportPartitionResourceModel
	if err := common.GetResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	// We only have this parsing for transitioning from v1alpha4 to v1 because old tf state files will not
	// have project ID stored. So we will try to get a fallback project to pass to the API.
	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())

	partition, httpResp, err := r.client.APIClient.IBPartitionsApi.GetIBPartition(ctx, projectID, state.ID.ValueString())
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		if err.Error() == notFoundMessage {
			// partition has most likely been deleted out of band, so we update Terraform state to match
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError("Failed to get transport partition",
			fmt.Sprintf("Fetching Crusoe transport partition failed: %s\n\nIf the problem persists, contact support@crusoecloud.com", common.UnpackAPIError(err)))

		return
	}

	state.ProjectID = types.StringValue(projectID)
	transportPartitionToTerraformResourceModel(&partition, &state)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *transportPartitionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// This should be unreachable, since all properties are marked as needing replacement on update.
	resp.Diagnostics.AddWarning("In-place updates not supported",
		"Updating transport partitions in place is not currently supported. If you're seeing this message, please"+
			" reach out to support@crusoecloud.com and let us know. In the meantime, you should be able to update your"+
			" partition by deleting it and then creating a new one.")
}

func (r *transportPartitionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state transportPartitionResourceModel
	if err := common.GetResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	httpResp, err := r.client.APIClient.IBPartitionsApi.DeleteIBPartition(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete partition",
			fmt.Sprintf("There was an error deleting a transport partition: %s", common.UnpackAPIError(err)))

		return
	}
}

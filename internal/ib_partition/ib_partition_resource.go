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

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha4"
	"github.com/crusoecloud/terraform-provider-crusoe/internal"
)

type ibPartitionResource struct {
	client *swagger.APIClient
}

type ibPartitionResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	IBNetworkID types.String `tfsdk:"ib_network_id"`
}

func NewIBPartitionResource() resource.Resource {
	return &ibPartitionResource{}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *ibPartitionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*swagger.APIClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", internal.ErrorMsgProviderInitFailed)

		return
	}

	r.client = client
}

//nolint:gocritic // Implements Terraform defined interface
func (r *ibPartitionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ib_partition"
}

//nolint:gocritic // Implements Terraform defined interface
func (r *ibPartitionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
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
		},
	}
}

func (r *ibPartitionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *ibPartitionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ibPartitionResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	roleID, err := internal.GetRole(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Role ID", err.Error())

		return
	}

	dataResp, httpResp, err := r.client.IBPartitionsApi.CreateIBPartition(ctx, swagger.IbPartitionsPostRequest{
		RoleId:      roleID,
		Name:        plan.Name.ValueString(),
		IbNetworkId: plan.IBNetworkID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create partition",
			fmt.Sprintf("There was an error creating an Infiniband partition: %s", err.Error()))

		return
	}
	defer httpResp.Body.Close()

	plan.ID = types.StringValue(dataResp.Id)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *ibPartitionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ibPartitionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dataResp, httpResp, err := r.client.IBPartitionsApi.GetIBPartition(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get IB partition",
			fmt.Sprintf("Fetching Crusoe Infiniband partition failed: %s\n\nIf the problem persists, contact support@crusoeenergy.com", err.Error()))

		return
	}
	defer httpResp.Body.Close()

	// TODO: handle deleted out of band
	//if disk == nil {
	//	// disk has most likely been deleted out of band, so we update Terraform state to match
	//	resp.State.RemoveResource(ctx)
	//
	//	return
	//}

	state.ID = types.StringValue(dataResp.Id)
	state.Name = types.StringValue(dataResp.Name)
	state.IBNetworkID = types.StringValue(dataResp.IbNetworkId)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *ibPartitionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// This should be unreachable, since all properties are marked as needing replacement on update.
	resp.Diagnostics.AddWarning("In-place updates not supported",
		"Updating IB partitions in place is not currently supported. If you're seeing this message, please"+
			" reach out to support@crusoecloud.com and let us know. In the meantime, you should be able to update your"+
			" partition by deleting it and then creating a new one.")

}

//nolint:gocritic // Implements Terraform defined interface
func (r *ibPartitionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ibPartitionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.IBPartitionsApi.DeleteIBPartition(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete partition",
			fmt.Sprintf("There was an error deleting an Infiniband partition: %s", err.Error()))

		return
	}
	defer httpResp.Body.Close()
}

package vpc_network

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type vpcNetworkResource struct {
	client *swagger.APIClient
}

type vpcNetworkResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
	CIDR      types.String `tfsdk:"cidr"`
	Gateway   types.String `tfsdk:"gateway"`
	Subnets   types.List   `tfsdk:"subnets"`
}

func NewVPCNetworkResource() resource.Resource {
	return &vpcNetworkResource{}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vpcNetworkResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *vpcNetworkResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vpc_network"
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vpcNetworkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"project_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace()},
			},
			"cidr": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"gateway": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates

			},
			"subnets": schema.ListAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
		},
	}
}

func (r *vpcNetworkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vpcNetworkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vpcNetworkResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := ""
	if plan.ProjectID.ValueString() == "" {
		project, err := common.GetFallbackProject(ctx, r.client, &resp.Diagnostics)
		if err != nil {

			resp.Diagnostics.AddError("Failed to create VPC Network",
				fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))
			return
		}
		projectID = project
	} else {
		projectID = plan.ProjectID.ValueString()
	}

	dataResp, httpResp, err := r.client.VPCNetworksApi.CreateVPCNetwork(ctx, swagger.VpcNetworkPostRequest{
		Name: plan.Name.ValueString(),
		Cidr: plan.CIDR.ValueString(),
	}, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create VPC Network",
			fmt.Sprintf("There was an error starting a create VPC Network operation (%s): %s", projectID, common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	plan.ID = types.StringValue(dataResp.Network.Id)
	plan.Name = types.StringValue(dataResp.Network.Name)
	plan.CIDR = types.StringValue(dataResp.Network.Cidr)
	plan.ProjectID = types.StringValue(projectID)
	plan.Gateway = types.StringValue(dataResp.Network.Gateway)

	subnets, _ := types.ListValueFrom(context.Background(), types.StringType, dataResp.Network.Subnets)
	plan.Subnets = subnets

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vpcNetworkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vpcNetworkResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	vpcNetwork, httpResp, err := r.client.VPCNetworksApi.GetVPCNetwork(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get VPC Network",
			fmt.Sprintf("Fetching Crusoe VPC Networks failed: %s\n\nIf the problem persists, contact support@crusoecloud.com", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		// VPC Network has most likely been deleted out of band, so we update Terraform state to match
		resp.State.RemoveResource(ctx)

		return
	}

	state.Name = types.StringValue(vpcNetwork.Name)
	state.CIDR = types.StringValue(vpcNetwork.Cidr)
	state.Gateway = types.StringValue(vpcNetwork.Gateway)

	subnets, _ := types.ListValueFrom(context.Background(), types.StringType, vpcNetwork.Subnets)
	state.Subnets = subnets

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vpcNetworkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state vpcNetworkResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan vpcNetworkResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dataResp, httpResp, err := r.client.VPCNetworksApi.PatchVPCNetwork(ctx,
		swagger.VpcNetworkPatchRequest{Name: plan.Name.ValueString()},
		plan.ProjectID.ValueString(),
		plan.ID.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update VPC Network",
			fmt.Sprintf("There was an error starting an update VPC Network operation: %s.\n\n", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	_, _, err = common.AwaitOperationAndResolve[swagger.VpcNetwork](ctx, dataResp.Operation, plan.ProjectID.ValueString(), r.client.VPCNetworkOperationsApi.GetNetworkingVPCNetworksOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update VPC Network",
			fmt.Sprintf("There was an error updating the VPC Network: %s.\n\n", common.UnpackAPIError(err)))

		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vpcNetworkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vpcNetworkResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dataResp, httpResp, err := r.client.VPCNetworksApi.DeleteVPCNetwork(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete VPC Network",
			fmt.Sprintf("There was an error starting a delete VPC Network operation: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	_, err = common.AwaitOperation(ctx, dataResp.Operation, state.ProjectID.ValueString(), r.client.VPCNetworkOperationsApi.GetNetworkingVPCNetworksOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete VPC Network",
			fmt.Sprintf("There was an error deleting a VPC Network: %s", common.UnpackAPIError(err)))

		return
	}
}

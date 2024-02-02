package vpc_subnet

import (
	"context"
	"fmt"
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

type vpcSubnetResource struct {
	client *swagger.APIClient
}

type vpcSubnetResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
	CIDR      types.String `tfsdk:"cidr"`
	Location  types.String `tfsdk:"location"`
	Network   types.String `tfsdk:"network"`
}

func NewVPCSubnetResource() resource.Resource {
	return &vpcSubnetResource{}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vpcSubnetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *vpcSubnetResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vpc_subnet"
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vpcSubnetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
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
			"cidr": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"location": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				}, // maintain across updates
			},
			"network": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				}, // maintain across updates
			},
		},
	}
}

func (r *vpcSubnetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vpcSubnetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vpcSubnetResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := ""
	if plan.ProjectID.ValueString() == "" {
		project, err := common.GetFallbackProject(ctx, r.client, &resp.Diagnostics)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create VPC Subnet",
				fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

			return
		}
		projectID = project
	} else {
		projectID = plan.ProjectID.ValueString()
	}

	dataResp, httpResp, err := r.client.VPCSubnetsApi.CreateVPCSubnet(ctx, swagger.VpcSubnetPostRequest{
		Name:         plan.Name.ValueString(),
		Cidr:         plan.CIDR.ValueString(),
		Location:     plan.Location.ValueString(),
		VpcNetworkId: plan.Network.ValueString(),
	}, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create VPC Subnet",
			fmt.Sprintf("There was an error starting a create VPC Subnet operation (%s): %s", projectID, common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	plan.ID = types.StringValue(dataResp.Subnet.Id)
	plan.Name = types.StringValue(dataResp.Subnet.Name)
	plan.CIDR = types.StringValue(dataResp.Subnet.Cidr)
	plan.Location = types.StringValue(dataResp.Subnet.Location)
	plan.Network = types.StringValue(dataResp.Subnet.VpcNetworkId)
	plan.ProjectID = types.StringValue(projectID)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vpcSubnetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vpcSubnetResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	vpcSubnet, httpResp, err := r.client.VPCSubnetsApi.GetVPCSubnet(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get VPC Subnet",
			fmt.Sprintf("Fetching Crusoe VPC Subnets failed: %s\n\nIf the problem persists, contact support@crusoecloud.com", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		// VPC Subnet has most likely been deleted out of band, so we update Terraform state to match
		resp.State.RemoveResource(ctx)

		return
	}

	state.Name = types.StringValue(vpcSubnet.Name)
	state.CIDR = types.StringValue(vpcSubnet.Cidr)
	state.Location = types.StringValue(vpcSubnet.Location)
	state.Network = types.StringValue(vpcSubnet.VpcNetworkId)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vpcSubnetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state vpcSubnetResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan vpcSubnetResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dataResp, httpResp, err := r.client.VPCSubnetsApi.PatchVPCSubnet(ctx,
		swagger.VpcSubnetPatchRequest{Name: plan.Name.ValueString()},
		plan.ProjectID.ValueString(),
		plan.ID.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update VPC Subnet",
			fmt.Sprintf("There was an error starting an update VPC Subnet operation: %s.\n\n", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	_, _, err = common.AwaitOperationAndResolve[swagger.VpcSubnet](ctx, dataResp.Operation, plan.ProjectID.ValueString(), func(ctx context.Context, projectID string, opID string) (swagger.Operation, *http.Response, error) {
		return r.client.VPCSubnetOperationsApi.GetNetworkingVPCSubnetsOperation(ctx, projectID, opID)
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update VPC Subnet",
			fmt.Sprintf("There was an error updating the VPC Subnet: %s.\n\n", common.UnpackAPIError(err)))

		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vpcSubnetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vpcSubnetResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dataResp, httpResp, err := r.client.VPCSubnetsApi.DeleteVPCSubnet(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete VPC Subnet",
			fmt.Sprintf("There was an error starting a delete VPC Subnet operation: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	_, err = common.AwaitOperation(ctx, dataResp.Operation, state.ProjectID.ValueString(), func(ctx context.Context, projectID string, opID string) (swagger.Operation, *http.Response, error) {
		return r.client.VPCSubnetOperationsApi.GetNetworkingVPCSubnetsOperation(ctx, projectID, opID)
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete VPC Subnet",
			fmt.Sprintf("There was an error deleting a VPC Subnet: %s", common.UnpackAPIError(err)))

		return
	}
}

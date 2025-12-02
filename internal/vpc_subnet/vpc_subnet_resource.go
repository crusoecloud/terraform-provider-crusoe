package vpc_subnet

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type vpcSubnetResource struct {
	client *common.CrusoeClient
}

type vpcSubnetResourceModel struct {
	ID                types.String `tfsdk:"id"`
	ProjectID         types.String `tfsdk:"project_id"`
	Name              types.String `tfsdk:"name"`
	CIDR              types.String `tfsdk:"cidr"`
	Location          types.String `tfsdk:"location"`
	Network           types.String `tfsdk:"network"`
	NATGatewayEnabled types.Bool   `tfsdk:"nat_gateway_enabled"`
	NATGateways       types.List   `tfsdk:"nat_gateways"`
}

type vpcSubnetNatGatewayResourceModel struct {
	ID                types.String `tfsdk:"id"`
	PublicIpv4Address types.String `tfsdk:"public_ipv4_address"`
	PublicIpv4Id      types.String `tfsdk:"public_ipv4_id"`
}

func NewVPCSubnetResource() resource.Resource {
	return &vpcSubnetResource{}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vpcSubnetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *vpcSubnetResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vpc_subnet"
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vpcSubnetResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 2,
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
			"nat_gateway_enabled": schema.BoolAttribute{
				MarkdownDescription: common.DevelopmentMessage,
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"nat_gateways": schema.ListNestedAttribute{
				MarkdownDescription: common.DevelopmentMessage,
				Computed:            true,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				NestedObject: schema.NestedAttributeObject{
					PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed: true,
						},
						"public_ipv4_address": schema.StringAttribute{
							Computed: true,
						},
						"public_ipv4_id": schema.StringAttribute{
							Computed: true,
						},
					},
				},
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

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())

	dataResp, httpResp, err := r.client.APIClient.VPCSubnetsApi.CreateVPCSubnet(ctx, swagger.VpcSubnetPostRequest{
		Name:              plan.Name.ValueString(),
		Cidr:              plan.CIDR.ValueString(),
		Location:          plan.Location.ValueString(),
		VpcNetworkId:      plan.Network.ValueString(),
		NatGatewayEnabled: plan.NATGatewayEnabled.ValueBool(),
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

	natGatewaysList, natDiags := natGatewaysToTerraformResourceModel(ctx, dataResp.Subnet.NatGateways)
	if natDiags.HasError() {
		resp.Diagnostics.Append(natDiags...)

		return
	}
	plan.NATGateways = natGatewaysList
	plan.NATGatewayEnabled = types.BoolValue(len(natGatewaysList.Elements()) > 0)

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

	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())

	vpcSubnet, httpResp, err := r.client.APIClient.VPCSubnetsApi.GetVPCSubnet(ctx, projectID, state.ID.ValueString())
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

	vpcSubnetToTerraformResourceModel(ctx, &vpcSubnet, &state, &resp.Diagnostics)

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

	patchReq := swagger.VpcSubnetPatchRequest{
		Name: plan.Name.ValueString(),
	}
	if !plan.NATGatewayEnabled.IsUnknown() && !plan.NATGatewayEnabled.IsNull() {
		switch plan.NATGatewayEnabled.ValueBool() {
		case true:
			patchReq.NatGatewayAction = "enable"
		case false:
			patchReq.NatGatewayAction = "disable"
		}
	}

	dataResp, httpResp, err := r.client.APIClient.VPCSubnetsApi.PatchVPCSubnet(ctx, patchReq,
		plan.ProjectID.ValueString(), plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to update VPC Subnet",
			fmt.Sprintf("There was an error starting an update VPC Subnet operation: %s.\n\n", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	_, _, err = common.AwaitOperationAndResolve[swagger.VpcSubnet](ctx, dataResp.Operation, plan.ProjectID.ValueString(), func(ctx context.Context, projectID string, opID string) (swagger.Operation, *http.Response, error) {
		return r.client.APIClient.VPCSubnetOperationsApi.GetNetworkingVPCSubnetsOperation(ctx, projectID, opID)
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

	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())

	dataResp, httpResp, err := r.client.APIClient.VPCSubnetsApi.DeleteVPCSubnet(ctx, projectID, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete VPC Subnet",
			fmt.Sprintf("There was an error starting a delete VPC Subnet operation: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	_, err = common.AwaitOperation(ctx, dataResp.Operation, projectID, func(ctx context.Context, projectID string, opID string) (swagger.Operation, *http.Response, error) {
		return r.client.APIClient.VPCSubnetOperationsApi.GetNetworkingVPCSubnetsOperation(ctx, projectID, opID)
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete VPC Subnet",
			fmt.Sprintf("There was an error deleting a VPC Subnet: %s", common.UnpackAPIError(err)))

		return
	}
}

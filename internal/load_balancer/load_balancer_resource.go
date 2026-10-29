package load_balancer

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

const maxPort = 65535

var errMissingCreateResult = errors.New("create operation returned no load balancer ID")

type loadBalancerResource struct {
	client *common.CrusoeClient
}

type loadBalancerResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	ProjectID              types.String `tfsdk:"project_id"`
	Name                   types.String `tfsdk:"name"`
	Location               types.String `tfsdk:"location"`
	VPCID                  types.String `tfsdk:"vpc_id"`
	Protocol               types.String `tfsdk:"protocol"`
	VIP                    types.String `tfsdk:"vip"`
	ListenPortsAndBackends types.Set    `tfsdk:"listen_ports_and_backends"`
	HealthCheckOptions     types.Object `tfsdk:"health_check_options"`
}

type listenPortAndBackendResourceModel struct {
	ListenPort types.Int64 `tfsdk:"listen_port"`
	Backends   types.Set   `tfsdk:"backends"`
}

type backendResourceModel struct {
	IP   types.String `tfsdk:"ip"`
	Port types.Int64  `tfsdk:"port"`
}

type healthCheckOptionsResourceModel struct {
	Timeout      types.Int64 `tfsdk:"timeout"`
	Interval     types.Int64 `tfsdk:"interval"`
	SuccessCount types.Int64 `tfsdk:"success_count"`
	FailureCount types.Int64 `tfsdk:"failure_count"`
}

func NewLoadBalancerResource() resource.Resource {
	return &loadBalancerResource{}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *loadBalancerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *loadBalancerResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_load_balancer"
}

//nolint:gocritic // Implements Terraform defined interface
func (r *loadBalancerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: common.DevelopmentMessage,
		Version:             2,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: apiDescID,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"project_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: providerDescProjectID,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescName,
				// ExternalLoadBalancerPatchRequest cannot carry a name, so renames replace.
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"location": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescLocation,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"vpc_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescVPCID,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"protocol": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescProtocol + " " + providerDescProtocol,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:          []validator.String{stringvalidator.OneOf("tcp", "udp")},
			},
			"vip": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: apiDescVIP,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			// A set because the API returns these in arbitrary order.
			"listen_ports_and_backends": schema.SetNestedAttribute{
				Required:            true,
				MarkdownDescription: apiDescListenPortsAndBackends,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"listen_port": schema.Int64Attribute{
							Required:            true,
							MarkdownDescription: apiDescListenPort,
							Validators:          []validator.Int64{int64validator.Between(1, maxPort)},
						},
						"backends": schema.SetNestedAttribute{
							Required:            true,
							MarkdownDescription: apiDescBackends,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"ip": schema.StringAttribute{
										Required:            true,
										MarkdownDescription: apiDescBackendIP + " " + providerDescBackendIP,
									},
									"port": schema.Int64Attribute{
										Required:            true,
										MarkdownDescription: apiDescBackendPort,
										Validators:          []validator.Int64{int64validator.Between(1, maxPort)},
									},
								},
							},
						},
					},
				},
			},
			"health_check_options": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: providerDescHealthCheckOptions,
				PlanModifiers:       []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"timeout": schema.Int64Attribute{
						Required:            true,
						MarkdownDescription: apiDescHealthCheckTimeout,
						Validators:          []validator.Int64{int64validator.AtLeast(1)},
					},
					"interval": schema.Int64Attribute{
						Required:            true,
						MarkdownDescription: apiDescHealthCheckInterval,
						Validators:          []validator.Int64{int64validator.AtLeast(1)},
					},
					"success_count": schema.Int64Attribute{
						Required:            true,
						MarkdownDescription: apiDescHealthCheckSuccessCount,
						Validators:          []validator.Int64{int64validator.AtLeast(1)},
					},
					"failure_count": schema.Int64Attribute{
						Required:            true,
						MarkdownDescription: apiDescHealthCheckFailureCount,
						Validators:          []validator.Int64{int64validator.AtLeast(1)},
					},
				},
			},
		},
	}
}

func (r *loadBalancerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceID, projectID, errMsg := common.ParseResourceIdentifiers(req, r.client, "load_balancer_id")
	if errMsg != "" {
		resp.Diagnostics.AddError("Failed to import load balancer", errMsg)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), resourceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
}

// readInto fetches the load balancer and maps it into state. Returns false if the resource is
// gone, so callers can drop it from state.
func (r *loadBalancerResource) readInto(ctx context.Context, projectID, loadBalancerID string,
	state *loadBalancerResourceModel, diags *diag.Diagnostics,
) (found bool) {
	loadBalancer, httpResp, err := r.client.APIClient.LoadBalancersApi.GetExternalLoadBalancer(ctx, projectID, loadBalancerID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
		return false
	}
	if err != nil {
		diags.AddError("Failed to get load balancer",
			fmt.Sprintf("Fetching load balancer failed: %s\n\nIf the problem persists, contact support@crusoecloud.com", common.UnpackAPIError(err)))

		return false
	}

	loadBalancerToResourceModel(ctx, &loadBalancer, state, diags)

	return true
}

//nolint:gocritic // Implements Terraform defined interface
func (r *loadBalancerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan loadBalancerResourceModel
	if err := common.GetResourceModel(ctx, req.Plan, &plan, &resp.Diagnostics); err != nil {
		return
	}

	postReq := swagger.ExternalLoadBalancerPostRequest{
		Name:                   plan.Name.ValueString(),
		Location:               plan.Location.ValueString(),
		Protocol:               plan.Protocol.ValueString(),
		VpcId:                  plan.VPCID.ValueString(),
		ListenPortsAndBackends: listenPortsAndBackendsToSwagger(ctx, plan.ListenPortsAndBackends, &resp.Diagnostics),
		HealthCheckOptions:     healthCheckOptionsToSwagger(ctx, plan.HealthCheckOptions, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())

	dataResp, httpResp, err := r.client.APIClient.LoadBalancersApi.CreateExternalLoadBalancer(ctx, postReq, projectID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to create load balancer",
			fmt.Sprintf("There was an error starting a create load balancer operation (%s): %s", projectID, common.UnpackAPIError(err)))

		return
	}

	created, _, err := common.AwaitOperationAndResolve[swagger.ExternalLoadBalancer](
		ctx, dataResp.Operation, projectID, r.client.APIClient.LoadBalancerOperationsApi.GetExternalLoadBalancerOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create load balancer",
			fmt.Sprintf("There was an error creating a load balancer: %s", common.UnpackAPIError(err)))

		return
	}
	if created == nil || created.Id == "" {
		resp.Diagnostics.AddError("Failed to create load balancer", errMissingCreateResult.Error())

		return
	}

	// The operation result omits location and vip, so state is mapped from a follow-up read.
	found := r.readInto(ctx, projectID, created.Id, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to create load balancer", "Load balancer was not found after creation.")

		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *loadBalancerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state loadBalancerResourceModel
	if err := common.GetResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())

	if !r.readInto(ctx, projectID, state.ID.ValueString(), &state, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}

		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *loadBalancerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state loadBalancerResourceModel
	if err := common.GetResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	var plan loadBalancerResourceModel
	if err := common.GetResourceModel(ctx, req.Plan, &plan, &resp.Diagnostics); err != nil {
		return
	}

	patchReq := swagger.ExternalLoadBalancerPatchRequest{
		ListenPortsAndBackends: listenPortsAndBackendsToSwagger(ctx, plan.ListenPortsAndBackends, &resp.Diagnostics),
		HealthCheckOptions:     healthCheckOptionsToSwagger(ctx, plan.HealthCheckOptions, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())
	loadBalancerID := state.ID.ValueString()

	dataResp, httpResp, err := r.client.APIClient.LoadBalancersApi.UpdateExternalLoadBalancer(ctx, patchReq, projectID, loadBalancerID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to update load balancer",
			fmt.Sprintf("There was an error starting an update load balancer operation: %s", common.UnpackAPIError(err)))

		return
	}

	// The update operation carries an empty result, so state is mapped from a follow-up read.
	_, err = common.AwaitOperation(ctx, dataResp.Operation, projectID,
		r.client.APIClient.LoadBalancerOperationsApi.GetExternalLoadBalancerOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update load balancer",
			fmt.Sprintf("There was an error updating the load balancer: %s.\n\n", common.UnpackAPIError(err)))

		return
	}

	if !r.readInto(ctx, projectID, loadBalancerID, &plan, &resp.Diagnostics) || resp.Diagnostics.HasError() {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Failed to update load balancer", "Load balancer was not found after update.")
		}

		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *loadBalancerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state loadBalancerResourceModel
	if err := common.GetResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())

	dataResp, httpResp, err := r.client.APIClient.LoadBalancersApi.DeleteExternalLoadBalancer(ctx, projectID, state.ID.ValueString())
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete load balancer",
			fmt.Sprintf("There was an error starting a delete load balancer operation: %s", common.UnpackAPIError(err)))

		return
	}

	_, err = common.AwaitOperation(ctx, dataResp.Operation, projectID,
		r.client.APIClient.LoadBalancerOperationsApi.GetExternalLoadBalancerOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete load balancer",
			fmt.Sprintf("There was an error deleting a load balancer: %s", common.UnpackAPIError(err)))
	}
}

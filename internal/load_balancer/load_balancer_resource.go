package load_balancer

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	validators "github.com/crusoecloud/terraform-provider-crusoe/internal/validators"
)

type loadBalancerResource struct {
	client *common.CrusoeClient
}

type loadBalancerResourceModel struct {
	ID                types.String `tfsdk:"id"`
	ProjectID         types.String `tfsdk:"project_id"`
	Name              types.String `tfsdk:"name"`
	NetworkInterfaces types.List   `tfsdk:"network_interfaces"`
	Destinations      types.List   `tfsdk:"destinations"`
	Location          types.String `tfsdk:"location"`
	Protocols         types.List   `tfsdk:"protocols"`
	Algorithm         types.String `tfsdk:"algorithm"`
	Type              types.String `tfsdk:"type"`
	IPs               types.List   `tfsdk:"ips"`
	HealthCheck       types.Object `tfsdk:"health_check"`
}

type loadBalancerNetworkTargetModel struct {
	Cidr       types.String `tfsdk:"cidr"`
	ResourceID types.String `tfsdk:"resource_id"`
}

type loadBalancerNetworkInterfaceModel struct {
	Network types.String `tfsdk:"network"`
	Subnet  types.String `tfsdk:"subnet"`
}

type loadBalancerIPAddressModel struct {
	PrivateIPv4 types.Object                        `tfsdk:"private_ipv4"`
	PublicIpv4  loadBalancerPublicIPv4ResourceModel `tfsdk:"public_ipv4"`
}

type loadBalancerPublicIPv4ResourceModel struct {
	ID      types.String `tfsdk:"id"`
	Address types.String `tfsdk:"address"`
	Type    types.String `tfsdk:"type"`
}

type healthCheckOptionsResourceModel struct {
	Timeout      types.String `tfsdk:"timeout"`
	Port         types.String `tfsdk:"port"`
	Interval     types.String `tfsdk:"interval"`
	SuccessCount types.String `tfsdk:"success_count"`
	FailureCount types.String `tfsdk:"failure_count"`
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
		Version:             1,
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
					stringplanmodifier.RequiresReplace(), // cannot be updated in place
				},
			},
			"name": schema.StringAttribute{
				Required: true,
			},
			"network_interfaces": schema.ListNestedAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}, // maintain across updates
				NestedObject: schema.NestedAttributeObject{
					PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, // maintain across updates
					Attributes: map[string]schema.Attribute{
						"network": schema.StringAttribute{
							Computed:      true,
							Optional:      true,
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
						"subnet": schema.StringAttribute{
							Computed:      true,
							Optional:      true,
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
					},
				},
			},
			"destinations": schema.ListNestedAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}, // maintain across updates
				NestedObject: schema.NestedAttributeObject{
					PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, // maintain across updates
					Attributes: map[string]schema.Attribute{
						"cidr": schema.StringAttribute{
							Computed:      true,
							Optional:      true,
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},                                                   // maintain across updates
							Validators:    []validator.String{validators.RegexValidator{RegexPattern: "^(\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}/32)?$"}}, // load balancers only support the /32 mask
						},
						"resource_id": schema.StringAttribute{
							Computed:      true,
							Optional:      true,
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
					},
				},
			},
			"location": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"protocols": schema.ListAttribute{
				ElementType:   types.StringType,
				Required:      true,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"algorithm": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
				Validators:    []validator.String{stringvalidator.OneOf("random")},         // we currently only support random
			},
			"type": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"ips": schema.ListNestedAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}, // maintain across updates
				NestedObject: schema.NestedAttributeObject{
					PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, // maintain across updates
					Attributes: map[string]schema.Attribute{
						"public_ipv4": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Computed:      true,
									PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
								},
								"address": schema.StringAttribute{
									Computed:      true,
									PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
								},
								"type": schema.StringAttribute{
									Computed: true,
								},
							},
							PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
						"private_ipv4": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"address": schema.StringAttribute{
									Computed: true,
								},
							},
							PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
					},
				},
			},
			"health_check": schema.SingleNestedAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, // maintain across updates
				Attributes: map[string]schema.Attribute{
					"timeout": schema.StringAttribute{
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
					},
					"port": schema.StringAttribute{
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
					},
					"interval": schema.StringAttribute{
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
					},
					"success_count": schema.StringAttribute{
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
					},
					"failure_count": schema.StringAttribute{
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
					},
				},
			},
		},
	}
}

func (r *loadBalancerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *loadBalancerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan loadBalancerResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	postReq := swagger.LoadBalancersPostRequest{
		Algorithm: plan.Algorithm.ValueString(),
		Location:  plan.Location.ValueString(),
		Name:      plan.Name.ValueString(),
	}

	// network interfaces
	tNetworkInterfaces := make([]loadBalancerNetworkInterfaceModel, 0, len(plan.NetworkInterfaces.Elements()))
	diags = plan.NetworkInterfaces.ElementsAs(ctx, &tNetworkInterfaces, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	networkInterfaces := make([]swagger.LoadBalancerNetworkInterface, 0, len(tNetworkInterfaces))
	for _, n := range tNetworkInterfaces {
		networkInterfaces = append(networkInterfaces, swagger.LoadBalancerNetworkInterface{
			Network: n.Network.ValueString(),
			Subnet:  n.Subnet.ValueString(),
		})
	}
	postReq.NetworkInterfaces = networkInterfaces

	// destinations
	tDestinations := make([]loadBalancerNetworkTargetModel, 0, len(plan.Destinations.Elements()))
	diags = plan.Destinations.ElementsAs(ctx, &tDestinations, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	destinations := make([]swagger.NetworkTarget, 0, len(tDestinations))
	for _, d := range tDestinations {
		destinations = append(destinations, swagger.NetworkTarget{
			Cidr:       d.Cidr.ValueString(),
			ResourceId: d.ResourceID.ValueString(),
		})
	}
	postReq.Destinations = destinations

	// health check
	var healthCheck swagger.HealthCheckOptions
	if !plan.HealthCheck.IsNull() && !plan.HealthCheck.IsUnknown() {
		plan.HealthCheck.As(ctx, healthCheck, basetypes.ObjectAsOptions{})
		postReq.HealthCheck = &healthCheck
	}

	// protocols
	protocols := make([]string, 0, len(plan.Protocols.Elements()))
	diags = plan.Protocols.ElementsAs(ctx, &protocols, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	postReq.Protocols = protocols

	// project id
	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())

	dataResp, httpResp, err := r.client.APIClient.InternalLoadBalancersApi.CreateLoadBalancer(ctx, postReq, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create load balancer",
			fmt.Sprintf("There was an error starting a create load balancer operation (%s): %s", projectID, common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	loadBalancer, _, err := common.AwaitOperationAndResolve[swagger.LoadBalancer](
		ctx, dataResp.Operation, projectID, r.client.APIClient.InternalLoadBalancerOperationsApi.GetNetworkingLoadBalancersOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create load balancer",
			fmt.Sprintf("There was an error creating a load balancer: %s", common.UnpackAPIError(err)))

		return
	}

	plan.ID = types.StringValue(loadBalancer.Id)
	ips, _ := loadBalancerIPsToTerraformResourceModel(loadBalancer.Ips)
	plan.IPs = ips
	plan.ProjectID = types.StringValue(projectID)
	plan.Type = types.StringValue(loadBalancer.Type_)
	plan.NetworkInterfaces, _ = loadBalancerNetworkInterfacesToTerraformResourceModel(loadBalancer.NetworkInterfaces)
	plan.HealthCheck, _ = types.ObjectValueFrom(ctx, loadBalancerHealthCheckSchema.AttrTypes, loadBalancerHealthCheckToTerraformResourceModel(loadBalancer.HealthCheck))
	plan.Destinations, _ = loadBalancerDestinationsToTerraformResourceModel(loadBalancer.Destinations)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *loadBalancerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state loadBalancerResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())

	loadBalancer, httpResp, err := r.client.APIClient.InternalLoadBalancersApi.GetLoadBalancer(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get load balancer",
			fmt.Sprintf("Fetching load balancer failed: %s\n\nIf the problem persists, contact support@crusoecloud.com", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		// Load balancer has most likely been deleted out of band, so we update Terraform state to match
		resp.State.RemoveResource(ctx)

		return
	}

	state.ProjectID = types.StringValue(projectID)
	loadBalancerUpdateTerraformState(ctx, &loadBalancer, &state)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *loadBalancerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state loadBalancerResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan loadBalancerResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	patchReq := swagger.LoadBalancersPatchRequestV1Alpha5{}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		patchReq.Name = plan.Name.ValueString()
	}

	if !plan.Destinations.IsNull() && !plan.Destinations.IsUnknown() {
		tDestinations := make([]loadBalancerNetworkTargetModel, 0, len(plan.Destinations.Elements()))
		diags = plan.Destinations.ElementsAs(ctx, &tDestinations, true)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		destinations := make([]swagger.NetworkTarget, 0, len(tDestinations))
		for _, d := range tDestinations {
			destinations = append(destinations, swagger.NetworkTarget{
				Cidr:       d.Cidr.ValueString(),
				ResourceId: d.ResourceID.ValueString(),
			})
		}
		patchReq.Destinations = destinations
	}

	healthCheckAttributesMap := plan.HealthCheck.Attributes()

	if !healthCheckAttributesMap["timeout"].IsNull() && !healthCheckAttributesMap["timeout"].IsUnknown() {
		patchReq.HealthCheck.Timeout = healthCheckAttributesMap["timeout"].String()
	}
	if !healthCheckAttributesMap["port"].IsNull() && !healthCheckAttributesMap["port"].IsUnknown() {
		patchReq.HealthCheck.Port = healthCheckAttributesMap["port"].String()
	}
	if !healthCheckAttributesMap["interval"].IsNull() && !healthCheckAttributesMap["interval"].IsUnknown() {
		patchReq.HealthCheck.Interval = healthCheckAttributesMap["interval"].String()
	}
	if !healthCheckAttributesMap["success_count"].IsNull() && !healthCheckAttributesMap["success_count"].IsUnknown() {
		patchReq.HealthCheck.SuccessCount = healthCheckAttributesMap["success_count"].String()
	}
	if !healthCheckAttributesMap["failure_count"].IsNull() && !healthCheckAttributesMap["failure_count"].IsUnknown() {
		patchReq.HealthCheck.FailureCount = healthCheckAttributesMap["failure_count"].String()
	}

	dataResp, httpResp, err := r.client.APIClient.InternalLoadBalancersApi.PatchLoadBalancer(ctx,
		patchReq,
		plan.ProjectID.ValueString(),
		plan.ID.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update load balancer",
			fmt.Sprintf("There was an error starting an update load balancer operation: %s.\n\n", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	_, _, err = common.AwaitOperationAndResolve[swagger.LoadBalancer](ctx, dataResp.Operation, plan.ProjectID.ValueString(), func(ctx context.Context, projectID string, opID string) (swagger.Operation, *http.Response, error) {
		return r.client.APIClient.InternalLoadBalancerOperationsApi.GetNetworkingLoadBalancersOperation(ctx, projectID, opID)
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update load balancer",
			fmt.Sprintf("There was an error updating the load balancer: %s.\n\n", common.UnpackAPIError(err)))

		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *loadBalancerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state loadBalancerResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dataResp, httpResp, err := r.client.APIClient.InternalLoadBalancersApi.DeleteLoadBalancer(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete load balancer",
			fmt.Sprintf("There was an error starting a delete load balancer operation: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	_, err = common.AwaitOperation(ctx, dataResp.Operation, state.ProjectID.ValueString(), func(ctx context.Context, projectID string, opID string) (swagger.Operation, *http.Response, error) {
		return r.client.APIClient.InternalLoadBalancerOperationsApi.GetNetworkingLoadBalancersOperation(ctx, projectID, opID)
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete load balancer",
			fmt.Sprintf("There was an error deleting a load balancer: %s", common.UnpackAPIError(err)))

		return
	}
}

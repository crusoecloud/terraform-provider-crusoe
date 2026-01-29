package firewall_rule

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	validators "github.com/crusoecloud/terraform-provider-crusoe/internal/validators"
)

type firewallRuleResource struct {
	client *common.CrusoeClient
}

type firewallRuleResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ProjectID        types.String `tfsdk:"project_id"`
	Name             types.String `tfsdk:"name"`
	Network          types.String `tfsdk:"network"`
	Action           types.String `tfsdk:"action"`
	Direction        types.String `tfsdk:"direction"`
	Protocols        types.String `tfsdk:"protocols"`
	Source           types.String `tfsdk:"source"`
	SourcePorts      types.String `tfsdk:"source_ports"`
	Destination      types.String `tfsdk:"destination"`
	DestinationPorts types.String `tfsdk:"destination_ports"`
}

func NewFirewallRuleResource() resource.Resource {
	return &firewallRuleResource{}
}

func (r *firewallRuleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *firewallRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vpc_firewall_rule"
}

func (r *firewallRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"name": schema.StringAttribute{
				Required:      true,
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
			"network": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"action": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validators.RegexValidator{RegexPattern: "^allow$"}}, // TODO: support deny once supported by API
			},
			"direction": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validators.RegexValidator{RegexPattern: "^(ingress|egress)"}},
			},
			"protocols": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
				// TODO: add validator
			},
			"source": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
				// TODO: add validator
			},
			"source_ports": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
				// TODO: add validator
			},
			"destination": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
				// TODO: add validator
			},
			"destination_ports": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
				// TODO: add validator
			},
		},
	}
}

func (r *firewallRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *firewallRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan firewallRuleResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())

	sourcePortsStr := strings.ReplaceAll(plan.SourcePorts.ValueString(), "*", "1-65535")
	destPortsStr := strings.ReplaceAll(plan.DestinationPorts.ValueString(), "*", "1-65535")

	dataResp, httpResp, err := r.client.APIClient.VPCFirewallRulesApi.CreateVPCFirewallRule(ctx, swagger.VpcFirewallRulesPostRequestV1Alpha5{
		VpcNetworkId:     plan.Network.ValueString(),
		Name:             plan.Name.ValueString(),
		Action:           plan.Action.ValueString(),
		Protocols:        stringToSlice(plan.Protocols.ValueString(), ","),
		Direction:        plan.Direction.ValueString(),
		Sources:          []swagger.FirewallRuleObject{toFirewallRuleObject(plan.Source.ValueString())},
		SourcePorts:      stringToSlice(sourcePortsStr, ","),
		Destinations:     []swagger.FirewallRuleObject{toFirewallRuleObject(plan.Destination.ValueString())},
		DestinationPorts: stringToSlice(destPortsStr, ","),
	}, projectID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to create firewall rule",
			fmt.Sprintf("There was an error starting a create firewall rule operation: %s", common.UnpackAPIError(err)))

		return
	}

	firewallRule, _, err := common.AwaitOperationAndResolve[swagger.VpcFirewallRule](
		ctx, dataResp.Operation, projectID,
		r.client.APIClient.VPCFirewallRuleOperationsApi.GetNetworkingVPCFirewallRulesOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create firewall rule",
			fmt.Sprintf("There was an error creating a firewall rule: %s", common.UnpackAPIError(err)))

		return
	}

	plan.ID = types.StringValue(firewallRule.Id)
	plan.ProjectID = types.StringValue(projectID)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *firewallRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state firewallRuleResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// We only have this parsing for transitioning from v1alpha4 to v1alpha5 because old tf state files will not
	// have project ID stored. So we will try to get a fallback project to pass to the API.
	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())

	rule, httpResp, err := r.client.APIClient.VPCFirewallRulesApi.GetVPCFirewallRule(ctx, projectID, state.ID.ValueString())
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		// fw rule has most likely been deleted out of band, so we update Terraform state to match
		resp.State.RemoveResource(ctx)

		return
	}

	state.ProjectID = types.StringValue(projectID)
	firewallRuleToTerraformResourceModel(&rule, &state)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *firewallRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state firewallRuleResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan firewallRuleResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	patchReq := swagger.VpcFirewallRulesPatchRequest{}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		patchReq.Name = plan.Name.ValueString()
	}
	if !plan.Protocols.IsNull() && !plan.Protocols.IsUnknown() {
		patchReq.Protocols = stringToSlice(plan.Protocols.ValueString(), ",")
	}
	if !plan.Destination.IsNull() && !plan.Destination.IsUnknown() {
		patchReq.Destinations = []swagger.FirewallRuleObject{toFirewallRuleObject(plan.Destination.ValueString())}
	}
	if !plan.DestinationPorts.IsNull() && !plan.DestinationPorts.IsUnknown() {
		patchReq.DestinationPorts = stringToSlice(plan.DestinationPorts.ValueString(), ",")
	}
	if !plan.Source.IsNull() && !plan.Source.IsUnknown() {
		patchReq.Sources = []swagger.FirewallRuleObject{toFirewallRuleObject(plan.Source.ValueString())}
	}
	if !plan.SourcePorts.IsNull() && !plan.SourcePorts.IsUnknown() {
		patchReq.SourcePorts = stringToSlice(plan.SourcePorts.ValueString(), ",")
	}

	dataResp, httpResp, err := r.client.APIClient.VPCFirewallRulesApi.PatchVPCFirewallRule(ctx,
		patchReq,
		plan.ProjectID.ValueString(),
		plan.ID.ValueString(),
	)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to patch firewall rule",
			fmt.Sprintf("There was an error updating the firewall rule: %s.", common.UnpackAPIError(err)))

		return
	}

	_, _, err = common.AwaitOperationAndResolve[swagger.VpcFirewallRule](ctx, dataResp.Operation, plan.ProjectID.ValueString(), r.client.APIClient.VPCFirewallRuleOperationsApi.GetNetworkingVPCFirewallRulesOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to patch firewall rule",
			fmt.Sprintf("There was an error updating the firewall rule: %s.", common.UnpackAPIError(err)))

		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *firewallRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state firewallRuleResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dataResp, httpResp, err := r.client.APIClient.VPCFirewallRulesApi.DeleteVPCFirewallRule(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete firewall rule",
			fmt.Sprintf("There was an error starting a delete firewall rule operation: %s", common.UnpackAPIError(err)))

		return
	}

	_, err = common.AwaitOperation(ctx, dataResp.Operation, state.ProjectID.ValueString(), r.client.APIClient.VPCFirewallRuleOperationsApi.GetNetworkingVPCFirewallRulesOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete firewall rule",
			fmt.Sprintf("There was an error deleting a firewall rule: %s", common.UnpackAPIError(err)))
	}
}

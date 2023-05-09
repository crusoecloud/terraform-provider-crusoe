package firewall_rule

import (
	"context"
	"strings"

	"terraform-provider-crusoe/internal"
	validators "terraform-provider-crusoe/internal/validators"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "gitlab.com/crusoeenergy/island/external/client-go/swagger/v1alpha4"
)

type firewallRuleResource struct {
	client *swagger.APIClient
}

type firewallRuleResourceModel struct {
	ID               types.String `tfsdk:"id"`
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

	client, ok := req.ProviderData.(*swagger.APIClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", internal.ErrorMsgProviderInitFailed)

		return
	}

	r.client = client
}

func (r *firewallRuleResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vpc_firewall_rule"
}

func (r *firewallRuleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
		},
		"name": schema.StringAttribute{
			Required:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
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
			Validators:    []validator.String{validators.RegexValidator{RegexPattern: "^ingress"}}, // TODO: support egress once supported by API
		},
		"protocols": schema.StringAttribute{
			Required:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			// TODO: add validator
		},
		"source": schema.StringAttribute{
			Required:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			// TODO: add validator
		},
		"source_ports": schema.StringAttribute{
			Required:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			// TODO: add validator
		},
		"destination": schema.StringAttribute{
			Required:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			// TODO: add validator
		},
		"destination_ports": schema.StringAttribute{
			Required:      true,
			PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			// TODO: add validator
		},
	}}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *firewallRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan firewallRuleResourceModel
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

	dataResp, httpResp, err := r.client.VPCFirewallRulesApi.CreateVPCFirewallRule(ctx, swagger.VpcFirewallRulesPostRequest{
		RoleId:           roleID,
		VpcNetworkId:     plan.Network.ValueString(),
		Name:             plan.Name.ValueString(),
		Action:           plan.Action.ValueString(),
		Protocols:        stringToSlice(plan.Protocols.ValueString(), ","),
		Direction:        plan.Direction.ValueString(),
		Sources:          []swagger.FirewallRuleObject{toFirewallRuleObject(plan.Source.ValueString())},
		SourcePorts:      stringToSlice(plan.SourcePorts.ValueString(), ","),
		Destinations:     []swagger.FirewallRuleObject{toFirewallRuleObject(plan.Destination.ValueString())},
		DestinationPorts: stringToSlice(plan.DestinationPorts.ValueString(), ","),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create firewall rule", err.Error())

		return
	}
	defer httpResp.Body.Close()

	firewallRule, _, err := internal.AwaitOperationAndResolve[swagger.VpcFirewallRule](
		ctx, dataResp.Operation,
		r.client.VPCFirewallRuleOperationsApi.GetNetworkingVPCFirewallRulesOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create firewall rule", err.Error())

		return
	}

	plan.ID = types.StringValue(firewallRule.Id)

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

	dataResp, httpResp, err := r.client.VPCFirewallRulesApi.GetVPCFirewallRule(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get firewall rule", "Fetching Crusoe firewall rule info failed.")

		return
	}
	defer httpResp.Body.Close()

	if (len(dataResp.FirewallRules)) == 0 {
		resp.Diagnostics.AddError("Failed to find firewall rule", "No matching firewall rule round. Has the firewall rule been manually deleted?")
	}

	rule := dataResp.FirewallRules[0]
	state.ID = types.StringValue(rule.Id)
	state.Name = types.StringValue(rule.Name)
	state.Network = types.StringValue(rule.VpcNetworkId)
	state.Action = types.StringValue(rule.Action)
	state.Direction = types.StringValue(rule.Direction)
	state.Protocols = types.StringValue(strings.Join(rule.Protocols, ","))
	state.Source = types.StringValue(cidrListToString(rule.Sources))
	state.SourcePorts = types.StringValue(strings.Join(rule.SourcePorts, ","))
	state.Destination = types.StringValue(cidrListToString(rule.Destinations))
	state.DestinationPorts = types.StringValue(strings.Join(rule.DestinationPorts, ","))

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *firewallRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// This should be unreachable, since all properties should be marked as needing replacement on update.
	resp.Diagnostics.AddWarning("Updates not supported",
		"Updating firewall rules is not currently supported. If you're seeing this message, please reach out to support@crusoecloud.com"+
			" and let us know. In the meantime, you should be able to update your rule by deleting it"+
			" and then creating a new one.")
}

//nolint:gocritic // Implements Terraform defined interface
func (r *firewallRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state firewallRuleResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dataResp, httpResp, err := r.client.VPCFirewallRulesApi.DeleteVPCFirewallRule(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete firewall rule", err.Error())

		return
	}
	defer httpResp.Body.Close()

	_, err = internal.AwaitOperation(ctx, dataResp.Operation, r.client.VPCFirewallRuleOperationsApi.GetNetworkingVPCFirewallRulesOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete firewall rule", err.Error())
	}
}

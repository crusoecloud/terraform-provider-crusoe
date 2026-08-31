package firewall_rule

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	validators "github.com/crusoecloud/terraform-provider-crusoe/internal/validators"
)

type firewallRuleResource struct {
	client *common.CrusoeClient
}

type firewallRuleResourceModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
	Network   types.String `tfsdk:"network"`
	Action    types.String `tfsdk:"action"`
	Direction types.String `tfsdk:"direction"`
	Protocols types.String `tfsdk:"protocols"`
	// Sources is the replacement for the deprecated Source field. Exactly one of
	// the two must be set.
	Source      types.String `tfsdk:"source"`
	Sources     types.List   `tfsdk:"sources"`
	SourcePorts types.String `tfsdk:"source_ports"`
	// Destinations is the replacement for the deprecated Destination field. Exactly
	// one of the two must be set.
	Destination      types.String `tfsdk:"destination"`
	Destinations     types.List   `tfsdk:"destinations"`
	DestinationPorts types.String `tfsdk:"destination_ports"`
}

// firewallRuleObjectModel is a single sources/destinations element, mirroring the
// API's FirewallRuleObject: exactly one of the two members is set.
type firewallRuleObjectModel struct {
	CIDR       types.String `tfsdk:"cidr"`
	ResourceID types.String `tfsdk:"resource_id"`
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
				Description:   apiDescID,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: apiDescName,
			},
			"project_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: providerDescProjectID,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"network": schema.StringAttribute{
				Required:      true,
				Description:   apiDescNetwork,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"action": schema.StringAttribute{
				Required:      true,
				Description:   apiDescAction,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validators.RegexValidator{RegexPattern: "^(allow|deny)$"}},
			},
			"direction": schema.StringAttribute{
				Required:      true,
				Description:   apiDescDirection,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{validators.RegexValidator{RegexPattern: "^(ingress|egress)"}},
			},
			"protocols": schema.StringAttribute{
				Required:    true,
				Description: apiDescProtocols,
				// TODO: add validator
			},
			"source": schema.StringAttribute{
				Optional:            true,
				DeprecationMessage:  common.FormatDeprecationWithReplacement("v1.2.0", "sources"),
				MarkdownDescription: providerDescSource,
				Validators: []validator.String{
					// Exactly one of the deprecated field or its replacement must be set.
					stringvalidator.ExactlyOneOf(
						path.MatchRoot("source"),
						path.MatchRoot("sources"),
					),
				},
			},
			"sources": schema.ListNestedAttribute{
				Optional: true,
				MarkdownDescription: apiDescSources + " " + providerDescRuleObjectConstraint + " " +
					providerDescSourceConstraint,
				Validators:   []validator.List{listvalidator.SizeAtLeast(1)},
				NestedObject: ruleObjectNestedAttribute(),
			},
			"source_ports": schema.StringAttribute{
				Required:    true,
				Description: apiDescSourcePorts,
				// TODO: add validator
			},
			"destination": schema.StringAttribute{
				Optional:            true,
				DeprecationMessage:  common.FormatDeprecationWithReplacement("v1.2.0", "destinations"),
				MarkdownDescription: providerDescDestination,
				Validators: []validator.String{
					// Exactly one of the deprecated field or its replacement must be set.
					stringvalidator.ExactlyOneOf(
						path.MatchRoot("destination"),
						path.MatchRoot("destinations"),
					),
				},
			},
			"destinations": schema.ListNestedAttribute{
				Optional: true,
				MarkdownDescription: apiDescDestinations + " " + providerDescRuleObjectConstraint + " " +
					providerDescDestinationConstraint,
				Validators:   []validator.List{listvalidator.SizeAtLeast(1)},
				NestedObject: ruleObjectNestedAttribute(),
			},
			"destination_ports": schema.StringAttribute{
				Required:    true,
				Description: apiDescDestinationPorts,
				// TODO: add validator
			},
		},
	}
}

func (r *firewallRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceID, projectID, errMsg := common.ParseResourceIdentifiers(req, r.client, "firewall_rule_id")
	if errMsg != "" {
		resp.Diagnostics.AddError("Failed to import Firewall Rule", errMsg)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), resourceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *firewallRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan firewallRuleResourceModel
	if err := common.GetResourceModel(ctx, req.Plan, &plan, &resp.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())

	sourcePortsStr := strings.ReplaceAll(plan.SourcePorts.ValueString(), "*", "1-65535")
	destPortsStr := strings.ReplaceAll(plan.DestinationPorts.ValueString(), "*", "1-65535")

	sources := ruleObjectsFromModel(ctx, plan.Sources, plan.Source, &resp.Diagnostics)
	destinations := ruleObjectsFromModel(ctx, plan.Destinations, plan.Destination, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	dataResp, httpResp, err := r.client.APIClient.VPCFirewallRulesApi.CreateVPCFirewallRule(ctx, swagger.VpcFirewallRulesPostRequestV1{
		VpcNetworkId:     plan.Network.ValueString(),
		Name:             plan.Name.ValueString(),
		Action:           plan.Action.ValueString(),
		Protocols:        stringToSlice(plan.Protocols.ValueString(), ","),
		Direction:        plan.Direction.ValueString(),
		Sources:          sources,
		SourcePorts:      stringToSlice(sourcePortsStr, ","),
		Destinations:     destinations,
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

	firewallRuleToTerraformResourceModel(ctx, firewallRule, &plan)
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *firewallRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state firewallRuleResourceModel
	if err := common.GetResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	// We only have this parsing for transitioning from v1alpha4 to V1 because old tf state files will not
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
	firewallRuleToTerraformResourceModel(ctx, &rule, &state)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *firewallRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state firewallRuleResourceModel
	if err := common.GetResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	var plan firewallRuleResourceModel
	if err := common.GetResourceModel(ctx, req.Plan, &plan, &resp.Diagnostics); err != nil {
		return
	}
	patchReq := swagger.VpcFirewallRulesPatchRequest{}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		patchReq.Name = plan.Name.ValueString()
	}
	if !plan.Protocols.IsNull() && !plan.Protocols.IsUnknown() {
		patchReq.Protocols = stringToSlice(plan.Protocols.ValueString(), ",")
	}
	if isSet(plan.Destination) || isSetList(plan.Destinations) {
		patchReq.Destinations = ruleObjectsFromModel(ctx, plan.Destinations, plan.Destination, &resp.Diagnostics)
	}
	if !plan.DestinationPorts.IsNull() && !plan.DestinationPorts.IsUnknown() {
		patchReq.DestinationPorts = stringToSlice(plan.DestinationPorts.ValueString(), ",")
	}
	if isSet(plan.Source) || isSetList(plan.Sources) {
		patchReq.Sources = ruleObjectsFromModel(ctx, plan.Sources, plan.Source, &resp.Diagnostics)
	}
	if !plan.SourcePorts.IsNull() && !plan.SourcePorts.IsUnknown() {
		patchReq.SourcePorts = stringToSlice(plan.SourcePorts.ValueString(), ",")
	}
	if resp.Diagnostics.HasError() {
		return
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

	firewallRule, _, err := common.AwaitOperationAndResolve[swagger.VpcFirewallRule](ctx, dataResp.Operation, plan.ProjectID.ValueString(), r.client.APIClient.VPCFirewallRuleOperationsApi.GetNetworkingVPCFirewallRulesOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to patch firewall rule",
			fmt.Sprintf("There was an error updating the firewall rule: %s.", common.UnpackAPIError(err)))

		return
	}

	firewallRuleToTerraformResourceModel(ctx, firewallRule, &plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *firewallRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state firewallRuleResourceModel
	if err := common.GetResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
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

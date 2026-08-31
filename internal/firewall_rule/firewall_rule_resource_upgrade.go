package firewall_rule

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// firewallRuleResourceModelV0 is the minimal set of attributes that we will need from a prior state to
// rebuild a firewall rule's state because these fields are not returned by the API.
//
// A v0 configuration can only use the combined source/destination fields, since the
// sources/destinations lists that replaced them did not exist yet. Carrying the two
// values over keeps the upgraded state on the same fields the configuration uses, so
// the first plan after the upgrade does not report a move to the new lists.
type firewallRuleResourceModelV0 struct {
	ID          types.String `tfsdk:"id"`
	Source      types.String `tfsdk:"source"`
	Destination types.String `tfsdk:"destination"`
}

func (r *firewallRuleResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed: true,
					},
					"source": schema.StringAttribute{
						Optional: true,
					},
					"destination": schema.StringAttribute{
						Optional: true,
					},
				},
			},
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var priorStateData firewallRuleResourceModelV0
				resp.Diagnostics.Append(req.State.Get(ctx, &priorStateData)...)

				if resp.Diagnostics.HasError() {
					return
				}

				if priorStateData.ID.IsNull() {
					resp.Diagnostics.AddError("Failed to migrate firewall rule to current version",
						"No ID was associated with the firewall rule.")

					return
				}

				// Note: we will iterate through all projects to find the firewall rule. This means we are not dependent
				// on project ID being present in the previous state, which allows us to be backwards-compatible with
				// more versions.
				firewallRule, projectID, err := findFirewallRule(ctx, r.client.APIClient, priorStateData.ID.ValueString())
				if err != nil {
					resp.Diagnostics.AddError("Failed to migrate firewall rule to current version",
						fmt.Sprintf("There was an error migrating the firewall rule to the current version: %v",
							err))

					return
				}

				state := firewallRuleResourceModel{
					ProjectID:   types.StringValue(projectID),
					Source:      priorStateData.Source,
					Destination: priorStateData.Destination,
				}
				firewallRuleToTerraformResourceModel(ctx, firewallRule, &state)
				resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
				if resp.Diagnostics.HasError() {
					resp.Diagnostics.AddError("Failed to migrate firewall rule to current version",
						"There was an error migrating the firewall rule to the current version.")

					return
				}
				resp.Diagnostics.AddWarning("Successfully migrated firewall rule to current version",
					"Terraform State has been successfully migrated to a new version. Please refer to"+
						" docs.crusoecloud.com for information about the updates.")
			},
		},
	}
}

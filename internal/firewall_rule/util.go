package firewall_rule

import (
	"context"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"
	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

var whitespaceRegex = regexp.MustCompile(`\s*`)

// cidrListToString converts a list of CIDRs to a comma separated string.
func cidrListToString(ruleObjects []swagger.FirewallRuleObject) string {
	out := ""
	numObjects := len(ruleObjects)
	for i := range ruleObjects {
		out += ruleObjects[i].Cidr
		if i < numObjects-1 {
			out += ","
		}
	}

	return out
}

// toFirewallRuleObject wraps an IP or CIDR string into a FirewallRuleObject.
func toFirewallRuleObject(ipOrCIDR string) swagger.FirewallRuleObject {
	return swagger.FirewallRuleObject{Cidr: ipOrCIDR}
}

// stringToSlice splits a delimited string list into a slice of strings.
func stringToSlice(s, delimiter string) []string {
	s = whitespaceRegex.ReplaceAllString(s, "")
	if s == "" {
		return []string{}
	}

	elems := strings.Split(s, delimiter)

	return elems
}

func findFirewallRule(ctx context.Context, client *swagger.APIClient, firewallRuleID string) (*swagger.VpcFirewallRule, string, error) {
	args := common.FindResourceArgs[swagger.VpcFirewallRule]{
		ResourceID:  firewallRuleID,
		GetResource: client.VPCFirewallRulesApi.GetVPCFirewallRule,
		IsResource: func(rule swagger.VpcFirewallRule, id string) bool {
			return rule.Id == id
		},
	}

	return common.FindResource[swagger.VpcFirewallRule](ctx, client, args)
}

func firewallRuleToTerraformResourceModel(rule *swagger.VpcFirewallRule, state *firewallRuleResourceModel) {
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
}

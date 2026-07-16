package firewall_rule

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec (VpcFirewallRule).
const (
	apiDescID               = "ID of the firewall rule."
	apiDescName             = "Name of the firewall rule."
	apiDescNetwork          = "ID of the VPC network the rule belongs to."
	apiDescAction           = "Action applied to traffic that matches the rule. Possible values: `allow`, `deny`."
	apiDescDirection        = "Direction of traffic the rule applies to. Possible values: `ingress` (inbound), `egress` (outbound)."
	apiDescProtocols        = "Network protocols the rule matches (for example, `tcp`, `udp`)."
	apiDescSource           = "Sources the rule matches, given as CIDR blocks or resource IDs."
	apiDescSourcePorts      = "Source ports the rule matches. Each entry is a single port or a port range (for example, `3000-8080`)."
	apiDescDestination      = "Destinations the rule matches, given as CIDR blocks or resource IDs."
	apiDescDestinationPorts = "Destination ports the rule matches. Each entry is a single port or a port range (for example, `3000-8080`)."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project the firewall rule belongs to. " + project.ProviderDescProjectIDFallback
)

var whitespaceRegex = regexp.MustCompile(`\s*`)

// wildcardPortRange is the canonical range the API expands the "*" port
// wildcard into.
const wildcardPortRange = "1-65535"

// cidrList extracts the CIDR strings from a list of FirewallRuleObjects.
func cidrList(ruleObjects []swagger.FirewallRuleObject) []string {
	out := make([]string, 0, len(ruleObjects))
	for i := range ruleObjects {
		out = append(out, ruleObjects[i].Cidr)
	}

	return out
}

// cidrListToString converts a list of CIDRs to a comma separated string.
func cidrListToString(ruleObjects []swagger.FirewallRuleObject) string {
	return strings.Join(cidrList(ruleObjects), ",")
}

// canonicalizeList normalizes a set of comma-separated values for comparison:
// it trims whitespace, drops empty elements, expands the "*" port wildcard to
// the range the API uses (when expandWildcard is set), and sorts so the
// comparison is order-insensitive.
func canonicalizeList(elems []string, expandWildcard bool) []string {
	out := make([]string, 0, len(elems))
	for _, e := range elems {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if expandWildcard && e == "*" {
			e = wildcardPortRange
		}
		out = append(out, e)
	}
	// An omitted/empty port list means "all ports", which the backend
	// materializes as the full range — treat it the same as an explicit
	// "1-65535" or "*". Only applies to ports (expandWildcard).
	if expandWildcard && len(out) == 0 {
		out = append(out, wildcardPortRange)
	}
	slices.Sort(out)

	return out
}

// listsSemanticallyEqual reports whether a configured comma-separated string and
// the slice the API returned describe the same set of values, ignoring order and
// whitespace (treating "*" as the full port range when expandWildcard is set).
func listsSemanticallyEqual(configured string, apiElems []string, expandWildcard bool) bool {
	return slices.Equal(
		canonicalizeList(stringToSlice(configured, ","), expandWildcard),
		canonicalizeList(apiElems, expandWildcard),
	)
}

// preserveListFormat keeps the user's configured representation when it is
// semantically equal to what the API returned, so cosmetic differences (e.g.
// "*" vs "1-65535", reordered elements) don't produce spurious diffs on these
// Required attributes. Otherwise it returns the API value joined with commas.
func preserveListFormat(configured string, apiElems []string, expandWildcard bool) string {
	if listsSemanticallyEqual(configured, apiElems, expandWildcard) {
		return configured
	}

	return strings.Join(apiElems, ",")
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
	// protocols, source(_ports) and destination(_ports) are Required attributes
	// the API may return in a normalized form (e.g. "*"/"" → "1-65535", reordered
	// lists). Preserve the user's configured representation when it is
	// semantically equal so reads don't produce spurious diffs and creates/updates
	// don't fail with "inconsistent result after apply".
	state.Protocols = types.StringValue(preserveListFormat(state.Protocols.ValueString(), rule.Protocols, false))
	state.Source = types.StringValue(preserveListFormat(state.Source.ValueString(), cidrList(rule.Sources), false))
	state.SourcePorts = types.StringValue(preserveListFormat(state.SourcePorts.ValueString(), rule.SourcePorts, true))
	state.Destination = types.StringValue(preserveListFormat(state.Destination.ValueString(), cidrList(rule.Destinations), false))
	state.DestinationPorts = types.StringValue(preserveListFormat(state.DestinationPorts.ValueString(), rule.DestinationPorts, true))
}

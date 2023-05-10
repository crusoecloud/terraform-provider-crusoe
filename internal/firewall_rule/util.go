package firewall_rule

import (
	"regexp"
	"strings"

	swagger "gitlab.com/crusoeenergy/island/external/client-go/swagger/v1alpha4"
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

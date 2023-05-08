package firewall_rule

import (
	"regexp"
	"strings"

	swagger "gitlab.com/crusoeenergy/island/external/client-go/swagger/v1alpha4"
)

var whitespaceRegex = regexp.MustCompile(`\s`)

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

func toFirewallRuleObject(ipOrCIDR string) swagger.FirewallRuleObject {
	return swagger.FirewallRuleObject{Cidr: ipOrCIDR}
}

// stringToSlice splits a delimited string list into a slice of strings.
func stringToSlice(s, delimiter string) []string {
	whitespaceRegex.ReplaceAllString(s, "")
	return strings.Split(s, delimiter)
}

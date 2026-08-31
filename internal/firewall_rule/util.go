package firewall_rule

import (
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec
// (VpcFirewallRule, FirewallRuleObject).
const (
	apiDescID               = "ID of the firewall rule."
	apiDescName             = "Name of the firewall rule."
	apiDescNetwork          = "ID of the VPC network the rule belongs to."
	apiDescAction           = "Action applied to traffic that matches the rule. Possible values: `allow`, `deny`."
	apiDescDirection        = "Direction of traffic the rule applies to. Possible values: `ingress` (inbound), `egress` (outbound)."
	apiDescProtocols        = "Network protocols the rule matches (for example, `tcp`, `udp`)."
	apiDescSources          = "Sources the rule matches, given as CIDR blocks or resource IDs."
	apiDescSourcePorts      = "Source ports the rule matches. Each entry is a single port or a port range (for example, `3000-8080`)."
	apiDescDestinations     = "Destinations the rule matches, given as CIDR blocks or resource IDs."
	apiDescDestinationPorts = "Destination ports the rule matches. Each entry is a single port or a port range (for example, `3000-8080`)."
	apiDescCIDR             = "CIDR block, or an IP address that is converted to a CIDR. Mutually exclusive with `resource_id`."
	apiDescResourceID       = "ID of a VPC network, subnet, or VM. Mutually exclusive with `cidr`."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project the firewall rule belongs to. " + project.ProviderDescProjectIDFallback

	// The combined source/destination fields are ambiguous — they accept either a
	// CIDR or a resource ID — and only ever describe a single entry. The
	// sources/destinations lists that mirror the API replace them.
	providerDescSource      = "Source of the firewall rule, as a CIDR or IP address. Deprecated in favor of `sources`. " + providerDescSourceConstraint
	providerDescDestination = "Destination of the firewall rule, as a CIDR or IP address. Deprecated in favor of `destinations`. " + providerDescDestinationConstraint

	// The source/destination fields are each declared Optional because the framework
	// cannot mark a field both Optional and Required; the ExactlyOneOf validators
	// enforce the real "exactly one required" constraints, which these notes document
	// since tfplugindocs lists the fields under "Optional".
	providerDescSourceConstraint      = "Exactly one of `source` or `sources` must be set."
	providerDescDestinationConstraint = "Exactly one of `destination` or `destinations` must be set."
	providerDescRuleObjectConstraint  = "Exactly one of `cidr` or `resource_id` must be set on each element."
)

// firewallRuleObjectType is the Terraform type of a single sources/destinations
// element, mirroring the API's FirewallRuleObject.
var firewallRuleObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"cidr":        types.StringType,
		"resource_id": types.StringType,
	},
}

// ruleObjectNestedAttribute is the shared shape of a sources/destinations element.
func ruleObjectNestedAttribute() schema.NestedAttributeObject {
	return schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"cidr": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: apiDescCIDR,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					// Rejects an element that sets both cidr and resource_id, or neither.
					// Declared on one of the two attributes so a violation is reported once.
					stringvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("cidr"),
						path.MatchRelative().AtParent().AtName("resource_id"),
					),
				},
			},
			"resource_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: apiDescResourceID,
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
		},
	}
}

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

// isSet reports whether a string attribute has a usable (non-null, non-unknown, non-empty) value.
func isSet(s types.String) bool {
	return !s.IsNull() && !s.IsUnknown() && s.ValueString() != ""
}

// isSetList reports whether a list attribute has a usable (non-null, non-unknown, non-empty) value.
func isSetList(l types.List) bool {
	return !l.IsNull() && !l.IsUnknown() && len(l.Elements()) > 0
}

// nullIfEmpty maps an empty API string to a null Terraform value, so the member of
// a rule object the API left unset stays null rather than becoming "".
func nullIfEmpty(s string) types.String {
	if s == "" {
		return types.StringNull()
	}

	return types.StringValue(s)
}

// canonicalCIDR normalizes a CIDR for comparison: the API converts a bare IP
// address into a single-address CIDR (e.g. "10.1.2.3" -> "10.1.2.3/32"), which
// would otherwise read as an out-of-band change.
func canonicalCIDR(cidr string) string {
	cidr = strings.TrimSpace(cidr)
	if cidr == "" || strings.Contains(cidr, "/") {
		return cidr
	}
	if strings.Contains(cidr, ":") {
		return cidr + "/128"
	}

	return cidr + "/32"
}

// ruleObjectModels converts a sources/destinations list into its element models.
func ruleObjectModels(ctx context.Context, list types.List) ([]firewallRuleObjectModel, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return nil, nil
	}

	var elems []firewallRuleObjectModel
	diags := list.ElementsAs(ctx, &elems, false)

	return elems, diags
}

// ruleObjectsFromModel converts a configured sources/destinations list into API rule
// objects, falling back to the deprecated combined field when the list is unset. The
// schema's ExactlyOneOf validators guarantee that exactly one of the two is configured
// and that each element sets exactly one of cidr/resource_id, which is all a
// FirewallRuleObject accepts.
func ruleObjectsFromModel(ctx context.Context, list types.List, deprecated types.String,
	diags *diag.Diagnostics,
) []swagger.FirewallRuleObject {
	if !isSetList(list) {
		// The deprecated field holds a comma-separated list of CIDRs, matching how it
		// is read back from the API (CCX-5403).
		return toFirewallRuleObjects(stringToSlice(deprecated.ValueString(), ","))
	}

	elems, elemDiags := ruleObjectModels(ctx, list)
	diags.Append(elemDiags...)

	objects := make([]swagger.FirewallRuleObject, 0, len(elems))
	for i := range elems {
		objects = append(objects, swagger.FirewallRuleObject{
			Cidr:       elems[i].CIDR.ValueString(),
			ResourceId: elems[i].ResourceID.ValueString(),
		})
	}

	return objects
}

// preserveConfiguredRuleObjects reports whether the configured rule objects still
// describe what the API returned, in which case they are kept as-is so that a
// difference in representation alone — a reordered list, or a bare IP the API
// stores as a single-address CIDR — does not read as a change. A real change to
// the set of targets does not survive the comparison, so drift still surfaces.
func preserveConfiguredRuleObjects(configured []firewallRuleObjectModel, objects []swagger.FirewallRuleObject) bool {
	if len(configured) == 0 {
		return false
	}

	configuredKeys := make([]string, 0, len(configured))
	for i := range configured {
		configuredKeys = append(configuredKeys,
			ruleObjectKey(configured[i].ResourceID.ValueString(), configured[i].CIDR.ValueString()))
	}

	apiKeys := make([]string, 0, len(objects))
	for i := range objects {
		apiKeys = append(apiKeys, ruleObjectKey(objects[i].ResourceId, objects[i].Cidr))
	}

	slices.Sort(configuredKeys)
	slices.Sort(apiKeys)

	return slices.Equal(configuredKeys, apiKeys)
}

// ruleObjectKey reduces a rule object to the member that identifies it, so a
// configured entry and the API's copy of it compare equal. The API returns a
// resource_id unchanged (verified against subnet and VM references), which is what
// lets a reference be compared rather than held.
func ruleObjectKey(resourceID, cidr string) string {
	if resourceID != "" {
		return "id:" + resourceID
	}

	return "cidr:" + canonicalCIDR(cidr)
}

// ruleObjectsToState maps API rule objects onto a sources/destinations list.
func ruleObjectsToState(ctx context.Context, objects []swagger.FirewallRuleObject, configured types.List) types.List {
	elems, _ := ruleObjectModels(ctx, configured)
	if preserveConfiguredRuleObjects(elems, objects) {
		return configured
	}

	models := make([]firewallRuleObjectModel, 0, len(objects))
	for i := range objects {
		models = append(models, firewallRuleObjectModel{
			CIDR:       nullIfEmpty(objects[i].Cidr),
			ResourceID: nullIfEmpty(objects[i].ResourceId),
		})
	}

	list, diags := types.ListValueFrom(ctx, firewallRuleObjectType, models)
	if diags.HasError() {
		return types.ListNull(firewallRuleObjectType)
	}

	return list
}

// applyRuleObjectsToState maps API rule objects back onto whichever of the two
// mutually-exclusive fields the configuration uses, so the deprecated field and the
// list never both end up populated (which would trip ExactlyOneOf and cause a
// perpetual diff).
func applyRuleObjectsToState(ctx context.Context, objects []swagger.FirewallRuleObject,
	list *types.List, deprecated *types.String,
) {
	if isSet(*deprecated) {
		// The deprecated field only ever carries CIDRs, which is what the API returns
		// for the rule objects it created from them.
		*deprecated = types.StringValue(preserveListFormat(deprecated.ValueString(), cidrList(objects), false))
		*list = types.ListNull(firewallRuleObjectType)

		return
	}

	*list = ruleObjectsToState(ctx, objects, *list)
}

// toFirewallRuleObjects wraps IP or CIDR strings into FirewallRuleObjects.
func toFirewallRuleObjects(ipsOrCIDRs []string) []swagger.FirewallRuleObject {
	out := make([]swagger.FirewallRuleObject, 0, len(ipsOrCIDRs))
	for _, ipOrCIDR := range ipsOrCIDRs {
		out = append(out, swagger.FirewallRuleObject{Cidr: ipOrCIDR})
	}

	return out
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

func firewallRuleToTerraformResourceModel(ctx context.Context, rule *swagger.VpcFirewallRule, state *firewallRuleResourceModel) {
	state.ID = types.StringValue(rule.Id)
	state.Name = types.StringValue(rule.Name)
	state.Network = types.StringValue(rule.VpcNetworkId)
	state.Action = types.StringValue(rule.Action)
	state.Direction = types.StringValue(rule.Direction)
	// protocols and the port lists are Required attributes the API may return in a
	// normalized form (e.g. "*"/"" → "1-65535", reordered lists). Preserve the user's
	// configured representation when it is semantically equal so reads don't produce
	// spurious diffs and creates/updates don't fail with "inconsistent result after apply".
	state.Protocols = types.StringValue(preserveListFormat(state.Protocols.ValueString(), rule.Protocols, false))
	applyRuleObjectsToState(ctx, rule.Sources, &state.Sources, &state.Source)
	state.SourcePorts = types.StringValue(preserveListFormat(state.SourcePorts.ValueString(), rule.SourcePorts, true))
	applyRuleObjectsToState(ctx, rule.Destinations, &state.Destinations, &state.Destination)
	state.DestinationPorts = types.StringValue(preserveListFormat(state.DestinationPorts.ValueString(), rule.DestinationPorts, true))
}

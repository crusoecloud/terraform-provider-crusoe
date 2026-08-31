package firewall_rule

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

// cidrObject and resourceIDObject build a single sources/destinations element of
// each of the two mutually-exclusive shapes.
func cidrObject(cidr string) firewallRuleObjectModel {
	return firewallRuleObjectModel{CIDR: types.StringValue(cidr), ResourceID: types.StringNull()}
}

func resourceIDObject(resourceID string) firewallRuleObjectModel {
	return firewallRuleObjectModel{CIDR: types.StringNull(), ResourceID: types.StringValue(resourceID)}
}

func ruleObjectList(t *testing.T, objects ...firewallRuleObjectModel) types.List {
	t.Helper()

	list, diags := types.ListValueFrom(context.Background(), firewallRuleObjectType, objects)
	if diags.HasError() {
		t.Fatalf("failed to build rule object list: %v", diags)
	}

	return list
}

func Test_ruleObjectsFromModel(t *testing.T) {
	tests := []struct {
		name       string
		list       []firewallRuleObjectModel
		deprecated types.String
		want       []swagger.FirewallRuleObject
	}{
		{
			name: "mixed cidr and resource id elements",
			list: []firewallRuleObjectModel{cidrObject("10.0.0.0/16"), resourceIDObject("vpc-123")},
			want: []swagger.FirewallRuleObject{{Cidr: "10.0.0.0/16"}, {ResourceId: "vpc-123"}},
		},
		{
			name:       "list wins over deprecated field",
			list:       []firewallRuleObjectModel{cidrObject("10.0.0.0/16")},
			deprecated: types.StringValue("0.0.0.0/0"),
			want:       []swagger.FirewallRuleObject{{Cidr: "10.0.0.0/16"}},
		},
		{
			name:       "falls back to deprecated field",
			deprecated: types.StringValue("0.0.0.0/0"),
			want:       []swagger.FirewallRuleObject{{Cidr: "0.0.0.0/0"}},
		},
		{
			name:       "deprecated field splits comma-separated cidrs",
			deprecated: types.StringValue("10.0.0.0/16,10.1.0.0/16"),
			want:       []swagger.FirewallRuleObject{{Cidr: "10.0.0.0/16"}, {Cidr: "10.1.0.0/16"}},
		},
		{
			name: "nothing set",
			want: []swagger.FirewallRuleObject{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := types.ListNull(firewallRuleObjectType)
			if tt.list != nil {
				list = ruleObjectList(t, tt.list...)
			}
			deprecated := tt.deprecated
			if deprecated.IsNull() {
				deprecated = types.StringNull()
			}

			var diags diag.Diagnostics
			got := ruleObjectsFromModel(context.Background(), list, deprecated, &diags)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ruleObjectsFromModel() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func Test_applyRuleObjectsToState(t *testing.T) {
	tests := []struct {
		name       string
		list       []firewallRuleObjectModel
		deprecated types.String
		objects    []swagger.FirewallRuleObject
		wantList   []firewallRuleObjectModel
		wantDeprec types.String
	}{
		{
			name:       "deprecated field updated from api",
			deprecated: types.StringValue("0.0.0.0/0"),
			objects:    []swagger.FirewallRuleObject{{Cidr: "0.0.0.0/0"}},
			wantDeprec: types.StringValue("0.0.0.0/0"),
		},
		{
			name:       "deprecated field reflects api change",
			deprecated: types.StringValue("10.0.0.0/16"),
			objects:    []swagger.FirewallRuleObject{{Cidr: "0.0.0.0/0"}},
			wantDeprec: types.StringValue("0.0.0.0/0"),
		},
		{
			name:     "configured cidrs preserved when api agrees",
			list:     []firewallRuleObjectModel{cidrObject("10.0.0.0/16"), cidrObject("10.1.0.0/16")},
			objects:  []swagger.FirewallRuleObject{{Cidr: "10.1.0.0/16"}, {Cidr: "10.0.0.0/16"}}, // reordered
			wantList: []firewallRuleObjectModel{cidrObject("10.0.0.0/16"), cidrObject("10.1.0.0/16")},
		},
		{
			name:     "bare ip preserved against api /32 conversion",
			list:     []firewallRuleObjectModel{cidrObject("10.1.2.3")},
			objects:  []swagger.FirewallRuleObject{{Cidr: "10.1.2.3/32"}},
			wantList: []firewallRuleObjectModel{cidrObject("10.1.2.3")},
		},
		{
			name:     "configured cidrs replaced on genuine change",
			list:     []firewallRuleObjectModel{cidrObject("10.0.0.0/16")},
			objects:  []swagger.FirewallRuleObject{{Cidr: "0.0.0.0/0"}},
			wantList: []firewallRuleObjectModel{cidrObject("0.0.0.0/0")},
		},
		{
			name:     "resource id round-trips unchanged",
			list:     []firewallRuleObjectModel{resourceIDObject("vpc-123")},
			objects:  []swagger.FirewallRuleObject{{ResourceId: "vpc-123"}},
			wantList: []firewallRuleObjectModel{resourceIDObject("vpc-123")},
		},
		{
			name:     "mixed cidr and resource id entries preserved when api agrees",
			list:     []firewallRuleObjectModel{cidrObject("10.4.0.0/16"), resourceIDObject("subnet-1")},
			objects:  []swagger.FirewallRuleObject{{ResourceId: "subnet-1"}, {Cidr: "10.4.0.0/16"}}, // reordered
			wantList: []firewallRuleObjectModel{cidrObject("10.4.0.0/16"), resourceIDObject("subnet-1")},
		},
		{
			// The API returns a resource_id unchanged, so a CIDR coming back in its place
			// is a real out-of-band edit rather than the reference being resolved.
			name:     "resource id replaced when the api reports a different target",
			list:     []firewallRuleObjectModel{resourceIDObject("vpc-123")},
			objects:  []swagger.FirewallRuleObject{{Cidr: "10.0.0.0/16"}},
			wantList: []firewallRuleObjectModel{cidrObject("10.0.0.0/16")},
		},
		{
			name:     "import populates list from api",
			objects:  []swagger.FirewallRuleObject{{ResourceId: "vpc-123"}, {Cidr: "10.0.0.0/16"}},
			wantList: []firewallRuleObjectModel{resourceIDObject("vpc-123"), cidrObject("10.0.0.0/16")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			list := types.ListNull(firewallRuleObjectType)
			if tt.list != nil {
				list = ruleObjectList(t, tt.list...)
			}
			deprecated := tt.deprecated

			applyRuleObjectsToState(context.Background(), tt.objects, &list, &deprecated)

			wantList := types.ListNull(firewallRuleObjectType)
			if tt.wantList != nil {
				wantList = ruleObjectList(t, tt.wantList...)
			}
			if !list.Equal(wantList) {
				t.Errorf("list = %v, want %v", list, wantList)
			}
			if !deprecated.Equal(tt.wantDeprec) {
				t.Errorf("deprecated = %v, want %v", deprecated, tt.wantDeprec)
			}
		})
	}
}

func Test_canonicalCIDR(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"10.0.0.0/16", "10.0.0.0/16"},
		{"10.1.2.3", "10.1.2.3/32"},
		{" 10.1.2.3 ", "10.1.2.3/32"},
		{"2001:db8::1", "2001:db8::1/128"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := canonicalCIDR(tt.in); got != tt.want {
				t.Errorf("canonicalCIDR(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// Test_toFirewallRuleObjects covers CCX-5403: comma-separated source/destination
// strings must map to one FirewallRuleObject per CIDR, not a single object
// holding the raw comma-separated string.
func Test_toFirewallRuleObjects(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []swagger.FirewallRuleObject
	}{
		{
			name: "empty list",
			in:   []string{},
			want: []swagger.FirewallRuleObject{},
		},
		{
			name: "single CIDR",
			in:   []string{"127.0.0.0/16"},
			want: []swagger.FirewallRuleObject{{Cidr: "127.0.0.0/16"}},
		},
		{
			name: "multiple CIDRs",
			in:   []string{"1.1.1.1/32", "2.2.2.2/32"},
			want: []swagger.FirewallRuleObject{{Cidr: "1.1.1.1/32"}, {Cidr: "2.2.2.2/32"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toFirewallRuleObjects(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("toFirewallRuleObjects(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// Test_toFirewallRuleObjects_splitsCommaSeparatedList exercises the full
// write-path composition used in Create/Update.
func Test_toFirewallRuleObjects_splitsCommaSeparatedList(t *testing.T) {
	got := toFirewallRuleObjects(stringToSlice("1.1.1.1/32, 2.2.2.2/32", ","))
	want := []swagger.FirewallRuleObject{{Cidr: "1.1.1.1/32"}, {Cidr: "2.2.2.2/32"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func Test_stringToSlice(t *testing.T) {
	type args struct {
		s         string
		delimiter string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "empty string",
			args: args{s: "", delimiter: ","},
			want: []string{},
		},
		{
			name: "single element",
			args: args{s: "asd", delimiter: ","},
			want: []string{"asd"},
		},
		{
			name: "multiple elements, no space",
			args: args{s: "asd,dsa", delimiter: ","},
			want: []string{"asd", "dsa"},
		},
		{
			name: "multiple elements, some spaces",
			args: args{s: "asd,dsa, qwe", delimiter: ","},
			want: []string{"asd", "dsa", "qwe"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stringToSlice(tt.args.s, tt.args.delimiter); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("stringToSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_preserveListFormat(t *testing.T) {
	tests := []struct {
		name           string
		configured     string
		apiElems       []string
		expandWildcard bool
		want           string
	}{
		{"wildcard preserved against expanded range", "*", []string{"1-65535"}, true, "*"},
		{"explicit range round-trips", "1-65535", []string{"1-65535"}, true, "1-65535"},
		{"reordered ports keep configured order", "80,443", []string{"443", "80"}, true, "80,443"},
		{"configured whitespace preserved when equal", "80, 443", []string{"80", "443"}, true, "80, 443"},
		{"genuine port change uses API value", "80", []string{"443"}, true, "443"},
		{"omitted ports equal backend full range", "", []string{"1-65535"}, true, ""},
		{"empty non-port list not treated as range", "", []string{"1-65535"}, false, "1-65535"},
		{"reordered protocols keep configured order", "tcp,udp", []string{"udp", "tcp"}, false, "tcp,udp"},
		{"changed CIDR uses API value", "10.0.0.0/8", []string{"0.0.0.0/0"}, false, "0.0.0.0/0"},
		{"wildcard not expanded without flag", "*", []string{"1-65535"}, false, "1-65535"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := preserveListFormat(tt.configured, tt.apiElems, tt.expandWildcard); got != tt.want {
				t.Errorf("preserveListFormat(%q, %v, %v) = %q, want %q",
					tt.configured, tt.apiElems, tt.expandWildcard, got, tt.want)
			}
		})
	}
}

// Test_firewallRuleToTerraformResourceModel_preservesConfiguredFormat is the
// regression guard for CCX-4493: a user-configured "*" must survive the
// transform even though the API expands it to "1-65535".
func Test_firewallRuleToTerraformResourceModel_preservesConfiguredFormat(t *testing.T) {
	state := &firewallRuleResourceModel{
		Protocols:        types.StringValue("tcp"),
		Source:           types.StringValue("0.0.0.0/0"),
		SourcePorts:      types.StringValue("*"),
		Destination:      types.StringValue("0.0.0.0/0"),
		DestinationPorts: types.StringValue("*"),
	}
	rule := &swagger.VpcFirewallRule{
		Id:               "fw-1",
		Name:             "rule",
		VpcNetworkId:     "net-1",
		Action:           "allow",
		Direction:        "ingress",
		Protocols:        []string{"tcp"},
		Sources:          []swagger.FirewallRuleObject{{Cidr: "0.0.0.0/0"}},
		SourcePorts:      []string{wildcardPortRange},
		Destinations:     []swagger.FirewallRuleObject{{Cidr: "0.0.0.0/0"}},
		DestinationPorts: []string{wildcardPortRange},
	}

	firewallRuleToTerraformResourceModel(context.Background(), rule, state)

	if got := state.SourcePorts.ValueString(); got != "*" {
		t.Errorf("source_ports = %q, want %q (preserved)", got, "*")
	}
	if got := state.DestinationPorts.ValueString(); got != "*" {
		t.Errorf("destination_ports = %q, want %q (preserved)", got, "*")
	}
	if got := state.ID.ValueString(); got != "fw-1" {
		t.Errorf("id = %q, want %q (from API)", got, "fw-1")
	}
	// The deprecated fields are in use, so their replacements must stay null rather
	// than being populated from the API response.
	if !state.Sources.IsNull() {
		t.Errorf("sources = %v, want null while `source` is configured", state.Sources)
	}
	if !state.Destinations.IsNull() {
		t.Errorf("destinations = %v, want null while `destination` is configured", state.Destinations)
	}
}

// Test_firewallRuleToTerraformResourceModel_sources covers the same transform for
// the sources/destinations lists that replaced the combined fields.
func Test_firewallRuleToTerraformResourceModel_sources(t *testing.T) {
	state := &firewallRuleResourceModel{
		Sources:      ruleObjectList(t, cidrObject("0.0.0.0/0")),
		Destinations: ruleObjectList(t, resourceIDObject("vm-1")),
	}
	rule := &swagger.VpcFirewallRule{
		Sources: []swagger.FirewallRuleObject{{Cidr: "0.0.0.0/0"}},
		// The API returns a VM reference unchanged rather than resolving it.
		Destinations: []swagger.FirewallRuleObject{{ResourceId: "vm-1"}},
	}

	firewallRuleToTerraformResourceModel(context.Background(), rule, state)

	if want := ruleObjectList(t, cidrObject("0.0.0.0/0")); !state.Sources.Equal(want) {
		t.Errorf("sources = %v, want %v", state.Sources, want)
	}
	if want := ruleObjectList(t, resourceIDObject("vm-1")); !state.Destinations.Equal(want) {
		t.Errorf("destinations = %v, want %v (resource reference round-trips)", state.Destinations, want)
	}
	if !state.Source.IsNull() || !state.Destination.IsNull() {
		t.Error("deprecated source/destination should stay null when the lists are configured")
	}
}

// Test_firewallRuleToTerraformResourceModel_reflectsAPIChange confirms a genuine
// out-of-band change is still surfaced (preserve only applies when equal).
func Test_firewallRuleToTerraformResourceModel_reflectsAPIChange(t *testing.T) {
	state := &firewallRuleResourceModel{
		SourcePorts: types.StringValue("80"),
		Sources:     ruleObjectList(t, cidrObject("10.0.0.0/16")),
	}
	rule := &swagger.VpcFirewallRule{
		SourcePorts: []string{"443"},
		Sources:     []swagger.FirewallRuleObject{{Cidr: "0.0.0.0/0"}},
	}

	firewallRuleToTerraformResourceModel(context.Background(), rule, state)

	if got := state.SourcePorts.ValueString(); got != "443" {
		t.Errorf("source_ports = %q, want %q (from API)", got, "443")
	}
	if want := ruleObjectList(t, cidrObject("0.0.0.0/0")); !state.Sources.Equal(want) {
		t.Errorf("sources = %v, want %v (from API)", state.Sources, want)
	}
}

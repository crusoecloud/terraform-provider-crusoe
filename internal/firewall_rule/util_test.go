package firewall_rule

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

func Test_cidrListToString(t *testing.T) {
	type args struct {
		ruleObjects []swagger.FirewallRuleObject
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "empty list",
			args: args{},
			want: "",
		},
		{
			name: "single CIDR",
			args: args{ruleObjects: []swagger.FirewallRuleObject{{Cidr: "127.0.0.0/16"}}},
			want: "127.0.0.0/16",
		},
		{
			name: "multiple CIDRs",
			args: args{ruleObjects: []swagger.FirewallRuleObject{{Cidr: "127.0.0.0/16"}, {Cidr: "127.0.0.1/24"}}},
			want: "127.0.0.0/16,127.0.0.1/24",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cidrListToString(tt.args.ruleObjects); got != tt.want {
				t.Errorf("cidrListToString() = %v, want %v", got, tt.want)
			}
		})
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

	firewallRuleToTerraformResourceModel(rule, state)

	if got := state.SourcePorts.ValueString(); got != "*" {
		t.Errorf("source_ports = %q, want %q (preserved)", got, "*")
	}
	if got := state.DestinationPorts.ValueString(); got != "*" {
		t.Errorf("destination_ports = %q, want %q (preserved)", got, "*")
	}
	if got := state.ID.ValueString(); got != "fw-1" {
		t.Errorf("id = %q, want %q (from API)", got, "fw-1")
	}
}

// Test_firewallRuleToTerraformResourceModel_reflectsAPIChange confirms a genuine
// out-of-band change is still surfaced (preserve only applies when equal).
func Test_firewallRuleToTerraformResourceModel_reflectsAPIChange(t *testing.T) {
	state := &firewallRuleResourceModel{SourcePorts: types.StringValue("80")}
	rule := &swagger.VpcFirewallRule{SourcePorts: []string{"443"}}

	firewallRuleToTerraformResourceModel(rule, state)

	if got := state.SourcePorts.ValueString(); got != "443" {
		t.Errorf("source_ports = %q, want %q (from API)", got, "443")
	}
}

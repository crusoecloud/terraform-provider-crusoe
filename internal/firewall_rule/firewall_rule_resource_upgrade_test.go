package firewall_rule

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

// TestFirewallRuleUpgradeStateV0PriorSchema checks that the v0 prior schema reads the
// combined source/destination fields and the port strings, which is what a v0
// configuration uses.
func TestFirewallRuleUpgradeStateV0PriorSchema(t *testing.T) {
	r, ok := NewFirewallRuleResource().(*firewallRuleResource)
	if !ok {
		t.Fatal("unexpected resource type")
	}

	upgraders := r.UpgradeState(context.Background())
	upgrader, ok := upgraders[0]
	if !ok {
		t.Fatal("no state upgrader registered for version 0")
	}
	if upgrader.PriorSchema == nil {
		t.Fatal("version 0 upgrader has no prior schema")
	}

	for _, name := range []string{"id", "source", "source_ports", "destination", "destination_ports"} {
		if _, ok := upgrader.PriorSchema.Attributes[name]; !ok {
			t.Errorf("v0 prior schema should read %q", name)
		}
	}
}

// TestFirewallRuleUpgradeStateV0KeepsDeprecatedFields covers the mapping the v0
// upgrader performs once it has re-read the rule from the API: a prior state that
// used the combined fields keeps them, so the upgraded state still matches the
// configuration. A prior state without them lands on the new lists instead.
func TestFirewallRuleUpgradeStateV0KeepsDeprecatedFields(t *testing.T) {
	rule := &swagger.VpcFirewallRule{
		Id:               "fw-1",
		Sources:          []swagger.FirewallRuleObject{{Cidr: "0.0.0.0/0"}},
		SourcePorts:      []string{wildcardPortRange},
		Destinations:     []swagger.FirewallRuleObject{{Cidr: "10.0.0.0/16"}},
		DestinationPorts: []string{"443"},
	}

	t.Run("prior state used the combined fields", func(t *testing.T) {
		state := firewallRuleResourceModel{
			Source:           types.StringValue("0.0.0.0/0"),
			SourcePorts:      types.StringValue("*"),
			Destination:      types.StringValue("10.0.0.0/16"),
			DestinationPorts: types.StringValue("443"),
		}

		firewallRuleToTerraformResourceModel(context.Background(), rule, &state)

		if got := state.Source.ValueString(); got != "0.0.0.0/0" {
			t.Errorf("source = %q, want %q (carried over)", got, "0.0.0.0/0")
		}
		if got := state.Destination.ValueString(); got != "10.0.0.0/16" {
			t.Errorf("destination = %q, want %q (carried over)", got, "10.0.0.0/16")
		}
		if !state.Sources.IsNull() || !state.Destinations.IsNull() {
			t.Error("sources/destinations should stay null while the combined fields are in use")
		}
		if got := state.SourcePorts.ValueString(); got != "*" {
			t.Errorf("source_ports = %q, want %q (carried over)", got, "*")
		}
		if got := state.DestinationPorts.ValueString(); got != "443" {
			t.Errorf("destination_ports = %q, want %q (carried over)", got, "443")
		}
		if !state.SourcePortRanges.IsNull() || !state.DestinationPortRanges.IsNull() {
			t.Error("source_port_ranges/destination_port_ranges should stay null while the port strings are in use")
		}
	})

	t.Run("prior state had no combined fields", func(t *testing.T) {
		var state firewallRuleResourceModel

		firewallRuleToTerraformResourceModel(context.Background(), rule, &state)

		if want := ruleObjectList(t, cidrObject("0.0.0.0/0")); !state.Sources.Equal(want) {
			t.Errorf("sources = %v, want %v", state.Sources, want)
		}
		if want := ruleObjectList(t, cidrObject("10.0.0.0/16")); !state.Destinations.Equal(want) {
			t.Errorf("destinations = %v, want %v", state.Destinations, want)
		}
		if !state.Source.IsNull() || !state.Destination.IsNull() {
			t.Error("source/destination should stay null when the prior state did not use them")
		}
		// Without a configured format to preserve, the lists take the API's raw values.
		if want := portList(t, wildcardPortRange); !state.SourcePortRanges.Equal(want) {
			t.Errorf("source_port_ranges = %v, want %v", state.SourcePortRanges, want)
		}
		if want := portList(t, "443"); !state.DestinationPortRanges.Equal(want) {
			t.Errorf("destination_port_ranges = %v, want %v", state.DestinationPortRanges, want)
		}
		if !state.SourcePorts.IsNull() || !state.DestinationPorts.IsNull() {
			t.Error("source_ports/destination_ports should stay null when the prior state did not use them")
		}
	})
}

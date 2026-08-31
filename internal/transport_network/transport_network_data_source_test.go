package transport_network

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

// Test_transportNetworksToModel_sorts asserts a shuffled API response yields a
// stable, key-sorted result (name, then id). The Crusoe list endpoints do not
// guarantee element order, so an unsorted mapping produces spurious diffs on
// unchanged infrastructure (CCX-4394).
func Test_transportNetworksToModel_sorts(t *testing.T) {
	items := []swagger.IbNetwork{
		{Id: "id-3", Name: "beta", Location: "us-east1-a"},
		{Id: "id-1", Name: "alpha", Location: "us-east1-a"},
		// Same name as the first entry: id breaks the tie.
		{Id: "id-2", Name: "beta", Location: "us-east1-a"},
	}

	got := transportNetworksToModel(items)

	want := []string{"id-1", "id-2", "id-3"}
	if len(got) != len(want) {
		t.Fatalf("got %d networks, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i] {
			t.Errorf("networks[%d].id = %q, want %q", i, got[i].ID, want[i])
		}
	}
}

// Test_transportNetworksToModel_capacities checks each network carries only its
// own capacities, rather than accumulating the previous network's.
func Test_transportNetworksToModel_capacities(t *testing.T) {
	items := []swagger.IbNetwork{
		{Id: "id-1", Name: "alpha", Capacities: []swagger.IbNetworkCapacity{{Quantity: 8, SliceType: "a100.8x"}}},
		{Id: "id-2", Name: "beta"},
	}

	got := transportNetworksToModel(items)

	if len(got[0].Capacities) != 1 {
		t.Errorf("alpha capacities = %d, want 1", len(got[0].Capacities))
	}
	if got[0].Capacities[0].Quantity != 8 {
		t.Errorf("alpha capacity quantity = %d, want 8", got[0].Capacities[0].Quantity)
	}
	if len(got[1].Capacities) != 0 {
		t.Errorf("beta capacities = %d, want 0", len(got[1].Capacities))
	}
}

// Test_transportNetworksSchema guards the CCX-2836 convention that every
// attribute carries a description, and that the API-populated fields are all
// Computed so no value is silently dropped from state.
func Test_transportNetworksSchema(t *testing.T) {
	ctx := context.Background()

	schemaResp := &datasource.SchemaResponse{}
	NewTransportNetworkDataSource().Schema(ctx, datasource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to build schema: %v", schemaResp.Diagnostics)
	}

	for name, attr := range schemaResp.Schema.Attributes {
		if attr.GetDescription() == "" {
			t.Errorf("attribute %q has no description", name)
		}
	}

	networks, ok := schemaResp.Schema.Attributes["transport_networks"].(schema.ListNestedAttribute)
	if !ok {
		t.Fatal("transport_networks attribute not found")
	}
	for name, attr := range networks.NestedObject.Attributes {
		if !attr.IsComputed() {
			t.Errorf("transport_networks.%s should be computed", name)
		}
		if attr.GetDescription() == "" {
			t.Errorf("transport_networks.%s has no description", name)
		}
	}

	capacities, ok := networks.NestedObject.Attributes["capacities"].(schema.ListNestedAttribute)
	if !ok {
		t.Fatal("transport_networks.capacities attribute not found")
	}
	for name, attr := range capacities.NestedObject.Attributes {
		if !attr.IsComputed() {
			t.Errorf("transport_networks.capacities.%s should be computed", name)
		}
		if attr.GetDescription() == "" {
			t.Errorf("transport_networks.capacities.%s has no description", name)
		}
	}
}

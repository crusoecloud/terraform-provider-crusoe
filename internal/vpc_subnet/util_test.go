package vpc_subnet

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

// Test_vpcSubnetToTerraformResourceModel covers the shared transform that all
// CRUD paths now use: scalar fields are sourced from the API, the (Computed)
// nat_gateways list is sorted by ID for deterministic ordering (CCX-4394),
// nat_gateway_enabled is derived from gateway presence, and project_id is left
// for the caller to set.
func Test_vpcSubnetToTerraformResourceModel(t *testing.T) {
	ctx := context.Background()
	state := &vpcSubnetResourceModel{ProjectID: types.StringValue("project-1")}
	subnet := &swagger.VpcSubnet{
		Id:           "subnet-1",
		Name:         "my-subnet",
		Cidr:         "10.0.1.0/24",
		Location:     "us-east1-a",
		VpcNetworkId: "vpc-1",
		NatGateways: []swagger.NatGateway{
			{Id: "nat-c", PublicIpv4Address: "1.1.1.3", PublicIpv4Id: "ip-c"},
			{Id: "nat-a", PublicIpv4Address: "1.1.1.1", PublicIpv4Id: "ip-a"},
			{Id: "nat-b", PublicIpv4Address: "1.1.1.2", PublicIpv4Id: "ip-b"},
		},
	}

	var diags diag.Diagnostics
	vpcSubnetToTerraformResourceModel(ctx, subnet, state, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got := state.ID.ValueString(); got != "subnet-1" {
		t.Errorf("id = %q, want %q", got, "subnet-1")
	}
	if got := state.Network.ValueString(); got != "vpc-1" {
		t.Errorf("network = %q, want %q", got, "vpc-1")
	}
	if !state.NATGatewayEnabled.ValueBool() {
		t.Error("nat_gateway_enabled = false, want true (gateways present)")
	}
	if got := state.ProjectID.ValueString(); got != "project-1" {
		t.Errorf("project_id = %q, want %q (untouched by transform)", got, "project-1")
	}

	var gws []vpcSubnetNatGatewayResourceModel
	if d := state.NATGateways.ElementsAs(ctx, &gws, false); d.HasError() {
		t.Fatalf("reading nat_gateways: %v", d)
	}
	gotIDs := make([]string, len(gws))
	for i, g := range gws {
		gotIDs[i] = g.ID.ValueString()
	}
	wantIDs := []string{"nat-a", "nat-b", "nat-c"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Errorf("nat gateway ids = %v, want %v (sorted)", gotIDs, wantIDs)
	}
}

// Test_vpcSubnetToTerraformResourceModel_noGateways confirms nat_gateway_enabled
// is false when the API returns no NAT gateways.
func Test_vpcSubnetToTerraformResourceModel_noGateways(t *testing.T) {
	ctx := context.Background()
	state := &vpcSubnetResourceModel{}
	subnet := &swagger.VpcSubnet{Id: "subnet-1"}

	var diags diag.Diagnostics
	vpcSubnetToTerraformResourceModel(ctx, subnet, state, &diags)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if state.NATGatewayEnabled.ValueBool() {
		t.Error("nat_gateway_enabled = true, want false (no gateways)")
	}
}

// subnetRawValue builds a raw subnet object. Attributes not named in attrs are null.
func subnetRawValue(ctx context.Context, t *testing.T, s schema.Schema, attrs map[string]tftypes.Value) tftypes.Value {
	t.Helper()

	objType, ok := s.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is %T, want tftypes.Object", s.Type().TerraformType(ctx))
	}

	vals := make(map[string]tftypes.Value, len(objType.AttributeTypes))
	for name, typ := range objType.AttributeTypes {
		if v, found := attrs[name]; found {
			vals[name] = v

			continue
		}
		vals[name] = tftypes.NewValue(typ, nil)
	}

	return tftypes.NewValue(objType, vals)
}

func natGatewayList(ctx context.Context, t *testing.T, ids ...string) types.List {
	t.Helper()

	gateways := make([]swagger.NatGateway, 0, len(ids))
	for _, id := range ids {
		gateways = append(gateways, swagger.NatGateway{
			Id:                id,
			PublicIpv4Address: "203.0.113.10",
			PublicIpv4Id:      "ip-" + id,
		})
	}

	list, diags := natGatewaysToTerraformResourceModel(ctx, gateways)
	if diags.HasError() {
		t.Fatalf("failed to build nat_gateways list: %v", diags)
	}

	return list
}

func TestNatGatewaysUseStateUnlessToggled(t *testing.T) {
	ctx := context.Background()

	r := &vpcSubnetResource{}
	schemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to build schema: %v", schemaResp.Diagnostics)
	}
	s := schemaResp.Schema

	unknownBool := tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue)
	plannedList := natGatewayList(ctx, t, "gw-planned")

	tests := []struct {
		name            string
		hasState        bool
		stateEnabled    bool
		stateGatewayIDs []string
		planEnabled     tftypes.Value
		planValue       *types.List // nil means unknown
		wantUnknown     bool
	}{
		{
			name:        "create leaves the list unknown",
			hasState:    false,
			planEnabled: tftypes.NewValue(tftypes.Bool, true),
			wantUnknown: true,
		},
		{
			name:         "update with nat_gateway_enabled unchanged false keeps empty state",
			hasState:     true,
			stateEnabled: false,
			planEnabled:  tftypes.NewValue(tftypes.Bool, false),
			wantUnknown:  false,
		},
		{
			name:            "update with nat_gateway_enabled unchanged true keeps gateways from state",
			hasState:        true,
			stateEnabled:    true,
			stateGatewayIDs: []string{"gw-1"},
			planEnabled:     tftypes.NewValue(tftypes.Bool, true),
			wantUnknown:     false,
		},
		{
			name:         "toggle false to true leaves the list unknown",
			hasState:     true,
			stateEnabled: false,
			planEnabled:  tftypes.NewValue(tftypes.Bool, true),
			wantUnknown:  true,
		},
		{
			name:            "toggle true to false leaves the list unknown",
			hasState:        true,
			stateEnabled:    true,
			stateGatewayIDs: []string{"gw-1"},
			planEnabled:     tftypes.NewValue(tftypes.Bool, false),
			wantUnknown:     true,
		},
		{
			name:         "unknown nat_gateway_enabled leaves the list unknown",
			hasState:     true,
			stateEnabled: false,
			planEnabled:  unknownBool,
			wantUnknown:  true,
		},
		{
			name:         "known planned value is left untouched",
			hasState:     true,
			stateEnabled: false,
			planEnabled:  tftypes.NewValue(tftypes.Bool, true),
			planValue:    &plannedList,
			wantUnknown:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateList := natGatewayList(ctx, t, tt.stateGatewayIDs...)
			stateListRaw, err := stateList.ToTerraformValue(ctx)
			if err != nil {
				t.Fatalf("failed to convert state list: %v", err)
			}

			stateValue := types.ListNull(vpcSubnetNatGatewaySchema)
			stateRaw := tftypes.NewValue(s.Type().TerraformType(ctx), nil)
			if tt.hasState {
				stateValue = stateList
				stateRaw = subnetRawValue(ctx, t, s, map[string]tftypes.Value{
					"nat_gateway_enabled": tftypes.NewValue(tftypes.Bool, tt.stateEnabled),
					"nat_gateways":        stateListRaw,
				})
			}

			planValue := types.ListUnknown(vpcSubnetNatGatewaySchema)
			if tt.planValue != nil {
				planValue = *tt.planValue
			}
			planListRaw, err := planValue.ToTerraformValue(ctx)
			if err != nil {
				t.Fatalf("failed to convert plan list: %v", err)
			}
			planRaw := subnetRawValue(ctx, t, s, map[string]tftypes.Value{
				"nat_gateway_enabled": tt.planEnabled,
				"nat_gateways":        planListRaw,
			})

			req := planmodifier.ListRequest{
				Path:        path.Root("nat_gateways"),
				Plan:        tfsdk.Plan{Raw: planRaw, Schema: s},
				State:       tfsdk.State{Raw: stateRaw, Schema: s},
				PlanValue:   planValue,
				StateValue:  stateValue,
				ConfigValue: types.ListNull(vpcSubnetNatGatewaySchema),
			}
			resp := &planmodifier.ListResponse{PlanValue: req.PlanValue}

			natGatewaysUseStateUnlessToggled().PlanModifyList(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}

			if tt.wantUnknown {
				if !resp.PlanValue.IsUnknown() {
					t.Fatalf("plan value = %v, want unknown", resp.PlanValue)
				}

				return
			}

			want := stateValue
			if tt.planValue != nil {
				want = *tt.planValue
			}
			if !resp.PlanValue.Equal(want) {
				t.Errorf("plan value = %v, want %v", resp.PlanValue, want)
			}
		})
	}
}

// fakeSubnetGetter returns the responses in order and repeats the last one.
func fakeSubnetGetter(responses ...swagger.VpcSubnet) (get getVPCSubnetFunc, calls *int) {
	count := 0

	return func(_ context.Context, _, _ string) (swagger.VpcSubnet, *http.Response, error) {
		idx := count
		if idx >= len(responses) {
			idx = len(responses) - 1
		}
		count++

		return responses[idx], &http.Response{Body: http.NoBody}, nil
	}, &count
}

func TestAwaitNATGatewayState(t *testing.T) {
	ctx := context.Background()

	withGateway := swagger.VpcSubnet{Id: "subnet-1", NatGateways: []swagger.NatGateway{{Id: "gw-1"}}}
	withoutGateway := swagger.VpcSubnet{Id: "subnet-1"}

	t.Run("returns immediately when the state already matches", func(t *testing.T) {
		get, calls := fakeSubnetGetter(withoutGateway)

		got, err := awaitNATGatewayState(ctx, get, "project", "subnet-1", false, time.Millisecond, time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if natGatewayPresent(got) {
			t.Error("expected no NAT gateway on the returned subnet")
		}
		if *calls != 1 {
			t.Errorf("get called %d times, want 1", *calls)
		}
	})

	t.Run("polls until the gateway is removed", func(t *testing.T) {
		get, calls := fakeSubnetGetter(withGateway, withGateway, withoutGateway)

		got, err := awaitNATGatewayState(ctx, get, "project", "subnet-1", false, time.Millisecond, time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if natGatewayPresent(got) {
			t.Error("expected no NAT gateway on the returned subnet")
		}
		if *calls != 3 {
			t.Errorf("get called %d times, want 3", *calls)
		}
	})

	t.Run("polls until the gateway is attached", func(t *testing.T) {
		get, _ := fakeSubnetGetter(withoutGateway, withGateway)

		got, err := awaitNATGatewayState(ctx, get, "project", "subnet-1", true, time.Millisecond, time.Second)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !natGatewayPresent(got) {
			t.Error("expected a NAT gateway on the returned subnet")
		}
	})

	t.Run("times out when the state never settles", func(t *testing.T) {
		get, _ := fakeSubnetGetter(withGateway)

		_, err := awaitNATGatewayState(ctx, get, "project", "subnet-1", false, time.Millisecond, 10*time.Millisecond)
		if !errors.Is(err, errNATGatewayTimeout) {
			t.Fatalf("error = %v, want %v", err, errNATGatewayTimeout)
		}
	})

	t.Run("stops when the context is cancelled", func(t *testing.T) {
		get, _ := fakeSubnetGetter(withGateway)
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		_, err := awaitNATGatewayState(cancelCtx, get, "project", "subnet-1", false, time.Second, time.Minute)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want %v", err, context.Canceled)
		}
	})

	t.Run("surfaces a get error", func(t *testing.T) {
		getErr := errors.New("boom")
		get := func(_ context.Context, _, _ string) (swagger.VpcSubnet, *http.Response, error) {
			return swagger.VpcSubnet{}, nil, getErr
		}

		_, err := awaitNATGatewayState(ctx, get, "project", "subnet-1", false, time.Millisecond, time.Second)
		if !errors.Is(err, getErr) {
			t.Fatalf("error = %v, want wrapped %v", err, getErr)
		}
	})
}

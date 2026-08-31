package firewall_rule

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

func TestFirewallRuleResource_Metadata(t *testing.T) {
	r := NewFirewallRuleResource()

	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "crusoe"}, resp)

	expected := "crusoe_vpc_firewall_rule"
	if resp.TypeName != expected {
		t.Errorf("TypeName: expected %q, got %q", expected, resp.TypeName)
	}
}

func firewallRuleSchema(t *testing.T) schema.Schema {
	t.Helper()

	r, ok := NewFirewallRuleResource().(*firewallRuleResource)
	if !ok {
		t.Fatal("unexpected resource type")
	}
	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)

	return resp.Schema
}

func TestFirewallRuleResource_DeprecatedFields(t *testing.T) {
	s := firewallRuleSchema(t)

	for _, name := range []string{"source", "destination"} {
		attr, ok := s.Attributes[name].(schema.StringAttribute)
		if !ok {
			t.Fatalf("%s attribute not found", name)
		}
		if attr.DeprecationMessage == "" {
			t.Errorf("%s should be deprecated", name)
		}
		if attr.Required {
			t.Errorf("deprecated %s should no longer be Required", name)
		}
		if !attr.Optional {
			t.Errorf("deprecated %s should be Optional", name)
		}
		if len(attr.Validators) == 0 {
			t.Errorf("%s should carry the ExactlyOneOf validator", name)
		}
	}
}

// TestFirewallRuleResource_RuleObjectLists checks that sources/destinations mirror
// the API's list-of-rule-objects shape and carry the per-element exactly-one
// constraint that keeps a `cidr` + `resource_id` (or neither) element out of a request.
func TestFirewallRuleResource_RuleObjectLists(t *testing.T) {
	s := firewallRuleSchema(t)

	for _, name := range []string{"sources", "destinations"} {
		attr, ok := s.Attributes[name].(schema.ListNestedAttribute)
		if !ok {
			t.Fatalf("%s attribute not found or not a ListNestedAttribute", name)
		}
		if !attr.Optional {
			t.Errorf("%s should be Optional", name)
		}
		if attr.Required {
			t.Errorf("%s should not be Required", name)
		}
		if len(attr.Validators) == 0 {
			t.Errorf("%s should reject an empty list", name)
		}

		nested := attr.NestedObject.Attributes
		if len(nested) != 2 {
			t.Errorf("%s elements should expose exactly cidr and resource_id, got %d attributes", name, len(nested))
		}
		for _, member := range []string{"cidr", "resource_id"} {
			memberAttr, isString := nested[member].(schema.StringAttribute)
			if !isString {
				t.Fatalf("%s.%s attribute not found", name, member)
			}
			if !memberAttr.Optional {
				t.Errorf("%s.%s should be Optional", name, member)
			}
			if len(memberAttr.Validators) == 0 {
				t.Errorf("%s.%s should have validators", name, member)
			}
		}

		cidrAttr, ok := nested["cidr"].(schema.StringAttribute)
		if !ok {
			t.Fatalf("%s.cidr attribute not found", name)
		}
		if !hasExactlyOneOfValidator(cidrAttr.Validators) {
			t.Errorf("%s.cidr should carry the ExactlyOneOf validator against resource_id", name)
		}
	}
}

// hasExactlyOneOfValidator reports whether one of the validators enforces the
// exactly-one-of constraint, identified by its description since the concrete type
// lives in an internal package of terraform-plugin-framework-validators.
func hasExactlyOneOfValidator(validators []validator.String) bool {
	for _, v := range validators {
		if strings.Contains(v.Description(context.Background()), "one and only one") {
			return true
		}
	}

	return false
}

// newImportStateResponse builds an empty ImportStateResponse backed by the
// given resource schema, mirroring how the framework initializes state before
// calling ImportState.
func newImportStateResponse(ctx context.Context, t *testing.T, schemaFn func(context.Context, resource.SchemaRequest, *resource.SchemaResponse)) *resource.ImportStateResponse {
	t.Helper()

	schemaResp := &resource.SchemaResponse{}
	schemaFn(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to build schema: %v", schemaResp.Diagnostics)
	}

	return &resource.ImportStateResponse{
		State: tfsdk.State{
			Raw:    tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), nil),
			Schema: schemaResp.Schema,
		},
	}
}

// TestFirewallRuleImportState exercises the ImportState path end-to-end: it
// confirms the parsed identifiers land in the "id" and "project_id" schema
// attributes, that the client project ID is used as a fallback, and that an
// invalid identifier surfaces a diagnostic. The parsing logic itself is covered
// by TestParseResourceIdentifiers in the common package.
func TestFirewallRuleImportState(t *testing.T) {
	const (
		resourceUUID = "11111111-1111-1111-1111-111111111111"
		projectUUID  = "22222222-2222-2222-2222-222222222222"
		fallbackUUID = "33333333-3333-3333-3333-333333333333"
	)

	ctx := context.Background()

	t.Run("explicit project id from suffix", func(t *testing.T) {
		r := &firewallRuleResource{client: &common.CrusoeClient{ProjectID: fallbackUUID}}
		resp := newImportStateResponse(ctx, t, r.Schema)

		r.ImportState(ctx, resource.ImportStateRequest{ID: resourceUUID + "," + projectUUID}, resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}

		var gotID, gotProject types.String
		resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("id"), &gotID)...)
		resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("project_id"), &gotProject)...)
		if resp.Diagnostics.HasError() {
			t.Fatalf("failed reading state attributes: %v", resp.Diagnostics)
		}

		if gotID.ValueString() != resourceUUID {
			t.Errorf("id = %q, want %q", gotID.ValueString(), resourceUUID)
		}
		if gotProject.ValueString() != projectUUID {
			t.Errorf("project_id = %q, want %q", gotProject.ValueString(), projectUUID)
		}
	})

	t.Run("falls back to client project id", func(t *testing.T) {
		r := &firewallRuleResource{client: &common.CrusoeClient{ProjectID: fallbackUUID}}
		resp := newImportStateResponse(ctx, t, r.Schema)

		r.ImportState(ctx, resource.ImportStateRequest{ID: resourceUUID}, resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
		}

		var gotProject types.String
		resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("project_id"), &gotProject)...)
		if resp.Diagnostics.HasError() {
			t.Fatalf("failed reading project_id: %v", resp.Diagnostics)
		}
		if gotProject.ValueString() != fallbackUUID {
			t.Errorf("project_id = %q, want fallback %q", gotProject.ValueString(), fallbackUUID)
		}
	})

	t.Run("invalid identifier surfaces diagnostic", func(t *testing.T) {
		r := &firewallRuleResource{client: &common.CrusoeClient{ProjectID: fallbackUUID}}
		resp := newImportStateResponse(ctx, t, r.Schema)

		r.ImportState(ctx, resource.ImportStateRequest{ID: "not-a-uuid"}, resp)

		if !resp.Diagnostics.HasError() {
			t.Error("expected diagnostics error for invalid identifier, got none")
		}
	})
}

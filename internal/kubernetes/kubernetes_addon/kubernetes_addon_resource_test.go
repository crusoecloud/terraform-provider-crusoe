package kubernetes_addon

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

func TestKubernetesAddonResource_Metadata(t *testing.T) {
	r := NewKubernetesAddonResource()

	resp := &resource.MetadataResponse{}
	r.Metadata(context.Background(), resource.MetadataRequest{ProviderTypeName: "crusoe"}, resp)

	if expected := "crusoe_kubernetes_addon"; resp.TypeName != expected {
		t.Errorf("TypeName: expected %q, got %q", expected, resp.TypeName)
	}
}

func TestKubernetesAddonResource_Schema(t *testing.T) {
	r := NewKubernetesAddonResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)
	attrs := schemaResp.Schema.Attributes

	for _, f := range []string{"cluster_id", "addon_type"} {
		a, ok := attrs[f]
		if !ok {
			t.Errorf("missing required attribute %q", f)

			continue
		}
		if !a.IsRequired() {
			t.Errorf("attribute %q should be Required", f)
		}
	}

	for _, f := range []string{"project_id", "version"} {
		a, ok := attrs[f]
		if !ok {
			t.Errorf("missing attribute %q", f)

			continue
		}
		if !a.IsOptional() || !a.IsComputed() {
			t.Errorf("attribute %q should be Optional+Computed", f)
		}
	}

	for _, f := range []string{"id", "installed_version", "status", "ready", "category"} {
		a, ok := attrs[f]
		if !ok {
			t.Errorf("missing attribute %q", f)

			continue
		}
		if !a.IsComputed() || a.IsRequired() || a.IsOptional() {
			t.Errorf("attribute %q should be Computed-only", f)
		}
	}
}

// TestKubernetesAddonResource_ForceNewFields verifies the identity attributes
// have plan modifiers (RequiresReplace).
func TestKubernetesAddonResource_ForceNewFields(t *testing.T) {
	r := NewKubernetesAddonResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	for _, f := range []string{"project_id", "cluster_id", "addon_type"} {
		attr, ok := schemaResp.Schema.Attributes[f].(schema.StringAttribute)
		if !ok {
			t.Errorf("attribute %q not found or not a StringAttribute", f)

			continue
		}
		if len(attr.PlanModifiers) == 0 {
			t.Errorf("attribute %q should have plan modifiers (RequiresReplace)", f)
		}
	}
}

func TestKubernetesAddonResource_VersionImmutable(t *testing.T) {
	r := NewKubernetesAddonResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	attr, ok := schemaResp.Schema.Attributes["version"].(schema.StringAttribute)
	if !ok {
		t.Fatal("version attribute not found or not a StringAttribute")
	}

	tests := []struct {
		name    string
		state   types.String
		config  types.String
		plan    types.String
		wantErr bool
	}{
		{"create", types.StringNull(), types.StringValue("v1.19.1"), types.StringValue("v1.19.1"), false},
		{"unchanged", types.StringValue("v1.19.1"), types.StringValue("v1.19.1"), types.StringValue("v1.19.1"), false},
		{"omitted keeps state", types.StringValue("v1.19.1"), types.StringNull(), types.StringUnknown(), false},
		{"changed", types.StringValue("v1.19.1"), types.StringValue("v1.20.0"), types.StringValue("v1.20.0"), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := planmodifier.StringRequest{StateValue: tc.state, PlanValue: tc.plan, ConfigValue: tc.config}
			resp := &planmodifier.StringResponse{PlanValue: tc.plan}
			for _, m := range attr.PlanModifiers {
				req.PlanValue = resp.PlanValue
				m.PlanModifyString(context.Background(), req, resp)
			}
			if got := resp.Diagnostics.HasError(); got != tc.wantErr {
				t.Errorf("HasError = %v, want %v (%v)", got, tc.wantErr, resp.Diagnostics)
			}
		})
	}
}

func TestKubernetesAddonResource_AddonTypeLimitedAvailabilityWarning(t *testing.T) {
	r := NewKubernetesAddonResource()
	schemaResp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, schemaResp)

	attr, ok := schemaResp.Schema.Attributes["addon_type"].(schema.StringAttribute)
	if !ok {
		t.Fatal("addon_type attribute not found or not a StringAttribute")
	}

	tests := []struct {
		name        string
		state       types.String
		plan        types.String
		wantWarning bool
	}{
		{"create", types.StringNull(), types.StringValue("cert-manager"), true},
		{"unchanged", types.StringValue("cert-manager"), types.StringValue("cert-manager"), false},
		{"changed", types.StringValue("cert-manager"), types.StringValue("cilium"), true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := planmodifier.StringRequest{StateValue: tc.state, PlanValue: tc.plan, ConfigValue: tc.plan}
			resp := &planmodifier.StringResponse{PlanValue: tc.plan}
			for _, m := range attr.PlanModifiers {
				m.PlanModifyString(context.Background(), req, resp)
			}

			warnings := resp.Diagnostics.Warnings()
			if got := len(warnings) > 0; got != tc.wantWarning {
				t.Fatalf("warning = %v, want %v (%v)", got, tc.wantWarning, resp.Diagnostics)
			}
			if tc.wantWarning && warnings[0].Detail() != common.LimitedAvailabilityMessage {
				t.Errorf("warning detail = %q, want common.LimitedAvailabilityMessage", warnings[0].Detail())
			}
		})
	}
}

func TestKubernetesAddonResource_ImplementsImportState(t *testing.T) {
	if _, ok := NewKubernetesAddonResource().(resource.ResourceWithImportState); !ok {
		t.Error("resource should implement ResourceWithImportState")
	}
}

package instance_template

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

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

// TestInstanceTemplateImportState exercises the ImportState path end-to-end: it
// confirms the parsed identifiers land in the "id" and "project_id" schema
// attributes, that the client project ID is used as a fallback, and that an
// invalid identifier surfaces a diagnostic. The parsing logic itself is covered
// by TestParseResourceIdentifiers in the common package.
func TestInstanceTemplateImportState(t *testing.T) {
	const (
		resourceUUID = "11111111-1111-1111-1111-111111111111"
		projectUUID  = "22222222-2222-2222-2222-222222222222"
		fallbackUUID = "33333333-3333-3333-3333-333333333333"
	)

	ctx := context.Background()

	t.Run("explicit project id from suffix", func(t *testing.T) {
		r := &instanceTemplateResource{client: &common.CrusoeClient{ProjectID: fallbackUUID}}
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
		r := &instanceTemplateResource{client: &common.CrusoeClient{ProjectID: fallbackUUID}}
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
		r := &instanceTemplateResource{client: &common.CrusoeClient{ProjectID: fallbackUUID}}
		resp := newImportStateResponse(ctx, t, r.Schema)

		r.ImportState(ctx, resource.ImportStateRequest{ID: "not-a-uuid"}, resp)

		if !resp.Diagnostics.HasError() {
			t.Error("expected diagnostics error for invalid identifier, got none")
		}
	})
}

// templateAttrs is a full set of raw attribute values for the instance template schema,
// with the partition alias pair unset. Individual tests override the attributes they care
// about, which keeps a plan and a state differing only where the test intends.
func templateAttrs(ctx context.Context, t *testing.T, s schema.Schema) map[string]tftypes.Value {
	t.Helper()

	objType, ok := s.Type().TerraformType(ctx).(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is not an object")
	}

	diskSetType, ok := objType.AttributeTypes["disks"].(tftypes.Set)
	if !ok {
		t.Fatalf("disks attribute is not a set")
	}

	diskType := diskSetType.ElementType
	disk := func(size, diskType_ string) tftypes.Value {
		return tftypes.NewValue(diskType, map[string]tftypes.Value{
			"size": tftypes.NewValue(tftypes.String, size),
			"type": tftypes.NewValue(tftypes.String, diskType_),
		})
	}

	return map[string]tftypes.Value{
		"id":                        tftypes.NewValue(tftypes.String, "template-1"),
		"project_id":                tftypes.NewValue(tftypes.String, "proj-1"),
		"name":                      tftypes.NewValue(tftypes.String, "my-template"),
		"type":                      tftypes.NewValue(tftypes.String, "a100.1x"),
		"ssh_key":                   tftypes.NewValue(tftypes.String, "ssh-ed25519 AAAA user@host"),
		"location":                  tftypes.NewValue(tftypes.String, "us-east1-a"),
		"image":                     tftypes.NewValue(tftypes.String, "ubuntu"),
		"startup_script":            tftypes.NewValue(tftypes.String, nil),
		"shutdown_script":           tftypes.NewValue(tftypes.String, nil),
		"subnet":                    tftypes.NewValue(tftypes.String, "subnet-1"),
		"ib_partition":              tftypes.NewValue(tftypes.String, nil),
		"transport_partition_id":    tftypes.NewValue(tftypes.String, nil),
		"public_ip_address_type":    tftypes.NewValue(tftypes.String, "dynamic"),
		"disks":                     tftypes.NewValue(objType.AttributeTypes["disks"], []tftypes.Value{disk("100GiB", "persistent-ssd")}),
		"shared_volume_attachments": tftypes.NewValue(objType.AttributeTypes["shared_volume_attachments"], nil),
		"reservation_id":            tftypes.NewValue(tftypes.String, nil),
		"placement_policy":          tftypes.NewValue(tftypes.String, "unspecified"),
		"nvlink_domain_id":          tftypes.NewValue(tftypes.String, nil),
	}
}

// newUpdateRequest builds an UpdateRequest and the matching prepopulated response. The
// response state starts as the prior state, which is how the framework hands it over, so a
// test can tell an explicit write apart from a method that returned without writing.
func newUpdateRequest(ctx context.Context, t *testing.T,
	stateAttrs, planAttrs map[string]tftypes.Value,
) (resource.UpdateRequest, *resource.UpdateResponse) {
	t.Helper()

	schemaResp := &resource.SchemaResponse{}
	(&instanceTemplateResource{}).Schema(ctx, resource.SchemaRequest{}, schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("failed to build schema: %v", schemaResp.Diagnostics)
	}

	s := schemaResp.Schema
	objType := s.Type().TerraformType(ctx)

	stateRaw := tftypes.NewValue(objType, stateAttrs)
	planRaw := tftypes.NewValue(objType, planAttrs)

	req := resource.UpdateRequest{
		State: tfsdk.State{Raw: stateRaw, Schema: s},
		Plan:  tfsdk.Plan{Raw: planRaw, Schema: s},
	}
	resp := &resource.UpdateResponse{
		State: tfsdk.State{Raw: stateRaw, Schema: s},
	}

	return req, resp
}

// TestInstanceTemplateUpdateAliasRenameIsInPlace covers the one change an instance template
// permits in place: renaming ib_partition to transport_partition_id.
//
// The resource is built with a nil client on purpose. Renaming the attribute does not
// change the backend object, so Update must make no API call; if it tries, the test panics
// instead of quietly passing.
func TestInstanceTemplateUpdateAliasRenameIsInPlace(t *testing.T) {
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	(&instanceTemplateResource{}).Schema(ctx, resource.SchemaRequest{}, schemaResp)

	stateAttrs := templateAttrs(ctx, t, schemaResp.Schema)
	stateAttrs["ib_partition"] = tftypes.NewValue(tftypes.String, "p1")

	planAttrs := templateAttrs(ctx, t, schemaResp.Schema)
	planAttrs["transport_partition_id"] = tftypes.NewValue(tftypes.String, "p1")

	req, resp := newUpdateRequest(ctx, t, stateAttrs, planAttrs)

	r := &instanceTemplateResource{}
	r.Update(ctx, req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
	}

	// The plan has to be written verbatim, or the rename is lost and Terraform reports an
	// inconsistent result.
	if !resp.State.Raw.Equal(req.Plan.Raw) {
		t.Errorf("state after update = %v, want the plan %v", resp.State.Raw, req.Plan.Raw)
	}
}

// TestInstanceTemplateUpdateRejectsNonAliasChange checks that a change to any other
// attribute still fails on immutability rather than being silently written to state.
func TestInstanceTemplateUpdateRejectsNonAliasChange(t *testing.T) {
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	(&instanceTemplateResource{}).Schema(ctx, resource.SchemaRequest{}, schemaResp)

	tests := []struct {
		name   string
		mutate func(attrs map[string]tftypes.Value)
	}{
		{
			name: "startup_script changes",
			mutate: func(attrs map[string]tftypes.Value) {
				attrs["startup_script"] = tftypes.NewValue(tftypes.String, "#!/bin/sh\necho hi")
			},
		},
		{
			// disks has no set-level replace modifier, so Terraform plans an element
			// removal as an in-place update and it reaches Update. The guard is what keeps
			// that from dropping the disk from state.
			name: "a disk is removed",
			mutate: func(attrs map[string]tftypes.Value) {
				attrs["disks"] = tftypes.NewValue(attrs["disks"].Type(), []tftypes.Value{})
			},
		},
		{
			name: "the partition changes at the same time as another attribute",
			mutate: func(attrs map[string]tftypes.Value) {
				attrs["transport_partition_id"] = tftypes.NewValue(tftypes.String, "p1")
				attrs["name"] = tftypes.NewValue(tftypes.String, "renamed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stateAttrs := templateAttrs(ctx, t, schemaResp.Schema)
			stateAttrs["ib_partition"] = tftypes.NewValue(tftypes.String, "p1")

			planAttrs := templateAttrs(ctx, t, schemaResp.Schema)
			planAttrs["ib_partition"] = tftypes.NewValue(tftypes.String, "p1")
			tt.mutate(planAttrs)

			req, resp := newUpdateRequest(ctx, t, stateAttrs, planAttrs)

			r := &instanceTemplateResource{}
			r.Update(ctx, req, resp)

			if !resp.Diagnostics.HasError() {
				t.Fatal("expected an immutability error, got none")
			}

			// Assert on the specific error, so that a failure to compare the plan and
			// state cannot make this test pass for the wrong reason.
			var foundImmutabilityError bool
			for _, d := range resp.Diagnostics.Errors() {
				if d.Summary() == "Failed to update instance template" {
					foundImmutabilityError = true
				}
			}
			if !foundImmutabilityError {
				t.Errorf("expected the immutability error, got: %v", resp.Diagnostics)
			}

			if !resp.State.Raw.Equal(req.State.Raw) {
				t.Error("state was modified despite the update being rejected")
			}
		})
	}
}

// TestInstanceTemplateValidateConfigAliasPair checks the conflict check on the alias pair.
// Both names set to the same partition stays legal, so a configuration written during the
// migration keeps working; two different partitions is ambiguous and is rejected.
func TestInstanceTemplateValidateConfigAliasPair(t *testing.T) {
	ctx := context.Background()

	schemaResp := &resource.SchemaResponse{}
	(&instanceTemplateResource{}).Schema(ctx, resource.SchemaRequest{}, schemaResp)

	tests := []struct {
		name        string
		ibPartition interface{}
		transportID interface{}
		wantError   bool
	}{
		{"neither set", nil, nil, false},
		{"deprecated only", "p1", nil, false},
		{"replacement only", nil, "p1", false},
		{"both set to the same partition", "p1", "p1", false},
		{"both set to different partitions", "p1", "p2", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := templateAttrs(ctx, t, schemaResp.Schema)
			attrs["ib_partition"] = tftypes.NewValue(tftypes.String, tt.ibPartition)
			attrs["transport_partition_id"] = tftypes.NewValue(tftypes.String, tt.transportID)

			s := schemaResp.Schema
			req := resource.ValidateConfigRequest{
				Config: tfsdk.Config{
					Raw:    tftypes.NewValue(s.Type().TerraformType(ctx), attrs),
					Schema: s,
				},
			}
			resp := &resource.ValidateConfigResponse{}

			(&instanceTemplateResource{}).ValidateConfig(ctx, req, resp)

			if got := resp.Diagnostics.HasError(); got != tt.wantError {
				t.Errorf("HasError() = %t, want %t (diagnostics: %v)", got, tt.wantError, resp.Diagnostics)
			}
		})
	}
}

// runModifierChain drives every plan modifier on one schema attribute in order, the way
// the framework does: the planned value is threaded from one modifier to the next and
// RequiresReplace accumulates across them and cannot be unset.
//
// Driving a modifier in isolation is not enough. An attribute that resolves an unknown in
// one modifier and calls RequiresReplace in another behaves differently depending on the
// order of the slice, and only the chain shows it.
func runModifierChain(ctx context.Context, t *testing.T, attrName string,
	stateValue, configValue, planValue types.String, stateIsNull bool,
) (types.String, bool) {
	t.Helper()

	schemaResp := &resource.SchemaResponse{}
	(&instanceTemplateResource{}).Schema(ctx, resource.SchemaRequest{}, schemaResp)

	s := schemaResp.Schema
	attr, ok := s.Attributes[attrName].(schema.StringAttribute)
	if !ok {
		t.Fatalf("%s is not a StringAttribute", attrName)
	}

	// The raw plan and state objects both have to be present and non-null. RequiresReplace
	// reads them directly to detect create and destroy, and a zero tfsdk.Plan reads as a
	// destroy, which would silently skip the modifier under test.
	rawFor := func(v types.String) tftypes.Value {
		switch {
		case v.IsUnknown():
			return tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
		case v.IsNull():
			return tftypes.NewValue(tftypes.String, nil)
		default:
			return tftypes.NewValue(tftypes.String, v.ValueString())
		}
	}

	objType := s.Type().TerraformType(ctx)

	stateAttrs := templateAttrs(ctx, t, s)
	stateAttrs[attrName] = rawFor(stateValue)
	stateRaw := tftypes.NewValue(objType, stateAttrs)
	if stateIsNull {
		stateRaw = tftypes.NewValue(objType, nil)
	}

	planAttrs := templateAttrs(ctx, t, s)
	planAttrs[attrName] = rawFor(planValue)
	planRaw := tftypes.NewValue(objType, planAttrs)

	req := planmodifier.StringRequest{
		Path:        path.Root(attrName),
		State:       tfsdk.State{Raw: stateRaw, Schema: s},
		Plan:        tfsdk.Plan{Raw: planRaw, Schema: s},
		StateValue:  stateValue,
		ConfigValue: configValue,
		PlanValue:   planValue,
	}

	requiresReplace := false
	for _, modifier := range attr.PlanModifiers {
		resp := &planmodifier.StringResponse{PlanValue: req.PlanValue}
		modifier.PlanModifyString(ctx, req, resp)
		if resp.Diagnostics.HasError() {
			t.Fatalf("modifier %q on %s: %v", modifier.Description(ctx), attrName, resp.Diagnostics)
		}

		// Mirror the framework: thread the planned value forward, accumulate replacement.
		req.PlanValue = resp.PlanValue
		requiresReplace = requiresReplace || resp.RequiresReplace
	}

	return req.PlanValue, requiresReplace
}

// TestInstanceTemplateUserOnlyAttributesDoNotForceReplacement covers reservation_id and
// nvlink_domain_id. Only the user sets either one, so both are null in state whenever the
// configuration omits them, and a null prior value used to leave the planned value unknown
// and recreate the template on every plan.
func TestInstanceTemplateUserOnlyAttributesDoNotForceReplacement(t *testing.T) {
	ctx := context.Background()

	for _, attrName := range []string{"reservation_id", "nvlink_domain_id"} {
		t.Run(attrName, func(t *testing.T) {
			t.Run("omitted and null in state", func(t *testing.T) {
				planValue, requiresReplace := runModifierChain(ctx, t, attrName,
					types.StringNull(), types.StringNull(), types.StringUnknown(), false)

				if requiresReplace {
					t.Error("forces replacement, want none: the value is unchanged and unset")
				}
				if !planValue.IsNull() {
					t.Errorf("planned value = %v, want null", planValue)
				}
			})

			t.Run("omitted but set in state", func(t *testing.T) {
				// Defensive rather than a state the pipeline produces: Terraform carries a
				// non-null prior value forward for an Optional+Computed attribute, so the
				// planned value normally arrives already known and only a null prior value
				// is marked unknown. That is why the pre-existing modifier order never
				// broke project_id or public_ip_address_type, which are never null.
				//
				// It is also the reason Computed cannot simply be dropped here: removing
				// the attribute from a configuration has to stay a silent no-op rather
				// than becoming a forced replacement.
				planValue, requiresReplace := runModifierChain(ctx, t, attrName,
					types.StringValue("v1"), types.StringNull(), types.StringUnknown(), false)

				if requiresReplace {
					t.Error("forces replacement, want none: the prior value is preserved")
				}
				if planValue.ValueString() != "v1" {
					t.Errorf("planned value = %v, want the prior value v1", planValue)
				}
			})

			t.Run("a configured change still forces replacement", func(t *testing.T) {
				_, requiresReplace := runModifierChain(ctx, t, attrName,
					types.StringValue("v1"), types.StringValue("v2"), types.StringValue("v2"), false)

				if !requiresReplace {
					t.Error("does not force replacement, want one: the attribute is immutable")
				}
			})

			t.Run("setting a value on a resource that had none forces replacement", func(t *testing.T) {
				_, requiresReplace := runModifierChain(ctx, t, attrName,
					types.StringNull(), types.StringValue("v1"), types.StringValue("v1"), false)

				if !requiresReplace {
					t.Error("does not force replacement, want one: the attribute is immutable")
				}
			})

			t.Run("create leaves the value unknown", func(t *testing.T) {
				planValue, requiresReplace := runModifierChain(ctx, t, attrName,
					types.StringNull(), types.StringNull(), types.StringUnknown(), true)

				if requiresReplace {
					t.Error("forces replacement on create, want none")
				}
				if !planValue.IsUnknown() {
					t.Errorf("planned value = %v, want unknown so the provider can fill it", planValue)
				}
			})
		})
	}
}

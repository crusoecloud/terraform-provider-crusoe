package common_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

const (
	deprecatedName  = "ib_partition_id"
	replacementName = "transport_partition_id"
)

func TestEffectiveAliasValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		deprecated  types.String
		replacement types.String
		want        types.String
	}{
		{"both null", types.StringNull(), types.StringNull(), types.StringNull()},
		{"deprecated only", types.StringValue("p1"), types.StringNull(), types.StringValue("p1")},
		{"replacement only", types.StringNull(), types.StringValue("p2"), types.StringValue("p2")},
		{"replacement wins", types.StringValue("p1"), types.StringValue("p2"), types.StringValue("p2")},
		{"empty replacement falls back", types.StringValue("p1"), types.StringValue(""), types.StringValue("p1")},
		{"empty deprecated", types.StringValue(""), types.StringValue("p2"), types.StringValue("p2")},
		// An empty string and a null both mean unset, so both resolve to null. Without
		// this, a half holding "" would not compare equal to a half holding null.
		{"both empty resolve to null", types.StringValue(""), types.StringValue(""), types.StringNull()},
		{"unknown replacement falls back", types.StringValue("p1"), types.StringUnknown(), types.StringValue("p1")},
		{"unknown deprecated is unset", types.StringUnknown(), types.StringNull(), types.StringNull()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := common.EffectiveAliasValue(tt.deprecated, tt.replacement)
			if !got.Equal(tt.want) {
				t.Errorf("EffectiveAliasValue(%v, %v) = %v, want %v", tt.deprecated, tt.replacement, got, tt.want)
			}
		})
	}
}

func TestEffectiveAliasString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		deprecated  string
		replacement string
		want        string
	}{
		{"both empty", "", "", ""},
		{"deprecated only", "p1", "", "p1"},
		{"replacement only", "", "p2", "p2"},
		{"replacement wins", "p1", "p2", "p2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := common.EffectiveAliasString(tt.deprecated, tt.replacement); got != tt.want {
				t.Errorf("EffectiveAliasString(%q, %q) = %q, want %q", tt.deprecated, tt.replacement, got, tt.want)
			}
		})
	}
}

func TestAliasPairConflicts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		deprecated  types.String
		replacement types.String
		want        bool
	}{
		{"neither set", types.StringNull(), types.StringNull(), false},
		{"deprecated only", types.StringValue("p1"), types.StringNull(), false},
		{"replacement only", types.StringNull(), types.StringValue("p1"), false},
		// Both set to the same partition is legal, so a configuration written mid-migration
		// keeps working.
		{"both set to same value", types.StringValue("p1"), types.StringValue("p1"), false},
		{"both set to different values", types.StringValue("p1"), types.StringValue("p2"), true},
		{"unknown is not a conflict", types.StringValue("p1"), types.StringUnknown(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := common.AliasPairConflicts(tt.deprecated, tt.replacement); got != tt.want {
				t.Errorf("AliasPairConflicts(%v, %v) = %t, want %t", tt.deprecated, tt.replacement, got, tt.want)
			}
		})
	}
}

// aliasPairSchema is a two-attribute schema standing in for a resource that carries an
// alias pair, so the modifier can be driven against real tfsdk plan and state objects.
func aliasPairSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			deprecatedName:  schema.StringAttribute{Optional: true},
			replacementName: schema.StringAttribute{Optional: true},
		},
	}
}

// aliasPairValue builds a raw object value for aliasPairSchema. A nil half is null.
func aliasPairValue(ctx context.Context, s schema.Schema, deprecated, replacement *string) tftypes.Value {
	objType := s.Type().TerraformType(ctx)

	toTF := func(v *string) tftypes.Value {
		if v == nil {
			return tftypes.NewValue(tftypes.String, nil)
		}

		return tftypes.NewValue(tftypes.String, *v)
	}

	return tftypes.NewValue(objType, map[string]tftypes.Value{
		deprecatedName:  toTF(deprecated),
		replacementName: toTF(replacement),
	})
}

func strPtr(s string) *string { return &s }

// runAliasModifier drives the full RequiresReplaceIf modifier at the given attribute, not
// the bare ifFunc. Only the wrapper applies the framework's create, destroy, and
// no-change early-outs, which are part of the behavior under test.
func runAliasModifier(ctx context.Context, t *testing.T, attrName string,
	stateRaw, planRaw tftypes.Value,
) *planmodifier.StringResponse {
	t.Helper()

	s := aliasPairSchema()
	attrPath := path.Root(attrName)

	state := tfsdk.State{Raw: stateRaw, Schema: s}
	plan := tfsdk.Plan{Raw: planRaw, Schema: s}

	var stateValue, planValue types.String
	if !stateRaw.IsNull() {
		if diags := state.GetAttribute(ctx, attrPath, &stateValue); diags.HasError() {
			t.Fatalf("failed to read state attribute %s: %v", attrName, diags)
		}
	}
	if !planRaw.IsNull() {
		if diags := plan.GetAttribute(ctx, attrPath, &planValue); diags.HasError() {
			t.Fatalf("failed to read plan attribute %s: %v", attrName, diags)
		}
	}

	req := planmodifier.StringRequest{
		Path:       attrPath,
		State:      state,
		Plan:       plan,
		StateValue: stateValue,
		PlanValue:  planValue,
	}

	resp := &planmodifier.StringResponse{PlanValue: planValue}

	modifier := stringplanmodifier.RequiresReplaceIf(
		common.AliasPairRequiresReplaceIf(deprecatedName, replacementName), "", "")
	modifier.PlanModifyString(ctx, req, resp)

	return resp
}

// aliasPairReplaceVerdict runs the modifier on both halves of the pair and reports whether
// either one asks for replacement.
//
// The union is the real behavior: Terraform records a replacement when any attribute's
// modifier asks for one. Asserting each half in isolation would be wrong, because the
// framework skips the ifFunc for a half whose own value did not change, so the unchanged
// half always reports false no matter what the pair as a whole is doing.
func aliasPairReplaceVerdict(ctx context.Context, t *testing.T, stateRaw, planRaw tftypes.Value) bool {
	t.Helper()

	verdict := false
	for _, attrName := range []string{deprecatedName, replacementName} {
		resp := runAliasModifier(ctx, t, attrName, stateRaw, planRaw)
		if resp.Diagnostics.HasError() {
			t.Fatalf("modifier on %s produced errors: %v", attrName, resp.Diagnostics)
		}
		verdict = verdict || resp.RequiresReplace
	}

	return verdict
}

// TestAliasPairRequiresReplaceIf covers each way a configuration can move between the two
// names of an alias pair.
func TestAliasPairRequiresReplaceIf(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := aliasPairSchema()
	objType := s.Type().TerraformType(ctx)
	nullObject := tftypes.NewValue(objType, nil)

	tests := []struct {
		name                              string
		stateDeprecated, stateReplacement *string
		planDeprecated, planReplacement   *string
		stateIsNull, planIsNull           bool
		wantReplace                       bool
	}{
		{
			name:        "create",
			stateIsNull: true, planReplacement: strPtr("p1"),
			wantReplace: false,
		},
		{
			name:            "destroy",
			stateDeprecated: strPtr("p1"), planIsNull: true,
			wantReplace: false,
		},
		{
			// The case this whole change exists for.
			name:            "rename to replacement, same partition",
			stateDeprecated: strPtr("p1"),
			planReplacement: strPtr("p1"),
			wantReplace:     false,
		},
		{
			name:            "rename to replacement, different partition",
			stateDeprecated: strPtr("p1"),
			planReplacement: strPtr("p2"),
			wantReplace:     true,
		},
		{
			name:            "change value on deprecated half",
			stateDeprecated: strPtr("p1"),
			planDeprecated:  strPtr("p2"),
			wantReplace:     true,
		},
		{
			name:             "change value on replacement half",
			stateReplacement: strPtr("p1"),
			planReplacement:  strPtr("p2"),
			wantReplace:      true,
		},
		{
			name:            "unset the partition",
			stateDeprecated: strPtr("p1"),
			wantReplace:     true,
		},
		{
			name:            "set the partition on a resource that had none",
			planReplacement: strPtr("p1"),
			wantReplace:     true,
		},
		{
			name:            "both halves set to the same value",
			stateDeprecated: strPtr("p1"),
			planDeprecated:  strPtr("p1"), planReplacement: strPtr("p1"),
			wantReplace: false,
		},
		{
			// A refreshed state carries both names; a config on the replacement is a no-op.
			name:            "state carries both names, config uses replacement",
			stateDeprecated: strPtr("p1"), stateReplacement: strPtr("p1"),
			planReplacement: strPtr("p1"),
			wantReplace:     false,
		},
		{
			// Stale state from before this change: only the deprecated half is populated.
			name:            "stale state without the replacement half",
			stateDeprecated: strPtr("p1"),
			planReplacement: strPtr("p1"),
			wantReplace:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stateRaw := aliasPairValue(ctx, s, tt.stateDeprecated, tt.stateReplacement)
			if tt.stateIsNull {
				stateRaw = nullObject
			}
			planRaw := aliasPairValue(ctx, s, tt.planDeprecated, tt.planReplacement)
			if tt.planIsNull {
				planRaw = nullObject
			}

			if got := aliasPairReplaceVerdict(ctx, t, stateRaw, planRaw); got != tt.wantReplace {
				t.Errorf("RequiresReplace across the pair = %t, want %t", got, tt.wantReplace)
			}
		})
	}
}

// TestAliasPairRequiresReplaceIfUnknown pins the conservative fallback. An unresolved
// interpolation reads as unset, which would make a rename look like a removal, so the
// modifier forces replacement instead of guessing.
func TestAliasPairRequiresReplaceIfUnknown(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := aliasPairSchema()
	objType := s.Type().TerraformType(ctx)

	stateRaw := aliasPairValue(ctx, s, strPtr("p1"), nil)
	planRaw := tftypes.NewValue(objType, map[string]tftypes.Value{
		deprecatedName:  tftypes.NewValue(tftypes.String, nil),
		replacementName: tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	})

	resp := runAliasModifier(ctx, t, deprecatedName, stateRaw, planRaw)
	if resp.Diagnostics.HasError() {
		t.Fatalf("modifier produced errors: %v", resp.Diagnostics)
	}
	if !resp.RequiresReplace {
		t.Error("RequiresReplace = false, want true for an unknown planned value")
	}
}

// TestAliasPairRequiresReplaceIfNestedPath checks that the sibling path is derived
// correctly for an attribute inside a list, which is how the compute instance host
// channel adapters are shaped.
func TestAliasPairRequiresReplaceIfNestedPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	const listName = "host_channel_adapters"

	s := schema.Schema{
		Attributes: map[string]schema.Attribute{
			listName: schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						deprecatedName:  schema.StringAttribute{Optional: true},
						replacementName: schema.StringAttribute{Optional: true},
					},
				},
			},
		},
	}

	objType := s.Type().TerraformType(ctx)

	schemaObject, ok := objType.(tftypes.Object)
	if !ok {
		t.Fatalf("schema type is not an object")
	}
	listType, ok := schemaObject.AttributeTypes[listName].(tftypes.List)
	if !ok {
		t.Fatalf("%s attribute is not a list", listName)
	}

	elemType := listType.ElementType

	elem := func(deprecated, replacement *string) tftypes.Value {
		toTF := func(v *string) tftypes.Value {
			if v == nil {
				return tftypes.NewValue(tftypes.String, nil)
			}

			return tftypes.NewValue(tftypes.String, *v)
		}

		return tftypes.NewValue(elemType, map[string]tftypes.Value{
			deprecatedName:  toTF(deprecated),
			replacementName: toTF(replacement),
		})
	}

	list := func(elems ...tftypes.Value) tftypes.Value {
		return tftypes.NewValue(objType, map[string]tftypes.Value{
			listName: tftypes.NewValue(tftypes.List{ElementType: elemType}, elems),
		})
	}

	tests := []struct {
		name        string
		stateRaw    tftypes.Value
		planRaw     tftypes.Value
		wantReplace bool
	}{
		{
			name:        "rename inside a list element",
			stateRaw:    list(elem(strPtr("p1"), nil)),
			planRaw:     list(elem(nil, strPtr("p1"))),
			wantReplace: false,
		},
		{
			name:        "partition change inside a list element",
			stateRaw:    list(elem(strPtr("p1"), nil)),
			planRaw:     list(elem(nil, strPtr("p2"))),
			wantReplace: true,
		},
		{
			// The state list is shorter than the plan list, so the sibling read runs past
			// the end of the state. That has to yield a null rather than an error.
			name:        "state list shorter than plan list",
			stateRaw:    list(),
			planRaw:     list(elem(nil, strPtr("p1"))),
			wantReplace: true,
		},
	}

	elemPath := path.Root(listName).AtListIndex(0)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			verdict := false
			for _, attrName := range []string{deprecatedName, replacementName} {
				attrPath := elemPath.AtName(attrName)

				state := tfsdk.State{Raw: tt.stateRaw, Schema: s}
				plan := tfsdk.Plan{Raw: tt.planRaw, Schema: s}

				var stateValue, planValue types.String
				// Diagnostics are ignored here: reading past the end of the state list is
				// the case under test and yields a null.
				_ = state.GetAttribute(ctx, attrPath, &stateValue)
				if diags := plan.GetAttribute(ctx, attrPath, &planValue); diags.HasError() {
					t.Fatalf("failed to read plan attribute: %v", diags)
				}

				req := planmodifier.StringRequest{
					Path:       attrPath,
					State:      state,
					Plan:       plan,
					StateValue: stateValue,
					PlanValue:  planValue,
				}
				resp := &planmodifier.StringResponse{PlanValue: planValue}

				modifier := stringplanmodifier.RequiresReplaceIf(
					common.AliasPairRequiresReplaceIf(deprecatedName, replacementName), "", "")
				modifier.PlanModifyString(ctx, req, resp)

				if resp.Diagnostics.HasError() {
					t.Fatalf("modifier on %s produced errors: %v", attrName, resp.Diagnostics)
				}
				verdict = verdict || resp.RequiresReplace
			}

			if verdict != tt.wantReplace {
				t.Errorf("RequiresReplace across the pair = %t, want %t", verdict, tt.wantReplace)
			}
		})
	}
}

// nullableComputedSchema stands in for an immutable attribute that only the user sets:
// Optional and Computed, so an omitted value is null in state rather than absent.
func nullableComputedSchema() schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"nvlink_domain_id": schema.StringAttribute{Optional: true, Computed: true},
		},
	}
}

// TestUseStateForUnknownIncludingNull covers the difference from the framework's
// UseStateForUnknown, which gives up on a null prior value and so leaves the plan unknown.
// Paired with RequiresReplace that recreates the resource on every plan.
func TestUseStateForUnknownIncludingNull(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	s := nullableComputedSchema()
	objType := s.Type().TerraformType(ctx)

	raw := func(v *string) tftypes.Value {
		val := tftypes.NewValue(tftypes.String, nil)
		if v != nil {
			val = tftypes.NewValue(tftypes.String, *v)
		}

		return tftypes.NewValue(objType, map[string]tftypes.Value{"nvlink_domain_id": val})
	}

	tests := []struct {
		name        string
		stateIsNull bool
		stateValue  types.String
		configValue types.String
		planValue   types.String
		want        types.String
	}{
		{
			// The bug: an omitted value is null in state, the framework marks the Computed
			// attribute unknown, and unknown never equals null.
			name:       "null prior value is preserved",
			stateValue: types.StringNull(), configValue: types.StringNull(),
			planValue: types.StringUnknown(),
			want:      types.StringNull(),
		},
		{
			// Unchanged from UseStateForUnknown, so removing a value from a configuration
			// stays a silent no-op rather than becoming a forced replacement.
			name:       "non-null prior value is preserved",
			stateValue: types.StringValue("nvd-1"), configValue: types.StringNull(),
			planValue: types.StringUnknown(),
			want:      types.StringValue("nvd-1"),
		},
		{
			name:       "a configured value wins",
			stateValue: types.StringNull(), configValue: types.StringValue("nvd-2"),
			planValue: types.StringValue("nvd-2"),
			want:      types.StringValue("nvd-2"),
		},
		{
			// An unresolved interpolation has to stay unknown.
			name:       "unknown configuration value stays unknown",
			stateValue: types.StringNull(), configValue: types.StringUnknown(),
			planValue: types.StringUnknown(),
			want:      types.StringUnknown(),
		},
		{
			// On create the provider still has to fill the attribute in.
			name:        "create leaves the value unknown",
			stateIsNull: true,
			stateValue:  types.StringNull(), configValue: types.StringNull(),
			planValue: types.StringUnknown(),
			want:      types.StringUnknown(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stateRaw := raw(nil)
			if !tt.stateValue.IsNull() {
				v := tt.stateValue.ValueString()
				stateRaw = raw(&v)
			}
			if tt.stateIsNull {
				stateRaw = tftypes.NewValue(objType, nil)
			}

			req := planmodifier.StringRequest{
				Path:        path.Root("nvlink_domain_id"),
				State:       tfsdk.State{Raw: stateRaw, Schema: s},
				StateValue:  tt.stateValue,
				ConfigValue: tt.configValue,
				PlanValue:   tt.planValue,
			}
			resp := &planmodifier.StringResponse{PlanValue: tt.planValue}

			common.UseStateForUnknownIncludingNull().PlanModifyString(ctx, req, resp)

			if resp.Diagnostics.HasError() {
				t.Fatalf("unexpected diagnostics: %v", resp.Diagnostics)
			}
			if !resp.PlanValue.Equal(tt.want) {
				t.Errorf("plan value = %v, want %v", resp.PlanValue, tt.want)
			}
		})
	}
}

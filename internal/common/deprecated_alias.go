package common

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// An alias pair is a deprecated attribute and the attribute that replaces it. Both names
// address the same value, so renaming one to the other in a configuration is not a change
// to the resource. The helpers below let a resource treat such a pair as one value.

// aliasValueIsSet reports whether one half of an alias pair holds a usable value. It
// matches the isSet convention used for the firewall rule source/sources pair: null,
// unknown, and empty all mean unset.
func aliasValueIsSet(s types.String) bool {
	return !s.IsNull() && !s.IsUnknown() && s.ValueString() != ""
}

// EffectiveAliasValue resolves an alias pair to the single value it names. The replacement
// wins when it is set, otherwise the deprecated attribute supplies the value. An unset pair
// resolves to null, so that a half holding an empty string and a half holding null compare
// as the same absent value.
func EffectiveAliasValue(deprecated, replacement types.String) types.String {
	if aliasValueIsSet(replacement) {
		return replacement
	}

	if aliasValueIsSet(deprecated) {
		return deprecated
	}

	return types.StringNull()
}

// EffectiveAliasString resolves an alias pair held as plain Go strings, for models that
// flatten a nested object into string fields. An empty string means unset.
func EffectiveAliasString(deprecated, replacement string) string {
	if replacement != "" {
		return replacement
	}

	return deprecated
}

// AliasPairConflicts reports whether both halves of an alias pair are set to different
// values, which does not say which value the resource should use.
//
// Both halves holding the same value is not a conflict, so a configuration written during
// the migration keeps working.
func AliasPairConflicts(deprecated, replacement types.String) bool {
	if !aliasValueIsSet(deprecated) || !aliasValueIsSet(replacement) {
		return false
	}

	return deprecated.ValueString() != replacement.ValueString()
}

// AliasPairUseStateForUnknown returns UseStateForUnknown for one Computed half of an alias
// pair whose value can change in place. Apply it to both halves, passing the same two names.
//
// The configuration usually sets only one name, and the other is filled in from the API.
// Plain UseStateForUnknown plans that other half as its prior value, so changing the value
// the pair names plans the two halves as different values, and the apply cannot satisfy
// both. This modifier plans the configured value on the unconfigured half when the value
// the pair names changes, and keeps the prior value otherwise, so a rename from the
// deprecated name to its replacement stays a no-op.
//
// The names are attribute names, not paths, as in AliasPairRequiresReplaceIf.
func AliasPairUseStateForUnknown(deprecatedName, replacementName string) planmodifier.String {
	return aliasPairUseStateForUnknownModifier{
		deprecatedName:  deprecatedName,
		replacementName: replacementName,
	}
}

type aliasPairUseStateForUnknownModifier struct {
	deprecatedName  string
	replacementName string
}

func (m aliasPairUseStateForUnknownModifier) Description(_ context.Context) string {
	return "Once the resource exists, the value of this attribute in state is preserved, " +
		"unless the configuration changes the value it shares with " + m.deprecatedName + " / " +
		m.replacementName + "."
}

func (m aliasPairUseStateForUnknownModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

//nolint:gocritic // hugeParam: req signature required by planmodifier.String interface
func (m aliasPairUseStateForUnknownModifier) PlanModifyString(ctx context.Context,
	req planmodifier.StringRequest, resp *planmodifier.StringResponse,
) {
	// On create there is no prior value to preserve, as in UseStateForUnknown.
	if req.StateValue.IsNull() {
		return
	}

	// A known planned value comes from the configuration; leave it alone.
	if !req.PlanValue.IsUnknown() {
		return
	}

	// An unknown configuration value has to stay unknown, or interpolation breaks.
	if req.ConfigValue.IsUnknown() {
		return
	}

	parent := req.Path.ParentPath()
	deprecatedPath := parent.AtName(m.deprecatedName)
	replacementPath := parent.AtName(m.replacementName)

	var stateDeprecated, stateReplacement, configDeprecated, configReplacement types.String

	resp.Diagnostics.Append(req.State.GetAttribute(ctx, deprecatedPath, &stateDeprecated)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, replacementPath, &stateReplacement)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, deprecatedPath, &configDeprecated)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, replacementPath, &configReplacement)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The configured half is not known yet, so neither is the value this half follows.
	if configDeprecated.IsUnknown() || configReplacement.IsUnknown() {
		return
	}

	effectiveConfig := EffectiveAliasValue(configDeprecated, configReplacement)
	if effectiveConfig.IsNull() || effectiveConfig.Equal(EffectiveAliasValue(stateDeprecated, stateReplacement)) {
		resp.PlanValue = req.StateValue

		return
	}

	resp.PlanValue = effectiveConfig
}

// AliasPairRequiresReplaceIf builds a RequiresReplaceIfFunc for one half of an alias pair.
// It forces replacement only when the value the pair names changes, so a user who renames
// the deprecated attribute to its replacement gets an in-place update instead of a
// destroyed resource. Apply it to both halves of the pair, passing the same two names.
//
// The names are attribute names, not paths. The sibling path is derived from the path being
// modified, so the same func works for a top-level attribute and for one nested inside a
// list.
func AliasPairRequiresReplaceIf(deprecatedName, replacementName string) stringplanmodifier.RequiresReplaceIfFunc {
	//nolint:gocritic // hugeParam: req signature required by stringplanmodifier.RequiresReplaceIfFunc
	return func(ctx context.Context, req planmodifier.StringRequest,
		resp *stringplanmodifier.RequiresReplaceIfFuncResponse,
	) {
		parent := req.Path.ParentPath()
		deprecatedPath := parent.AtName(deprecatedName)
		replacementPath := parent.AtName(replacementName)

		// Both halves are read from req.Plan and req.State rather than from
		// req.PlanValue/req.StateValue. The framework passes the original plan to every
		// attribute and stores attributes in a map, so reading the request keeps the two
		// halves of the pair from reaching different verdicts in a different order.
		var stateDeprecated, stateReplacement, planDeprecated, planReplacement types.String

		resp.Diagnostics.Append(req.State.GetAttribute(ctx, deprecatedPath, &stateDeprecated)...)
		resp.Diagnostics.Append(req.State.GetAttribute(ctx, replacementPath, &stateReplacement)...)
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, deprecatedPath, &planDeprecated)...)
		resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, replacementPath, &planReplacement)...)
		if resp.Diagnostics.HasError() {
			return
		}

		// An unresolved interpolation reads as unset, which would make a rename look like a
		// removal. Replacement is always safe, and it is what this attribute did before the
		// alias pair existed, so fall back to it when the plan is not yet knowable.
		if planDeprecated.IsUnknown() || planReplacement.IsUnknown() {
			resp.RequiresReplace = true

			return
		}

		effectiveState := EffectiveAliasValue(stateDeprecated, stateReplacement)
		effectivePlan := EffectiveAliasValue(planDeprecated, planReplacement)

		resp.RequiresReplace = !effectiveState.Equal(effectivePlan)
	}
}

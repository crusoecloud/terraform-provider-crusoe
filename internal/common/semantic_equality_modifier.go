package common

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// SemanticEqualityStringModifier suppresses a plan diff when the stored value
// and the configured one mean the same thing, by planning the stored value.
//
// A custom type's StringSemanticEquals is not enough on its own. The framework
// applies semantic equality in CreateResource, ReadResource, UpdateResource and
// ReadDataSource — but NOT in PlanResourceChange. So an attribute whose stored
// value legitimately differs in spelling from the configured one still produces
// a diff at plan time.
//
// That is reachable through `terraform import`: ImportState seeds almost
// nothing, so Read has no prior value to preserve and state takes the API's
// spelling. Every later plan then compares that against the configuration with
// no semantic equality in play. On an attribute that is also immutable, the
// difference is not merely a spurious diff — it is a hard error.
//
// This modifier is the framework's equivalent of the older SDK's
// DiffSuppressFunc, and like it, the stored value is what survives.
//
// Ordering matters. Place this before any modifier that compares the plan
// against state (ImmutableStringModifier, RequiresReplaceIf), so those see the
// normalized value.
//
// The comparison is supplied rather than taken from the value's own type
// because planmodifier.StringRequest carries a plain types.String: a custom
// type is erased at this boundary, so the modifier cannot recover the
// attribute's semantic equality by itself.
type SemanticEqualityStringModifier struct {
	// equal reports whether the stored and planned values mean the same thing.
	equal func(stateValue, planValue string) bool
	// meaning describes what the two values share, for the plan description —
	// e.g. "same Kubernetes version".
	meaning string
}

// NewSemanticEqualityStringModifier builds a SemanticEqualityStringModifier.
// meaning completes the sentence "suppresses the difference when both values
// are the ...".
func NewSemanticEqualityStringModifier(
	meaning string, equal func(stateValue, planValue string) bool,
) SemanticEqualityStringModifier {
	return SemanticEqualityStringModifier{equal: equal, meaning: meaning}
}

func (m SemanticEqualityStringModifier) Description(_ context.Context) string {
	return fmt.Sprintf("Keeps the stored value when it and the configured value are the %s.", m.meaning)
}

func (m SemanticEqualityStringModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

//nolint:gocritic // hugeParam: req signature required by planmodifier.String interface
func (m SemanticEqualityStringModifier) PlanModifyString(
	_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse,
) {
	// Nothing stored to prefer (create), or nothing concrete planned to compare
	// against — another modifier may still resolve it.
	if req.StateValue.IsNull() || req.StateValue.IsUnknown() ||
		req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {

		return
	}

	// Already identical: leave the plan alone rather than rewriting it to an
	// equal value.
	if req.StateValue.Equal(req.PlanValue) {
		return
	}

	if m.equal == nil || !m.equal(req.StateValue.ValueString(), req.PlanValue.ValueString()) {
		return
	}

	resp.PlanValue = req.StateValue
}

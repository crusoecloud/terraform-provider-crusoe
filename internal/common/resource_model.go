package common

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// ErrGetResourceModel is returned by GetResourceModel when extracting the
// resource model fails. The underlying diagnostics are appended to the
// caller's respDiags, so callers typically just return on a non-nil error.
var ErrGetResourceModel = errors.New("unable to get resource model")

// TFDataGetter is implemented by tfsdk.State, tfsdk.Plan, and tfsdk.Config.
type TFDataGetter interface {
	Get(ctx context.Context, target interface{}) diag.Diagnostics
}

// GetResourceModel extracts a resource model from state, plan, or config into
// dest. Returns ErrGetResourceModel if there were errors (already appended to
// respDiags). The model type is inferred from dest, so callers do not need to
// specify the type parameter explicitly.
func GetResourceModel[T any](ctx context.Context, source TFDataGetter, dest *T, respDiags *diag.Diagnostics) error {
	diags := source.Get(ctx, dest)
	respDiags.Append(diags...)

	if respDiags.HasError() {
		return ErrGetResourceModel
	}

	return nil
}

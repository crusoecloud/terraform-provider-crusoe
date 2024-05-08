package internal

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// RunningInstanceCountValidator validates that the given configuration value matches the validator's RunningInstanceCount pattern.
type RunningInstanceCountValidator struct{}

func (v RunningInstanceCountValidator) Description(ctx context.Context) string {
	return "Number of instances must be greater than 0"
}

func (v RunningInstanceCountValidator) MarkdownDescription(ctx context.Context) string {
	return "Number of instances must be greater than 0"
}

//nolint:gocritic // Implements Terraform defined interface
func (v RunningInstanceCountValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	// skip validation if the value is still unknown, which is the case for vars before evaluation
	if req.ConfigValue.IsUnknown() {
		return
	}

	numInstances := req.ConfigValue.ValueInt64()

	if numInstances < 0 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid argument",
			"Number of running instances must be greater than or equal to 0.")
	}
}

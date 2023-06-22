package internal

import (
	"context"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// StorageSizeValidator validates that a given data storage size is accepted by the storage API.
type StorageSizeValidator struct{}

func (v StorageSizeValidator) Description(ctx context.Context) string {
	return "Storage size must be in GiB or TiB"
}

func (v StorageSizeValidator) MarkdownDescription(ctx context.Context) string {
	return "Storage size must be in GiB or TiB"
}

//nolint:gocritic // Implements Terraform defined interface
func (v StorageSizeValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	input := req.ConfigValue.ValueString()
	input = strings.ToLower(input)

	if !strings.HasSuffix(input, "gib") && !strings.HasSuffix(input, "tib") {
		resp.Diagnostics.AddAttributeError(req.Path, "Unsupported Data Size",
			"Storage size must be in GiB or TiB")

		return
	}

	size := input[0 : len(input)-3]
	if _, err := strconv.Atoi(size); err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Unsupported Data Size",
			"Storage size must be in GiB or TiB, e.g. 100GiB")
	}
}

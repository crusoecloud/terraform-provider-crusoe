package internal

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// RegexValidator validates that the given configuration value matches the validator's regex pattern.
type RegexValidator struct {
	RegexPattern string
}

func (v RegexValidator) Description(ctx context.Context) string {
	return fmt.Sprintf("String must conform to the pattern %v", v.RegexPattern)
}

func (v RegexValidator) MarkdownDescription(ctx context.Context) string {
	return fmt.Sprintf("String must conform to the pattern %v", v.RegexPattern)
}

//nolint:gocritic // Implements Terraform defined interface
func (v RegexValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	r := regexp.MustCompile(v.RegexPattern)

	if !r.MatchString(req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid argument",
			fmt.Sprintf("%s must conform to the regex pattern /%s/, which %q does not.", req.Path.String(), v.RegexPattern, req.ConfigValue.ValueString()))
	}
}

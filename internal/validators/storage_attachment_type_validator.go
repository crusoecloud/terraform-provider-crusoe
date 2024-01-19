package internal

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type DiskModeType string

const (
	DiskReadOnly  DiskModeType = "read-only"
	DiskReadWrite DiskModeType = "read-write"
)

// StorageModeValidator validates that a given data storage size is accepted by the storage API.
type StorageModeValidator struct{}

func (v StorageModeValidator) Description(ctx context.Context) string {
	return "Disk attachment type must be either 'disk-readonly' or 'disk-readwrite'"
}

func (v StorageModeValidator) MarkdownDescription(ctx context.Context) string {
	return "Disk attachment type must be either 'disk-readonly' or 'disk-readwrite'"
}

//nolint:gocritic // Implements Terraform defined interface
func (v StorageModeValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	input := req.ConfigValue.ValueString()
	input = strings.ToLower(input)

	if input != string(DiskReadOnly) && input != string(DiskReadWrite) {
		resp.Diagnostics.AddAttributeError(req.Path, "Unsupported Disk Mode Type",
			"Disk mode type must be either 'read-only' or 'read-write'")
	}
}

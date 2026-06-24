// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package disk

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type diskTypeModifier struct{}

// Description returns a human-readable description of the plan modifier.
func (m diskTypeModifier) Description(_ context.Context) string {
	return "If no value is set, Crusoe Terraform Provider will pick default and warn user."
}

// MarkdownDescription returns a markdown description of the plan modifier.
func (m diskTypeModifier) MarkdownDescription(_ context.Context) string {
	return "If no value is set, Crusoe Terraform Provider will pick default and warn user."
}

// PlanModifyString implements the plan modification logic.
//
//nolint:gocritic // Implements Terraform defined interface
func (m diskTypeModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// If being destroyed, ignore
	if req.Plan.Raw.IsNull() {
		return
	}

	// if have valid state use that if default plan value
	if !req.StateValue.IsNull() && !req.StateValue.IsUnknown() {
		if req.PlanValue.ValueString() == defaultDiskType {
			resp.PlanValue = req.StateValue
			resp.Diagnostics.AddWarning("Disk Type should be specified",
				fmt.Sprintf("Disk Type was not specified. Using current value: type = %s. This field will be required in a future release.", resp.PlanValue))
		}

		return
	}

	// if plan value is default, set to the proper default and warn
	if req.PlanValue.ValueString() == defaultDiskType {
		resp.PlanValue = types.StringValue(persistentSSD)
		resp.Diagnostics.AddWarning("Disk Type should be specified",
			fmt.Sprintf("Disk Type was not specified. Using default value: type = %s. This field will be required in a future release.", resp.PlanValue))
	}
}

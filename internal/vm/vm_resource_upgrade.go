package vm

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// vmResourceModelV0 is the minimal set of attributes that we will need from a prior state to
// rebuild an instance's state because these fields are not returned by the API.
type vmResourceModelV0 struct {
	ID     types.String `tfsdk:"id"`
	SSHKey types.String `tfsdk:"ssh_key"`
	Image  types.String `tfsdk:"image"`
}

func (r *vmResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed: true,
					},
					"ssh_key": schema.StringAttribute{
						Required: true,
					},
					"image": schema.StringAttribute{
						Optional: true,
					},
				},
			},
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var priorStateData vmResourceModelV0
				resp.Diagnostics.Append(req.State.Get(ctx, &priorStateData)...)

				if resp.Diagnostics.HasError() {
					return
				}

				if priorStateData.ID.IsNull() {
					resp.Diagnostics.AddError("Failed to migrate instance to current version",
						"No ID was associated with the instance.")

					return
				}

				// Note: we will iterate through all projects to find the instance. This means we are not dependent
				// on project ID being present in the previous state, which allows us to be backwards-compatible with
				// more versions.
				instance, err := findInstance(ctx, r.client, priorStateData.ID.ValueString())
				if err != nil {
					resp.Diagnostics.AddError("Failed to migrate instance to current version",
						fmt.Sprintf("There was an error migrating the instance to the current version: %v", err))

					return
				}

				var state vmResourceModel

				vmToTerraformResourceModel(instance, &state)
				state.SSHKey = priorStateData.SSHKey
				state.Image = priorStateData.Image

				resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
				if resp.Diagnostics.HasError() {
					resp.Diagnostics.AddError("Failed to migrate instance to current version",
						"There was an error migrating the instance to the current version.")

					return
				}
				resp.Diagnostics.AddWarning("Successfully migrated instance to current version",
					"Terraform State has been successfully migrated to a new version. Please refer to"+
						" docs.crusoecloud.com for information about the updates.")
			},
		},
	}
}

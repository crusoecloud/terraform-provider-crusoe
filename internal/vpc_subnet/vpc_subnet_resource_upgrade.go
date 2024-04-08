package vpc_subnet

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// vpcSubnetModelV0 is the minimal set of attributes that we will need from a prior state to
// rebuild a VPC subnet's state because these fields are not returned by the API.
type vpcSubnetModelV0 struct {
	ID types.String `tfsdk:"id"`
}

func (r *vpcSubnetResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed: true,
					},
				},
			},
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var priorStateData vpcSubnetModelV0
				resp.Diagnostics.Append(req.State.Get(ctx, &priorStateData)...)

				if resp.Diagnostics.HasError() {
					return
				}

				if priorStateData.ID.IsNull() {
					resp.Diagnostics.AddError("Failed to migrate VPC subnet to current version",
						"No ID was associated with the VPC subnet.")

					return
				}

				// Note: we will iterate through all projects to find the VPC subnet. This means we are not dependent
				// on project ID being present in the previous state, which allows us to be backwards-compatible with
				// more versions.
				vpcSubnet, projectID, err := findVpcSubnet(ctx, r.client, priorStateData.ID.ValueString())
				if err != nil {
					resp.Diagnostics.AddError("Failed to migrate VPC subnet to current version",
						fmt.Sprintf("There was an error migrating the VPC subnet to the current version: %v",
							err))

					return
				}

				var state vpcSubnetResourceModel
				state.ProjectID = types.StringValue(projectID)
				vpcSubnetToTerraformResourceModel(vpcSubnet, &state)

				resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
				if resp.Diagnostics.HasError() {
					resp.Diagnostics.AddError("Failed to migrate VPC subnet to current version",
						"There was an error migrating the VPC subnet to the current version.")

					return
				}
				resp.Diagnostics.AddWarning("Successfully migrated VPC subnet to current version",
					"Terraform State has been successfully migrated to a new version. Please refer to"+
						" docs.crusoecloud.com for information about the updates.")
			},
		},
	}
}

package vpc_network

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// vpcNetworkModelV0 is the minimal set of attributes that we will need from a prior state to
// rebuild a VPC network's state because these fields are not returned by the API.
type vpcNetworkModelV0 struct {
	ID types.String `tfsdk:"id"`
}

func (r *vpcNetworkResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
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
				var priorStateData vpcNetworkModelV0
				resp.Diagnostics.Append(req.State.Get(ctx, &priorStateData)...)

				if resp.Diagnostics.HasError() {
					return
				}

				if priorStateData.ID.IsNull() {
					resp.Diagnostics.AddError("Failed to migrate VPC network to current version",
						"No ID was associated with the VPC network.")

					return
				}

				// Note: we will iterate through all projects to find the VPC network. This means we are not dependent
				// on project ID being present in the previous state, which allows us to be backwards-compatible with
				// more versions.
				vpcNetwork, projectID, err := findVpcNetwork(ctx, r.client.APIClient, priorStateData.ID.ValueString())
				if err != nil {
					resp.Diagnostics.AddError("Failed to migrate VPC network to current version",
						fmt.Sprintf("There was an error migrating the VPC network to the current version: %v",
							err))

					return
				}

				var state vpcNetworkResourceModel
				state.ProjectID = types.StringValue(projectID)
				vpcNetworkToTerraformResourceModel(vpcNetwork, &state)

				resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
				if resp.Diagnostics.HasError() {
					resp.Diagnostics.AddError("Failed to migrate VPC network to current version",
						"There was an error migrating the VPC network to the current version.")

					return
				}
				resp.Diagnostics.AddWarning("Successfully migrated VPC network to current version",
					"Terraform State has been successfully migrated to a new version. Please refer to"+
						" docs.crusoecloud.com for information about the updates.")
			},
		},
	}
}

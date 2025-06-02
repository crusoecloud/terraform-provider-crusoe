package vpc_subnet

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
)

// vpcSubnetModelV0 is the minimal set of attributes that we will need from a prior state to
// rebuild a VPC subnet's state because these fields are not returned by the API.
type vpcSubnetModelV0 struct {
	ID types.String `tfsdk:"id"`
}

type vpcSubnetModelV1 struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
	CIDR      types.String `tfsdk:"cidr"`
	Location  types.String `tfsdk:"location"`
	Network   types.String `tfsdk:"network"`
}

type vpcSubnetModel interface {
	getID() types.String
}

func (m *vpcSubnetModelV0) getID() types.String {
	return m.ID
}

func (m *vpcSubnetModelV1) getID() types.String {
	return m.ID
}

func (r *vpcSubnetResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
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
				stateUpgraderFunc[*vpcSubnetModelV0](ctx, req, resp, r.client)
			},
		},
		1: {
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
					"project_id": schema.StringAttribute{
						Optional: true,
						Computed: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
							stringplanmodifier.RequiresReplace(),
						},
					},
					"cidr": schema.StringAttribute{
						Required:      true,
						PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
					},
					"name": schema.StringAttribute{
						Required: true,
					},
					"location": schema.StringAttribute{
						Required: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"network": schema.StringAttribute{
						Required: true,
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
							stringplanmodifier.UseStateForUnknown(),
						},
					},
				},
			},
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				stateUpgraderFunc[*vpcSubnetModelV1](ctx, req, resp, r.client)
			},
		},
	}
}

func stateUpgraderFunc[T vpcSubnetModel](ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse, client *swagger.APIClient) {
	var priorStateData T
	resp.Diagnostics.Append(req.State.Get(ctx, &priorStateData)...)

	if resp.Diagnostics.HasError() {
		return
	}

	id := priorStateData.getID()
	if id.IsNull() || id.ValueString() == "" {
		resp.Diagnostics.AddError("Failed to migrate VPC subnet to current version",
			"No ID was associated with the VPC subnet in the prior state.")

		return
	}

	vpcSubnet, projectID, err := findVpcSubnet(ctx, client, id.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to migrate VPC subnet to current version",
			fmt.Sprintf("Error fetching VPC subnet %s during migration: %v", id.ValueString(), err))

		return
	}

	var newStateData vpcSubnetResourceModel
	newStateData.ProjectID = types.StringValue(projectID)
	vpcSubnetToTerraformResourceModel(ctx, vpcSubnet, &newStateData, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newStateData)...)
	if resp.Diagnostics.HasError() {
		resp.Diagnostics.AddError("Failed to migrate VPC subnet to current version",
			"There was an error migrating the VPC subnet to the current version.")

		return
	}

	resp.Diagnostics.AddWarning("Successfully migrated VPC subnet to current version",
		"Terraform State has been successfully migrated to a new version. Please refer to"+
			" docs.crusoecloud.com for information about the updates.")
}

package instance_group

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type instanceGroupResourceModelV0 struct {
	ID                   types.String `tfsdk:"id"`
	Name                 types.String `tfsdk:"name"`
	TemplateID           types.String `tfsdk:"instance_template"`
	RunningInstanceCount types.Int64  `tfsdk:"running_instance_count"`
	InstanceNamePrefix   types.String `tfsdk:"instance_name_prefix"`
	Instances            types.List   `tfsdk:"instances"`
	ProjectID            types.String `tfsdk:"project_id"`
}

func (r *instanceGroupResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed: true,
					},
					"project_id": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"name": schema.StringAttribute{
						Required: true,
					},
					"instance_name_prefix": schema.StringAttribute{
						Required: true,
					},
					"instance_template": schema.StringAttribute{
						Required: true,
					},
					"running_instance_count": schema.Int64Attribute{
						Required: true,
					},
					"instances": schema.ListAttribute{
						ElementType: types.StringType,
						Computed:    true,
					},
				},
			},
			StateUpgrader: upgradeStateV0ToV1,
		},
	}
}

func upgradeStateV0ToV1(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	var oldState instanceGroupResourceModelV0
	resp.Diagnostics.Append(req.State.Get(ctx, &oldState)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Map v0 fields to v1 fields:
	// - instance_template → instance_template_id
	// - running_instance_count → desired_count (was user's requested count)
	// - instance_name_prefix → dropped (no longer used)
	// - instances → dropped (split into active/inactive, will be populated by Read)
	// - New computed fields will be populated on next Read
	newState := instanceGroupResourceModel{
		ID:                   oldState.ID,
		Name:                 oldState.Name,
		InstanceTemplateID:   oldState.TemplateID,
		ProjectID:            oldState.ProjectID,
		DesiredCount:         oldState.RunningInstanceCount,
		ActiveInstanceIDs:    types.ListNull(types.StringType),
		InactiveInstanceIDs:  types.ListNull(types.StringType),
		RunningInstanceCount: types.Int64Null(),
		State:                types.StringNull(),
		CreatedAt:            types.StringNull(),
		UpdatedAt:            types.StringNull(),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

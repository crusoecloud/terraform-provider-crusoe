package repository

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// repositoryModelV2 is the v2 state, which stored the upstream credentials in plain state.
type repositoryModelV2 struct {
	ProjectID        types.String             `tfsdk:"project_id"`
	Location         types.String             `tfsdk:"location"`
	Name             types.String             `tfsdk:"name"`
	Mode             types.String             `tfsdk:"mode"`
	UpstreamRegistry *upstreamRegistryModelV2 `tfsdk:"upstream_registry"`
}

type upstreamRegistryModelV2 struct {
	Provider                   types.String                   `tfsdk:"provider"`
	Url                        types.String                   `tfsdk:"url"`
	UpstreamRegistryCrdentials *upstreamRegistryCredentialsV2 `tfsdk:"upstream_registry_credentials"`
}

type upstreamRegistryCredentialsV2 struct {
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

func repositorySchemaV2() *schema.Schema {
	return &schema.Schema{
		Version: 2,
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{Computed: true, Optional: true},
			"location":   schema.StringAttribute{Required: true},
			"name":       schema.StringAttribute{Required: true},
			"mode":       schema.StringAttribute{Required: true},
			"upstream_registry": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"provider": schema.StringAttribute{Required: true},
					"url":      schema.StringAttribute{Required: true},
					"upstream_registry_credentials": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"username": schema.StringAttribute{Required: true},
							"password": schema.StringAttribute{Required: true, Sensitive: true},
						},
					},
				},
			},
		},
	}
}

func (r *repositoryResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		2: {
			PriorSchema:   repositorySchemaV2(),
			StateUpgrader: upgradeRepositoryStateV2ToV3,
		},
	}
}

// Write-only values must be null in state, so the v2 username and password are dropped.
func upgradeRepositoryStateV2ToV3(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	var prior repositoryModelV2
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	upgraded := repositoryResourceModel{
		ProjectID: prior.ProjectID,
		Location:  prior.Location,
		Name:      prior.Name,
		Mode:      prior.Mode,
	}

	if prior.UpstreamRegistry != nil {
		upgraded.UpstreamRegistry = &upstreamRegistryResourceModel{
			Provider: prior.UpstreamRegistry.Provider,
			Url:      prior.UpstreamRegistry.Url,
		}

		if prior.UpstreamRegistry.UpstreamRegistryCrdentials != nil {
			upgraded.UpstreamRegistry.UpstreamRegistryCrdentials = &upstreamRegistryCredentialsResourceModel{
				Username:           types.StringNull(),
				Password:           types.StringNull(),
				CredentialsVersion: types.Int64Null(),
			}
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, upgraded)...)
}

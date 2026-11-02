package kubernetes_addon

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource = &kubernetesAddonsDataSource{}
)

type kubernetesAddonsDataSource struct {
	client *common.CrusoeClient
}

func NewKubernetesAddonsDataSource() datasource.DataSource {
	return &kubernetesAddonsDataSource{}
}

func (ds *kubernetesAddonsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_addons"
}

func (ds *kubernetesAddonsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*common.CrusoeClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", common.ErrorMsgProviderInitFailed)

		return
	}

	ds.client = client
}

func (ds *kubernetesAddonsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: providerDescAddonsDataSource,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: providerDescAddonsID,
			},
			"project_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: providerDescProjectID,
			},
			"cluster_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescClusterID,
			},
			"addons": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: providerDescAddons,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"addon_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: providerDescAddonType,
						},
						"version": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: providerDescVersion,
						},
						"installed_version": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: providerDescInstalledVersion,
						},
						"status": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: providerDescStatus,
						},
						"ready": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: providerDescReady,
						},
						"category": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: providerDescCategory,
						},
					},
				},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *kubernetesAddonsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config kubernetesAddonsDataSourceModel
	if err := common.GetResourceModel(ctx, req.Config, &config, &resp.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(ds.client, config.ProjectID.ValueString())
	clusterID := config.ClusterID.ValueString()

	listResp, httpResp, err := ds.client.APIClient.KubernetesAddonsApi.ListAddons(ctx, projectID, clusterID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to list Kubernetes add-ons",
			fmt.Sprintf("Failed to list add-ons: %s.", common.UnpackAPIError(err)))

		return
	}

	addons := make([]installedAddonModel, 0, len(listResp.Addons))
	for i := range listResp.Addons {
		addons = append(addons, installedAddonToListModel(&listResp.Addons[i]))
	}

	common.SortByKeys(addons, func(a installedAddonModel) string { return a.AddonType.ValueString() })

	state := kubernetesAddonsDataSourceModel{
		ID:        types.StringValue(clusterID),
		ProjectID: types.StringValue(projectID),
		ClusterID: types.StringValue(clusterID),
		Addons:    addons,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

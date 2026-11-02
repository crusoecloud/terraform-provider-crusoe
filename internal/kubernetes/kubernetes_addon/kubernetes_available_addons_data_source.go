package kubernetes_addon

import (
	"context"
	"fmt"
	"net/http"

	"github.com/antihax/optional"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

const (
	availableAddonsPageSize = 500
	maxAvailableAddonsPages = 100
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource                     = &kubernetesAvailableAddonsDataSource{}
	_ datasource.DataSourceWithConfigValidators = &kubernetesAvailableAddonsDataSource{}
)

type kubernetesAvailableAddonsDataSource struct {
	client *common.CrusoeClient
}

func NewKubernetesAvailableAddonsDataSource() datasource.DataSource {
	return &kubernetesAvailableAddonsDataSource{}
}

func (ds *kubernetesAvailableAddonsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_available_addons"
}

func (ds *kubernetesAvailableAddonsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (ds *kubernetesAvailableAddonsDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("kubernetes_version"),
			path.MatchRoot("cluster_id"),
		),
	}
}

func (ds *kubernetesAvailableAddonsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: providerDescAvailableAddonsDataSource,
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: providerDescProjectID,
			},
			"kubernetes_version": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: providerDescAvailableKubernetesVersion,
			},
			"cluster_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: providerDescAvailableClusterID,
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
						"description": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: apiDescDescription,
						},
						"category": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: providerDescCategory,
						},
						"removable": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: apiDescRemovable,
						},
						"versions": schema.ListAttribute{
							ElementType:         types.StringType,
							Computed:            true,
							MarkdownDescription: apiDescVersions,
						},
					},
				},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *kubernetesAvailableAddonsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config kubernetesAvailableAddonsDataSourceModel
	if err := common.GetResourceModel(ctx, req.Config, &config, &resp.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(ds.client, config.ProjectID.ValueString())

	apiAddons, err := ds.listAllAvailableAddons(ctx, projectID,
		config.ClusterID.ValueString(), config.KubernetesVersion.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to list available Kubernetes add-ons",
			fmt.Sprintf("Failed to list available add-ons: %s.", common.UnpackAPIError(err)))

		return
	}

	addons := make([]availableAddonModel, 0, len(apiAddons))
	for i := range apiAddons {
		addon, convErr := availableAddonToModel(&apiAddons[i])
		if convErr != nil {
			resp.Diagnostics.AddError("Failed to list available Kubernetes add-ons", convErr.Error())

			return
		}
		addons = append(addons, addon)
	}

	common.SortByKeys(addons, func(a availableAddonModel) string { return a.AddonType.ValueString() })

	config.ProjectID = types.StringValue(projectID)
	config.Addons = addons

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// listAllAvailableAddons pages through the catalog, by cluster when clusterID is
// set and by kubernetesVersion otherwise.
func (ds *kubernetesAvailableAddonsDataSource) listAllAvailableAddons(
	ctx context.Context, projectID, clusterID, kubernetesVersion string,
) ([]swagger.KubernetesAvailableAddon, error) {
	var addons []swagger.KubernetesAvailableAddon
	pageToken := ""

	for page := 0; ; page++ {
		if page >= maxAvailableAddonsPages {
			return nil, fmt.Errorf("too many pages (more than %d) while listing available add-ons", maxAvailableAddonsPages)
		}

		var listResp swagger.KubernetesListAvailableAddonsResponse
		var err error

		if clusterID != "" {
			opts := &swagger.KubernetesAddonsApiListClusterAvailableAddonsOpts{
				PageSize: optional.NewInt64(availableAddonsPageSize),
			}
			if pageToken != "" {
				opts.PageToken = optional.NewString(pageToken)
			}
			var httpResp *http.Response
			listResp, httpResp, err = ds.client.APIClient.KubernetesAddonsApi.ListClusterAvailableAddons(
				ctx, projectID, clusterID, opts)
			if httpResp != nil {
				httpResp.Body.Close()
			}
		} else {
			opts := &swagger.KubernetesAddonsApiListAvailableAddonsOpts{
				PageSize: optional.NewInt64(availableAddonsPageSize),
			}
			if pageToken != "" {
				opts.PageToken = optional.NewString(pageToken)
			}
			var httpResp *http.Response
			listResp, httpResp, err = ds.client.APIClient.KubernetesAddonsApi.ListAvailableAddons(
				ctx, projectID, kubernetesVersion, opts)
			if httpResp != nil {
				httpResp.Body.Close()
			}
		}
		if err != nil {
			return nil, err
		}

		addons = append(addons, listResp.Addons...)

		if listResp.NextPageToken == "" {
			break
		}
		if listResp.NextPageToken == pageToken {
			return nil, fmt.Errorf("page token did not advance while listing available add-ons")
		}
		pageToken = listResp.NextPageToken
	}

	return addons, nil
}

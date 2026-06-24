package token

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

const keyUsageRegistry = "registry"

type tokensDataSource struct {
	client *common.CrusoeClient
}

type tokensDataSourceModel struct {
	ProjectID types.String           `tfsdk:"project_id"`
	Tokens    []tokenDataSourceModel `tfsdk:"tokens"`
}

type tokenDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Alias     types.String `tfsdk:"alias"`
	ExpiresAt types.String `tfsdk:"expires_at"`
	CreatedAt types.String `tfsdk:"created_at"`
	LastUsed  types.String `tfsdk:"last_used"`
}

func NewRegistryTokensDataSource() datasource.DataSource {
	return &tokensDataSource{}
}

func (ds *tokensDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*common.CrusoeClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected ProviderData type",
			fmt.Sprintf("Expected *swagger.APIClient, got: %T", req.ProviderData),
		)

		return
	}
	ds.client = client
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *tokensDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_registry_tokens"
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *tokensDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{
		Description: "Fetches a list of container registry tokens.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Optional:           true,
				DeprecationMessage: common.FormatDeprecation("v0.6.0") + " This field has no effect; registry tokens are org-scoped, not project-scoped.",
			},
			"tokens": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of container registry tokens.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:    true,
							Description: "Unique identifier for the token.",
						},
						"alias": schema.StringAttribute{
							Computed: true,
						},
						"expires_at": schema.StringAttribute{
							Computed: true,
						},
						"created_at": schema.StringAttribute{
							Computed: true,
						},
						"last_used": schema.StringAttribute{
							Computed: true,
						},
					},
				},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *tokensDataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	var state tokensDataSourceModel
	diags := request.Config.Get(ctx, &state)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	tokens, httpResp, err := ds.client.APIClient.LimitedUsageAPIKeyApi.GetLimitedUsageAPIKeys(ctx)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		response.Diagnostics.AddError("Failed to list tokens", fmt.Sprintf("Error listing tokens: %s", common.UnpackAPIError(err)))

		return
	}

	for _, token := range tokens.Items {
		if token.Usage != keyUsageRegistry {
			continue
		}

		var expiresAt, createdAt, lastUsed types.String

		if token.ExpiresAt != "" {
			expiresAt = types.StringValue(token.ExpiresAt)
		} else {
			expiresAt = types.StringNull()
		}

		if token.CreatedAt != "" {
			createdAt = types.StringValue(token.CreatedAt)
		} else {
			createdAt = types.StringNull()
		}

		if token.LastUsed != "" {
			lastUsed = types.StringValue(token.LastUsed)
		} else {
			lastUsed = types.StringNull()
		}

		state.Tokens = append(state.Tokens, tokenDataSourceModel{
			ID:        types.StringValue(token.KeyId),
			Alias:     types.StringValue(token.Alias),
			ExpiresAt: expiresAt,
			CreatedAt: createdAt,
			LastUsed:  lastUsed,
		})
	}

	diags = response.State.Set(ctx, &state)
	response.Diagnostics.Append(diags...)
}

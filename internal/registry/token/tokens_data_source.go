package token

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

const keyUsageRegistry = "registry"

type tokensDataSource struct {
	client *swagger.APIClient
}

type tokensDataSourceModel struct {
	Tokens    []tokenDataSourceModel `tfsdk:"tokens"`
	ProjectID types.String           `tfsdk:"project_id"`
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

func (t *tokensDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*swagger.APIClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected ProviderData type",
			fmt.Sprintf("Expected *swagger.APIClient, got: %T", req.ProviderData),
		)

		return
	}
	t.client = client
}

//nolint:gocritic // Implements Terraform defined interface
func (t *tokensDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_registry_tokens"
}

//nolint:gocritic // Implements Terraform defined interface
func (t *tokensDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{
		Description: "Fetches a list of container registry tokens.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Optional: true,
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
func (t *tokensDataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	var state tokensDataSourceModel
	diags := request.Config.Get(ctx, &state)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	projectID, err := common.GetProjectIDOrFallback(ctx, t.client, &response.Diagnostics, state.ProjectID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Failed to fetch project ID", fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	tokens, httpResp, err := t.client.LimitedUsageAPIKeyApi.GetLimitedUsageAPIKeys(ctx)
	if err != nil {
		response.Diagnostics.AddError("Failed to list tokens", fmt.Sprintf("Error listing tokens: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

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

	state.ProjectID = types.StringValue(projectID)
	diags = response.State.Set(ctx, &state)
	response.Diagnostics.Append(diags...)
}

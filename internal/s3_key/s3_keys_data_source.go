package s3_key

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type s3KeysDataSource struct {
	client *common.CrusoeClient
}

type s3KeysDataSourceModel struct {
	OrganizationID types.String `tfsdk:"organization_id"`
	Keys           []s3KeyModel `tfsdk:"keys"`
}

type s3KeyModel struct {
	KeyID       string `tfsdk:"key_id"`
	AccessKeyID string `tfsdk:"access_key_id"`
	Alias       string `tfsdk:"alias"`
	Status      string `tfsdk:"status"`
	CreatedAt   string `tfsdk:"created_at"`
	ExpireAt    string `tfsdk:"expire_at"`
	UserID      string `tfsdk:"user_id"`
}

func NewS3KeysDataSource() datasource.DataSource {
	return &s3KeysDataSource{}
}

// Configure adds the provider configured client to the data source.
func (ds *s3KeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

//nolint:gocritic // Implements Terraform defined interface
func (ds *s3KeysDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_storage_s3_keys"
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *s3KeysDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{
		MarkdownDescription: common.DevelopmentMessage + "\n\nFetches the list of S3-compatible storage access keys for an organization.",
		Attributes: map[string]schema.Attribute{
			"organization_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: descOrganizationID + " If not specified, inferred from the authenticated user.",
			},
			"keys": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of S3 access keys.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descKeyID,
						},
						"access_key_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descAccessKeyID,
						},
						"alias": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descAlias,
						},
						"status": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descStatus,
						},
						"created_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descCreatedAt,
						},
						"expire_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descExpireAt,
						},
						"user_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descUserID,
						},
					},
				},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *s3KeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	resp.Diagnostics.AddWarning("Development Feature", common.DevelopmentMessage)

	var config s3KeysDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get organization ID - use provided value or fetch from user's orgs
	orgID := config.OrganizationID.ValueString()
	if orgID == "" {
		var err error
		orgID, err = getUserOrg(ctx, ds.client.APIClient)
		if err != nil {
			resp.Diagnostics.AddError("Failed to determine organization",
				fmt.Sprintf("Could not determine organization: %s", err))

			return
		}
	}

	dataResp, httpResp, err := ds.client.APIClient.S3KeysApi.ListS3Keys(ctx, orgID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch S3 keys",
			fmt.Sprintf("Could not fetch S3 keys: %s", common.UnpackAPIError(err)))

		return
	}

	state := s3KeysDataSourceModel{
		OrganizationID: types.StringValue(orgID),
	}
	for i := range dataResp.Items {
		state.Keys = append(state.Keys, s3KeyAPIToDataSourceModel(&dataResp.Items[i]))
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

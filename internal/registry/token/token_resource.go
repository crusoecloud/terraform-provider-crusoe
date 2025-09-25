package token

import (
	"context"
	"fmt"

	"github.com/antihax/optional"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource = &tokenResource{}
)

type tokenResource struct {
	client *swagger.APIClient
}

type tokenResourceModel struct {
	ID        types.String `tfsdk:"id"`
	Alias     types.String `tfsdk:"alias"`
	ExpiresAt types.String `tfsdk:"expires_at"`
	Token     types.String `tfsdk:"token"`
}

func NewRegistryTokenResource() resource.Resource {
	return &tokenResource{}
}

func (t *tokenResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*swagger.APIClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *swagger.APIClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}
	t.client = client
}

//nolint:gocritic // Implements Terraform defined interface
func (t *tokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_registry_token"
}

//nolint:gocritic // Implements Terraform defined interface
func (t *tokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a container registry token for authentication.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(), // maintain across updates
				},
			},
			"alias": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(), // cannot be updated in place
				},
			},
			"expires_at": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(), // maintain across updates
				},
			},
			"token": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(), // maintain across updates
				},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (t *tokenResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan tokenResourceModel
	diags := request.Plan.Get(ctx, &plan)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	alias := plan.Alias
	expiresAt := plan.ExpiresAt
	body := swagger.CcrTokenRequest{}
	if !alias.IsNull() {
		body.Alias = alias.ValueString()
	}
	if !expiresAt.IsNull() {
		body.ExpiresAt = expiresAt.ValueString()
	}

	tokenReq := &swagger.CcrApiCreateCcrTokenOpts{
		Body: optional.NewInterface(body),
	}

	token, httpResp, err := t.client.CcrApi.CreateCcrToken(ctx, tokenReq)
	if err != nil {
		response.Diagnostics.AddError("Failed to create token",
			fmt.Sprintf("Error creating token: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	// After creating the token, fetch the token ID from the list API
	tokens, listResp, err := t.client.LimitedUsageAPIKeyApi.GetLimitedUsageAPIKeys(ctx)
	if err != nil {
		response.Diagnostics.AddError("Failed to fetch token ID after creation",
			fmt.Sprintf("Error fetching token list: %s", common.UnpackAPIError(err)))

		return
	}
	defer listResp.Body.Close()

	var tokenID string
	for _, listToken := range tokens.Items {
		if listToken.Usage == keyUsageRegistry && listToken.Alias == plan.Alias.ValueString() {
			tokenID = listToken.KeyId

			break
		}
	}

	if tokenID == "" {
		response.Diagnostics.AddError("Failed to find created token",
			"Could not find the newly created token in the token list")

		return
	}

	var state tokenResourceModel
	// Update the state with the created token
	state.ID = types.StringValue(tokenID)
	state.Alias = types.StringValue(plan.Alias.ValueString())
	state.ExpiresAt = types.StringValue(plan.ExpiresAt.ValueString())
	state.Token = types.StringValue(token.Token)

	diags = response.State.Set(ctx, state)
	response.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (t *tokenResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state tokenResourceModel
	diags := request.State.Get(ctx, &state)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	// For tokens, we generally can't read back the token value for security reasons
	// We'll just keep the existing state
	diags = response.State.Set(ctx, &state)
	response.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (t *tokenResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	response.Diagnostics.AddError(
		"Update Not Supported",
		"Token updates are not supported. Please delete and recreate the token if changes are needed.",
	)
}

//nolint:gocritic // Implements Terraform defined interface
func (t *tokenResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state tokenResourceModel
	diags := request.State.Get(ctx, &state)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	httpResp, err := t.client.LimitedUsageAPIKeyApi.DeleteLimitedUsageAPIKey(ctx, state.ID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Failed to delete token",
			fmt.Sprintf("Error deleting token: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()
}

func (t *tokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

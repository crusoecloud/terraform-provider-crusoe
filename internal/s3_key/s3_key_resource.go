package s3_key

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type s3KeyResource struct {
	client *common.CrusoeClient
}

type s3KeyResourceModel struct {
	KeyID           types.String `tfsdk:"key_id"`
	AccessKeyID     types.String `tfsdk:"access_key_id"`
	SecretAccessKey types.String `tfsdk:"secret_access_key"`
	Alias           types.String `tfsdk:"alias"`
	OrganizationID  types.String `tfsdk:"organization_id"`
	ExpireAt        types.String `tfsdk:"expire_at"`
	Status          types.String `tfsdk:"status"`
	CreatedAt       types.String `tfsdk:"created_at"`
	UserID          types.String `tfsdk:"user_id"`
}

func NewS3KeyResource() resource.Resource {
	return &s3KeyResource{}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *s3KeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*common.CrusoeClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", common.ErrorMsgProviderInitFailed)

		return
	}

	r.client = client
}

//nolint:gocritic // Implements Terraform defined interface
func (r *s3KeyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_storage_s3_key"
}

//nolint:gocritic // Implements Terraform defined interface
func (r *s3KeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: common.DevelopmentMessage + "\n\nManages an S3-compatible storage access key.\n\n" +
			"**Important:** The `secret_access_key` is only returned once on creation. " +
			"Terraform will store it in state, but it cannot be retrieved from the API after creation.",
		Attributes: map[string]schema.Attribute{
			"key_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: descKeyID,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"access_key_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: descAccessKeyID,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"secret_access_key": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: descSecretAccessKey,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"alias": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: descAlias,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"organization_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: descOrganizationID + " If not specified, inferred from the authenticated user.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"expire_at": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: descExpireAt + " Must be in RFC3339 format (e.g., `2025-12-31T23:59:59Z`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: descStatus,
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: descCreatedAt,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: descUserID,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *s3KeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: "organization_id,access_key_id"
	// Note: secret_access_key cannot be imported as it's not retrievable from the API
	resp.Diagnostics.AddWarning("Secret Access Key Not Imported",
		"The secret_access_key cannot be retrieved from the API and will be empty after import. "+
			"If you need the secret key, you must delete and recreate the resource.")

	resource.ImportStatePassthroughID(ctx, path.Root("access_key_id"), req, resp)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *s3KeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan s3KeyResourceModel
	if err := getResourceModel(ctx, req.Plan, &plan, &resp.Diagnostics); err != nil {
		return
	}

	// Get organization ID - use provided value or fetch from user's orgs
	orgID := plan.OrganizationID.ValueString()
	if orgID == "" {
		var err error
		orgID, err = getUserOrg(ctx, r.client.APIClient)
		if err != nil {
			resp.Diagnostics.AddError("Failed to determine organization",
				fmt.Sprintf("Could not determine organization: %s", err))

			return
		}
	}

	// Build create request
	createReq := swagger.CreateS3KeyRequest{}
	if !plan.Alias.IsNull() && !plan.Alias.IsUnknown() {
		createReq.Alias = plan.Alias.ValueString()
	}
	if !plan.ExpireAt.IsNull() && !plan.ExpireAt.IsUnknown() {
		createReq.ExpireAt = plan.ExpireAt.ValueString()
	}

	dataResp, httpResp, err := r.client.APIClient.S3KeysApi.CreateS3Key(ctx, createReq, orgID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to create S3 key",
			fmt.Sprintf("There was an error creating the S3 key: %s", common.UnpackAPIError(err)))

		return
	}

	// Store the one-time secret key immediately
	plan.AccessKeyID = types.StringValue(dataResp.AccessKeyId)
	plan.SecretAccessKey = types.StringValue(dataResp.SecretKey)
	plan.OrganizationID = types.StringValue(orgID)
	if isValidExpireAt(dataResp.ExpireAt) {
		plan.ExpireAt = types.StringValue(dataResp.ExpireAt)
	}

	// Fetch the created key to get additional computed fields
	keys, getResp, err := r.client.APIClient.S3KeysApi.ListS3Keys(ctx, orgID)
	if getResp != nil {
		defer getResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to read S3 key after creation",
			fmt.Sprintf("The key was created but could not be read: %s", common.UnpackAPIError(err)))

		return
	}

	// Find the created key by access_key_id
	for i := range keys.Items {
		if keys.Items[i].AccessKeyId == dataResp.AccessKeyId {
			plan.KeyID = types.StringValue(keys.Items[i].KeyUuid)
			plan.Status = types.StringValue(keys.Items[i].Status)
			plan.CreatedAt = types.StringValue(keys.Items[i].CreatedAt)
			plan.UserID = types.StringValue(keys.Items[i].UserId)
			if keys.Items[i].Alias != "" {
				plan.Alias = types.StringValue(keys.Items[i].Alias)
			}
			if isValidExpireAt(keys.Items[i].ExpireAt) {
				plan.ExpireAt = types.StringValue(keys.Items[i].ExpireAt)
			}

			break
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *s3KeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state s3KeyResourceModel
	if err := getResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	orgID := state.OrganizationID.ValueString()
	if orgID == "" {
		var err error
		orgID, err = getUserOrg(ctx, r.client.APIClient)
		if err != nil {
			resp.Diagnostics.AddError("Failed to determine organization",
				fmt.Sprintf("Could not determine organization: %s", err))

			return
		}
	}

	keys, httpResp, err := r.client.APIClient.S3KeysApi.ListS3Keys(ctx, orgID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to read S3 key",
			fmt.Sprintf("There was an error reading the S3 key: %s", common.UnpackAPIError(err)))

		return
	}

	// Find the key by access_key_id
	accessKeyID := state.AccessKeyID.ValueString()
	var found bool
	for i := range keys.Items {
		if keys.Items[i].AccessKeyId == accessKeyID {
			found = true
			state.KeyID = types.StringValue(keys.Items[i].KeyUuid)
			state.Status = types.StringValue(keys.Items[i].Status)
			state.CreatedAt = types.StringValue(keys.Items[i].CreatedAt)
			state.UserID = types.StringValue(keys.Items[i].UserId)
			state.OrganizationID = types.StringValue(orgID)
			if keys.Items[i].Alias != "" {
				state.Alias = types.StringValue(keys.Items[i].Alias)
			}
			if isValidExpireAt(keys.Items[i].ExpireAt) {
				state.ExpireAt = types.StringValue(keys.Items[i].ExpireAt)
			}
			// Note: SecretAccessKey is preserved from state - it cannot be retrieved from the API

			break
		}
	}

	if !found {
		// Key was deleted out of band
		resp.State.RemoveResource(ctx)

		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *s3KeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// S3 keys do not support updates - all mutable fields have RequiresReplace
	resp.Diagnostics.AddError("Update not supported",
		"S3 keys do not support in-place updates. Changes to alias, expire_at, or organization_id require replacing the resource.")
}

//nolint:gocritic // Implements Terraform defined interface
func (r *s3KeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state s3KeyResourceModel
	if err := getResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	orgID := state.OrganizationID.ValueString()
	if orgID == "" {
		var err error
		orgID, err = getUserOrg(ctx, r.client.APIClient)
		if err != nil {
			resp.Diagnostics.AddError("Failed to determine organization",
				fmt.Sprintf("Could not determine organization: %s", err))

			return
		}
	}

	accessKeyID := state.AccessKeyID.ValueString()

	httpResp, err := r.client.APIClient.S3KeysApi.DeleteS3Key(ctx, orgID, accessKeyID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete S3 key",
			fmt.Sprintf("There was an error deleting the S3 key: %s", common.UnpackAPIError(err)))

		return
	}
}

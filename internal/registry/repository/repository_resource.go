package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/antihax/optional"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                 = &repositoryResource{}
	_ resource.ResourceWithUpgradeState = &repositoryResource{}
)

type repositoryResource struct {
	client *common.CrusoeClient
}

type repositoryResourceModel struct {
	ProjectID        types.String                   `tfsdk:"project_id"`
	Location         types.String                   `tfsdk:"location"`
	Name             types.String                   `tfsdk:"name"`
	Mode             types.String                   `tfsdk:"mode"`
	UpstreamRegistry *upstreamRegistryResourceModel `tfsdk:"upstream_registry"`
}

type upstreamRegistryResourceModel struct {
	Provider                   types.String                              `tfsdk:"provider"`
	Url                        types.String                              `tfsdk:"url"`
	UpstreamRegistryCrdentials *upstreamRegistryCredentialsResourceModel `tfsdk:"upstream_registry_credentials"`
}

type upstreamRegistryCredentialsResourceModel struct {
	Username           types.String `tfsdk:"username"`
	Password           types.String `tfsdk:"password"`
	CredentialsVersion types.Int64  `tfsdk:"credentials_version"`
}

func NewRegistryRepositoryResource() resource.Resource {
	return &repositoryResource{}
}

func (r *repositoryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *repositoryResource) Metadata(ctx context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_registry_repository"
}

//nolint:gocritic // Implements Terraform defined interface
func (r *repositoryResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// DO NOT lower or remove, or Terraform rejects existing state as "managed
		// by a newer provider version" and breaks upgrades. This resource was
		// born at Version 2; Version 3 made the upstream credentials write-only (see UpgradeState).
		Version:             3,
		MarkdownDescription: providerDescResource,
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: providerDescProjectID,
				// UseStateForUnknown must run first so an unset (computed) project_id doesn't plan as a change and force a replace.
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"location": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescLocation,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescName,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"mode": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescMode,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
				Validators: []validator.String{
					stringvalidator.OneOf("pull-through-cache", "standard"),
				},
			},
			"upstream_registry": schema.SingleNestedAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.Object{requiresReplaceIfUpstreamAddedOrRemoved()},
				Attributes: map[string]schema.Attribute{
					"provider": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: apiDescUpstreamProvider,
						PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
					},
					"url": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: apiDescUpstreamURL,
						PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
					},
					"upstream_registry_credentials": schema.SingleNestedAttribute{
						Optional:      true,
						PlanModifiers: []planmodifier.Object{requiresReplaceIfCredentialsRemoved()},
						Attributes: map[string]schema.Attribute{
							"username": schema.StringAttribute{
								Required:            true,
								WriteOnly:           true,
								MarkdownDescription: providerDescUpstreamCredsUsername,
							},
							"password": schema.StringAttribute{
								Required:            true,
								Sensitive:           true,
								WriteOnly:           true,
								MarkdownDescription: providerDescUpstreamCredsPassword,
							},
							"credentials_version": schema.Int64Attribute{
								Optional:            true,
								MarkdownDescription: providerDescUpstreamCredsVersion,
							},
						},
					},
				},
			},
		},
	}
}

// Child RequiresReplace modifiers don't run when the whole block goes to or from null, so the block replaces itself.
func requiresReplaceIfUpstreamAddedOrRemoved() planmodifier.Object {
	return objectplanmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.ObjectRequest, resp *objectplanmodifier.RequiresReplaceIfFuncResponse) {
			resp.RequiresReplace = req.StateValue.IsNull() != req.PlanValue.IsNull()
		},
		"Adding or removing the upstream registry requires replacing the repository.",
		"Adding or removing the upstream registry requires replacing the repository.",
	)
}

// The credentials API can only rotate, and a private upstream can't run anonymously, so removal replaces.
func requiresReplaceIfCredentialsRemoved() planmodifier.Object {
	return objectplanmodifier.RequiresReplaceIf(
		func(_ context.Context, req planmodifier.ObjectRequest, resp *objectplanmodifier.RequiresReplaceIfFuncResponse) {
			resp.RequiresReplace = !req.StateValue.IsNull() && req.PlanValue.IsNull()
		},
		"Removing upstream registry credentials requires replacing the repository.",
		"Removing upstream registry credentials requires replacing the repository.",
	)
}

// upstreamCredentials takes a config-sourced upstream_registry, the only place write-only values are non-null.
func upstreamCredentials(registry *upstreamRegistryResourceModel) *swagger.UpstreamRegistryCredentials {
	if registry == nil || registry.UpstreamRegistryCrdentials == nil {
		return nil
	}

	creds := registry.UpstreamRegistryCrdentials
	if creds.Username.ValueString() == "" && creds.Password.ValueString() == "" {
		return nil
	}

	return &swagger.UpstreamRegistryCredentials{
		Username: creds.Username.ValueString(),
		Password: creds.Password.ValueString(),
	}
}

// Write-only credentials are never in plan or state, so credentials_version is the only change signal.
func credentialsChanged(state, plan *upstreamRegistryResourceModel) bool {
	if plan == nil || plan.UpstreamRegistryCrdentials == nil {
		return false
	}
	if state == nil || state.UpstreamRegistryCrdentials == nil {
		return true
	}

	return !plan.UpstreamRegistryCrdentials.CredentialsVersion.Equal(state.UpstreamRegistryCrdentials.CredentialsVersion)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *repositoryResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan repositoryResourceModel
	if err := common.GetResourceModel(ctx, request.Plan, &plan, &response.Diagnostics); err != nil {
		return
	}

	// Write-only credentials are null in the plan; only the config carries them.
	var config repositoryResourceModel
	if err := common.GetResourceModel(ctx, request.Config, &config, &response.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())

	var upstreamRegistry *swagger.UpstreamRegistry
	repositoryMode := plan.Mode.ValueString()
	if repositoryMode == "pull-through-cache" {
		if plan.UpstreamRegistry == nil {
			response.Diagnostics.AddError("Missing upstream_registry block", "The 'upstream_registry' block is required when mode is 'pull-through-cache'. Please provide it in your configuration.")

			return
		}
		upstreamRegistry = &swagger.UpstreamRegistry{
			Provider:                    plan.UpstreamRegistry.Provider.ValueString(),
			Url:                         plan.UpstreamRegistry.Url.ValueString(),
			UpstreamRegistryCredentials: upstreamCredentials(config.UpstreamRegistry),
		}
	}

	createRequest := swagger.RepositoryRequest{
		Location:         plan.Location.ValueString(),
		Name:             plan.Name.ValueString(),
		Mode:             repositoryMode,
		UpstreamRegistry: upstreamRegistry,
	}

	opts := &swagger.CcrApiCreateCcrRepositoryOpts{
		Body: optional.NewInterface(createRequest),
	}
	repository, httpResp, err := r.client.APIClient.CcrApi.CreateCcrRepository(ctx, projectID, opts)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		response.Diagnostics.AddError("Failed to create repository",
			fmt.Sprintf("Error creating the repository: %s", common.UnpackAPIError(err)))

		return
	}

	// Seed credentials from the plan; repositoryToResourceModel preserves them.
	state := repositoryResourceModel{UpstreamRegistry: plan.UpstreamRegistry}
	repositoryToResourceModel(&repository, &state, projectID)

	diags := response.State.Set(ctx, &state)
	response.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *repositoryResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var stored repositoryResourceModel
	if err := common.GetResourceModel(ctx, request.State, &stored, &response.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, stored.ProjectID.ValueString())

	opts := &swagger.CcrApiGetCcrRepositoryOpts{
		Location: optional.NewString(stored.Location.ValueString()),
	}
	repository, httpResp, err := r.client.APIClient.CcrApi.GetCcrRepository(ctx, projectID, stored.Name.ValueString(), opts)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			response.State.RemoveResource(ctx)

			return
		}

		response.Diagnostics.AddError("Failed to read repository",
			fmt.Sprintf("Error reading the repository: %s", common.UnpackAPIError(err)))

		return
	}
	// stored carries prior-state credentials, which repositoryToResourceModel preserves.
	repositoryToResourceModel(&repository, &stored, projectID)

	diags := response.State.Set(ctx, &stored)
	response.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *repositoryResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan repositoryResourceModel
	if err := common.GetResourceModel(ctx, request.Plan, &plan, &response.Diagnostics); err != nil {
		return
	}

	var state repositoryResourceModel
	if err := common.GetResourceModel(ctx, request.State, &state, &response.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())

	// Rotating upstream credentials is the only in-place update; every other change, including removing credentials, replaces.
	if credentialsChanged(state.UpstreamRegistry, plan.UpstreamRegistry) {
		// Write-only credentials are null in plan and state; only the config carries them.
		var config repositoryResourceModel
		if err := common.GetResourceModel(ctx, request.Config, &config, &response.Diagnostics); err != nil {
			return
		}

		creds := upstreamCredentials(config.UpstreamRegistry)
		if creds == nil {
			response.Diagnostics.AddError("Failed to update repository credentials",
				"upstream_registry_credentials is missing from the configuration.")

			return
		}

		opts := &swagger.CcrApiUpdateCcrRepositoryCredentialsOpts{
			Body: optional.NewInterface(*creds),
		}
		httpResp, err := r.client.APIClient.CcrApi.UpdateCcrRepositoryCredentials(ctx, projectID, state.Name.ValueString(), opts)
		if httpResp != nil {
			defer httpResp.Body.Close()
		}
		if err != nil {
			response.Diagnostics.AddError("Failed to update repository credentials",
				fmt.Sprintf("Error updating upstream registry credentials: %s", common.UnpackAPIError(err)))

			return
		}
	}

	// Re-read the repository to pick up any server-side changes.
	getOpts := &swagger.CcrApiGetCcrRepositoryOpts{
		Location: optional.NewString(plan.Location.ValueString()),
	}
	repository, httpResp, err := r.client.APIClient.CcrApi.GetCcrRepository(ctx, projectID, state.Name.ValueString(), getOpts)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		response.Diagnostics.AddError("Failed to read repository after update",
			fmt.Sprintf("Error reading the repository: %s", common.UnpackAPIError(err)))

		return
	}

	// Seed credentials from the plan; repositoryToResourceModel preserves them.
	newState := repositoryResourceModel{UpstreamRegistry: plan.UpstreamRegistry}
	repositoryToResourceModel(&repository, &newState, projectID)

	diags := response.State.Set(ctx, &newState)
	response.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *repositoryResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var stored repositoryResourceModel
	if err := common.GetResourceModel(ctx, request.State, &stored, &response.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, stored.ProjectID.ValueString())

	opts := &swagger.CcrApiDeleteCcrRepositoryOpts{
		Location: optional.NewString(stored.Location.ValueString()),
	}
	httpResp, err := r.client.APIClient.CcrApi.DeleteCcrRepository(ctx, projectID, stored.Name.ValueString(), opts)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		response.Diagnostics.AddError("Failed to delete repository",
			fmt.Sprintf("Error deleting repository: %s", common.UnpackAPIError(err)))

		return
	}
}

func (r *repositoryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Expect import ID as <location>/<name>
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError(
			"Invalid import ID format",
			"Expected import ID in the format <location>/<name> (e.g., us-southcentral1-a/standard-bug-bash)",
		)

		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("location"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[1])...)
}

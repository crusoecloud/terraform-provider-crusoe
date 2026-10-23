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
	_ resource.Resource = &repositoryResource{}
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
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
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
		// born at Version 2 (its very first release); no state upgrader exists
		// or is needed.
		Version: 2,
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: providerDescProjectID,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(), // cannot be updated in place
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
				Optional: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplaceIf(
						requiresReplaceIfAddedOrRemoved,
						"Adding or removing the upstream registry requires replacement.",
						"Adding or removing the upstream registry requires replacement.",
					),
				},
				Attributes: map[string]schema.Attribute{
					"provider": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: apiDescUpstreamProvider,
						PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
					},
					"url": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: apiDescUpstreamURL,
						PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
					},
					// Credentials are the only attributes that can be updated in place.
					"upstream_registry_credentials": schema.SingleNestedAttribute{
						Optional: true,
						PlanModifiers: []planmodifier.Object{
							objectplanmodifier.RequiresReplaceIf(
								requiresReplaceIfCredentialsCleared,
								"Removing or emptying the upstream registry credentials requires replacement.",
								"Removing or emptying the upstream registry credentials requires replacement.",
							),
						},
						Attributes: map[string]schema.Attribute{
							// An empty value is left out of the request body, so the API
							// never receives it. Reject it instead of recording a value the
							// repository does not have; omit the whole block for an upstream
							// registry that needs no credentials.
							"username": schema.StringAttribute{
								Required:            true,
								MarkdownDescription: apiDescUpstreamCredsUsername,
								Validators: []validator.String{
									stringvalidator.LengthAtLeast(1),
								},
							},
							"password": schema.StringAttribute{
								Required:            true,
								Sensitive:           true,
								MarkdownDescription: apiDescUpstreamCredsPassword,
								Validators: []validator.String{
									stringvalidator.LengthAtLeast(1),
								},
							},
						},
					},
				},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *repositoryResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan repositoryResourceModel
	diags := request.Plan.Get(ctx, &plan)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
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
			UpstreamRegistryCredentials: upstreamRegistryCredentialsFromModel(plan.UpstreamRegistry),
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

	diags = response.State.Set(ctx, &state)
	response.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *repositoryResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var stored repositoryResourceModel
	diags := request.State.Get(ctx, &stored)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
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

	diags = response.State.Set(ctx, &stored)
	response.Diagnostics.Append(diags...)
}

// Update changes the upstream registry credentials in place. Every other attribute
// requires replacement, so the credentials are the only thing an update can change.
//
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

	planCredentials := upstreamRegistryCredentialsFromModel(plan.UpstreamRegistry)
	stateCredentials := upstreamRegistryCredentialsFromModel(state.UpstreamRegistry)
	if planCredentials == nil && plan.UpstreamRegistry != nil && plan.UpstreamRegistry.UpstreamRegistryCrdentials != nil {
		// Schema validators reject empty values, so an apply never reaches this.
		// State cannot stand in for the check either: a freshly imported repository
		// carries no credentials in state whether or not it has them, because the
		// API never returns the password.
		response.Diagnostics.AddError(
			"Empty Upstream Registry Credentials Not Supported",
			"The upstream registry credentials of a repository cannot be set to empty values: an empty "+
				"value is left out of the request, so the repository would keep the credentials it has. "+
				"Omit the upstream_registry_credentials block for an upstream registry that needs no "+
				"credentials, or destroy and recreate the repository.",
		)

		return
	}

	if credentialsCleared(stateCredentials, planCredentials) {
		// requiresReplaceIfCredentialsCleared plans a replacement for this, so an
		// update never reaches it. Refuse rather than report a success that only
		// changed state: an empty value is left out of the request body, so the
		// repository would keep the credentials it has.
		response.Diagnostics.AddError(
			"Removing Upstream Registry Credentials Not Supported",
			"Removing a username or password from the upstream registry credentials of an existing "+
				"repository is not supported. To remove them, the repository must be destroyed and recreated.",
		)

		return
	}

	if planCredentials != nil && !upstreamRegistryCredentialsEqual(planCredentials, stateCredentials) {
		opts := &swagger.CcrApiUpdateCcrRepositoryCredentialsOpts{
			Body: optional.NewInterface(*planCredentials),
		}
		httpResp, err := r.client.APIClient.CcrApi.UpdateCcrRepositoryCredentials(ctx, projectID, state.Name.ValueString(), opts)
		if httpResp != nil {
			defer httpResp.Body.Close()
		}
		if err != nil {
			response.Diagnostics.AddError("Failed to update repository",
				fmt.Sprintf("Error updating the repository upstream registry credentials: %s", common.UnpackAPIError(err)))

			return
		}
	}

	plan.ProjectID = types.StringValue(projectID)

	diags := response.State.Set(ctx, &plan)
	response.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *repositoryResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var stored repositoryResourceModel
	diags := request.State.Get(ctx, &stored)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
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

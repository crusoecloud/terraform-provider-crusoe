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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource = &repositoryResource{}
)

type repositoryResource struct {
	client *swagger.APIClient
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

	client, ok := req.ProviderData.(*swagger.APIClient)
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
		Version: 2,
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Computed: true,
				Optional: true,
			},
			"location": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"mode": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
				Validators: []validator.String{
					stringvalidator.OneOf("pull-through-cache", "standard"),
				},
			},
			"upstream_registry": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"provider": schema.StringAttribute{
						Required: true,
					},
					"url": schema.StringAttribute{
						Required: true,
					},
					"upstream_registry_credentials": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"username": schema.StringAttribute{
								Required: true,
							},
							"password": schema.StringAttribute{
								Required:  true,
								Sensitive: true,
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

	projectID, err := common.GetProjectIDOrFallback(ctx, r.client, &response.Diagnostics, plan.ProjectID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Failed to fetch project ID",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}
	var upstreamRegistry *swagger.UpstreamRegistry
	repositoryMode := plan.Mode.ValueString()
	if repositoryMode == "pull-through-cache" {
		if plan.UpstreamRegistry == nil {
			response.Diagnostics.AddError("Missing upstream_registry block", "The 'upstream_registry' block is required when mode is 'pull-through-cache'. Please provide it in your configuration.")

			return
		}
		provider := plan.UpstreamRegistry.Provider.ValueString()
		url := plan.UpstreamRegistry.Url.ValueString()
		inputRegistryCreds := plan.UpstreamRegistry.UpstreamRegistryCrdentials
		var registryCreds *swagger.UpstreamRegistryCredentials
		if inputRegistryCreds != nil {
			username := inputRegistryCreds.Username.ValueString()
			password := inputRegistryCreds.Password.ValueString()
			if username != "" || password != "" {
				registryCreds = &swagger.UpstreamRegistryCredentials{
					Password: password,
					Username: username,
				}
			}
		}

		upstreamRegistry = &swagger.UpstreamRegistry{
			Provider:                    provider,
			Url:                         url,
			UpstreamRegistryCredentials: registryCreds,
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
	repository, httpResp, err := r.client.CcrApi.CreateCcrRepository(ctx, projectID, opts)
	if err != nil {
		response.Diagnostics.AddError("Failed to create repository",
			fmt.Sprintf("Error creating the repository: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	var state repositoryResourceModel
	state.Location = types.StringValue(repository.Location)
	state.Name = types.StringValue(repository.Name)
	state.Mode = types.StringValue(repository.Mode)
	state.ProjectID = types.StringValue(projectID)
	state.UpstreamRegistry = plan.UpstreamRegistry
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

	projectID, err := common.GetProjectIDOrFallback(ctx, r.client, &response.Diagnostics, stored.ProjectID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Failed to fetch project ID",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	repository, httpResp, err := r.client.CcrApi.GetCcrRepository(ctx, projectID, stored.Name.ValueString(), stored.Location.ValueString())
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			response.State.RemoveResource(ctx)

			return
		}

		response.Diagnostics.AddError("Failed to read repository",
			fmt.Sprintf("Error reading the repository: %s", common.UnpackAPIError(err)))

		return
	}
	var state repositoryResourceModel

	diags = response.State.Get(ctx, &state)
	response.Diagnostics.Append(diags...)

	if response.Diagnostics.HasError() {
		return
	}

	state.Location = types.StringValue(repository.Location)
	state.ProjectID = types.StringValue(projectID)
	state.Name = types.StringValue(repository.Name)
	state.Mode = types.StringValue(repository.Mode)
	state.UpstreamRegistry = nil
	// Set UpstreamRegistry from API response if present
	if repository.UpstreamRegistry != nil {
		state.UpstreamRegistry = &upstreamRegistryResourceModel{
			Provider: types.StringValue(repository.UpstreamRegistry.Provider),
			Url:      types.StringValue(repository.UpstreamRegistry.Url),
		}
		if repository.UpstreamRegistry.UpstreamRegistryCredentials != nil {
			state.UpstreamRegistry.UpstreamRegistryCrdentials = &upstreamRegistryCredentialsResourceModel{
				Username: types.StringValue(repository.UpstreamRegistry.UpstreamRegistryCredentials.Username),
				Password: types.StringValue(repository.UpstreamRegistry.UpstreamRegistryCredentials.Password),
			}
		}
	}

	diags = response.State.Set(ctx, &state)
	response.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *repositoryResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	panic("Updating repository is not currently supported")
}

//nolint:gocritic // Implements Terraform defined interface
func (r *repositoryResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var stored repositoryResourceModel
	diags := request.State.Get(ctx, &stored)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	projectID, err := common.GetProjectIDOrFallback(ctx, r.client, &response.Diagnostics, stored.ProjectID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Failed to fetch project ID",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	httpResp, err := r.client.CcrApi.DeleteCcrRepository(ctx, projectID, stored.Name.ValueString(), stored.Location.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Failed to delete repository",
			fmt.Sprintf("Error deleting repository: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()
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

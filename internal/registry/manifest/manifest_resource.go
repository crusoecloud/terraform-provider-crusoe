package manifest

import (
	"context"
	"fmt"
	"github.com/antihax/optional"
	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource = &manifestResource{}
)

type manifestResource struct {
	client *swagger.APIClient
}

type manifestResourceModel struct {
	ProjectID types.String `tfsdk:"project_id"`
	Location  types.String `tfsdk:"location"`
	RepoName  types.String `tfsdk:"repo_name"`
	ImageName types.String `tfsdk:"image_name"`
	Digest    types.String `tfsdk:"digest"`
	Tag       types.String `tfsdk:"tag"`
}

func NewRegistryManifestResource() resource.Resource {
	return &manifestResource{}
}

func (m *manifestResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	m.client = client
}

func (m *manifestResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_registry_manifest"
}

func (m *manifestResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a container registry manifest. This resource only supports deletion of manifests.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Optional: true,
			},
			"location": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"repo_name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"image_name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"digest": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tag": schema.StringAttribute{
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (m *manifestResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	response.Diagnostics.AddError(
		"Create Not Supported",
		"This resource only supports deletion of existing manifests. Manifests are created by pushing to the registry directly.",
	)
}

func (m *manifestResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state manifestResourceModel
	diags := request.State.Get(ctx, &state)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	// For a delete-only resource, we'll just keep the state as-is
	// In a real implementation, you might want to check if the manifest still exists
	// and remove from state if it doesn't, but for simplicity we'll keep it
	diags = response.State.Set(ctx, &state)
	response.Diagnostics.Append(diags...)
}

func (m *manifestResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	response.Diagnostics.AddError(
		"Update Not Supported",
		"This resource only supports deletion of manifests. Manifest updates are handled by pushing new versions to the registry.",
	)
}

func (m *manifestResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state manifestResourceModel
	diags := request.State.Get(ctx, &state)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	projectID, err := common.GetProjectIDOrFallback(ctx, m.client, &response.Diagnostics, state.ProjectID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Failed to fetch project ID",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))
		return
	}
	imageDigest := state.Digest
	imageTag := state.Tag
	manifestOpts := swagger.CcrApiDeleteCcrManifestOpts{}
	if !imageDigest.IsNull() {
		manifestOpts.Digest = optional.NewString(imageDigest.ValueString())
	}
	if !imageTag.IsNull() {
		manifestOpts.Tag = optional.NewString(imageTag.ValueString())
	}

	httpResp, err := m.client.CcrApi.DeleteCcrManifest(ctx, projectID, state.RepoName.ValueString(), state.ImageName.ValueString(), state.Location.ValueString(), &manifestOpts)
	if err != nil {
		response.Diagnostics.AddError("Failed to delete manifest",
			fmt.Sprintf("Error deleting manifest: %s", common.UnpackAPIError(err)))
		return
	}
	defer httpResp.Body.Close()
}

func (m *manifestResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("digest"), req, resp)
}

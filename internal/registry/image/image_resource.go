package image

import (
	"context"
	"fmt"
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
	_ resource.Resource = &imageResource{}
)

type imageResource struct {
	client *swagger.APIClient
}

type imageResourceModel struct {
	ProjectID types.String `tfsdk:"project_id"`
	Location  types.String `tfsdk:"location"`
	RepoName  types.String `tfsdk:"repo_name"`
	ImageName types.String `tfsdk:"image_name"`
}

func NewRegistryImageResource() resource.Resource {
	return &imageResource{}
}

func (i *imageResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	i.client = client
}

func (i *imageResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_registry_image"
}

func (i *imageResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a container registry image. This resource only supports deletion of images.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Optional:    true,
				Description: "Project ID. If not specified, the provider-level project ID will be used.",
			},
			"location": schema.StringAttribute{
				Required:    true,
				Description: "Location where the registry repository is hosted.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"repo_name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the repository containing the image.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"image_name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the image to manage.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (i *imageResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	response.Diagnostics.AddError(
		"Create Not Supported",
		"This resource only supports deletion of existing images. Images are created by pushing to the registry directly.",
	)
}

func (i *imageResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state imageResourceModel
	diags := request.State.Get(ctx, &state)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	// For a delete-only resource, we'll just keep the state as-is
	// In a real implementation, you might want to check if the image still exists
	// and remove from state if it doesn't, but for simplicity we'll keep it
	diags = response.State.Set(ctx, &state)
	response.Diagnostics.Append(diags...)
}

func (i *imageResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	response.Diagnostics.AddError(
		"Update Not Supported",
		"This resource only supports deletion of images. Image updates are handled by pushing new versions to the registry.",
	)
}

func (i *imageResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state imageResourceModel
	diags := request.State.Get(ctx, &state)
	response.Diagnostics.Append(diags...)
	if response.Diagnostics.HasError() {
		return
	}

	projectID, err := common.GetProjectIDOrFallback(ctx, i.client, &response.Diagnostics, state.ProjectID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Failed to fetch project ID",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))
		return
	}

	httpResp, err := i.client.CcrApi.DeleteCcrImage(ctx, projectID, state.RepoName.ValueString(), state.ImageName.ValueString(), state.Location.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Failed to delete image",
			fmt.Sprintf("Error deleting image: %s", common.UnpackAPIError(err)))
		return
	}
	defer httpResp.Body.Close()
}

func (i *imageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

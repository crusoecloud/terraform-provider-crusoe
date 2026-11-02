package kubernetes_addon

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &kubernetesAddonResource{}
	_ resource.ResourceWithImportState = &kubernetesAddonResource{}
)

type kubernetesAddonResource struct {
	client *common.CrusoeClient
}

func NewKubernetesAddonResource() resource.Resource {
	return &kubernetesAddonResource{}
}

func (r *kubernetesAddonResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *kubernetesAddonResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_addon"
}

func (r *kubernetesAddonResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: providerDescResource,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: providerDescID,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"project_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: providerDescProjectID,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),    // cannot be updated in place
					stringplanmodifier.UseStateForUnknown(), // maintain across updates
				},
			},
			"cluster_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescClusterID,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"addon_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: providerDescAddonType,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(), // cannot be updated in place
					common.NewDevelopmentWarningStringModifier("Limited Availability", common.LimitedAvailabilityMessage),
				},
			},
			"version": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: providerDescVersion,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(), // maintain across updates
					common.NewImmutableStringModifier(
						"Add-on Version Change Not Supported",
						"Cannot change version from %q to %q after the add-on is installed. "+
							"Remove the add-on and add it again to install a different version.",
					),
				},
			},
			"installed_version": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: providerDescInstalledVersion,
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: providerDescStatus,
			},
			"ready": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: providerDescReady,
			},
			"category": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: providerDescCategory,
			},
		},
	}
}

// findInstalledAddon returns the cluster's installed add-on matching addonType.
// httpResp is returned so callers can detect a 404.
func (r *kubernetesAddonResource) findInstalledAddon(ctx context.Context, projectID, clusterID, addonType string) (
	addon swagger.KubernetesInstalledAddon, found bool, httpResp *http.Response, err error,
) {
	listResp, httpResp, err := r.client.APIClient.KubernetesAddonsApi.ListAddons(ctx, projectID, clusterID)
	if err != nil {
		return swagger.KubernetesInstalledAddon{}, false, httpResp, err
	}

	for i := range listResp.Addons {
		if listResp.Addons[i].AddonType == addonType {
			return listResp.Addons[i], true, httpResp, nil
		}
	}

	return swagger.KubernetesInstalledAddon{}, false, httpResp, nil
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesAddonResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesAddonResourceModel
	if err := common.GetResourceModel(ctx, req.Plan, &plan, &resp.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())
	clusterID := plan.ClusterID.ValueString()
	addonType := plan.AddonType.ValueString()

	_, exists, existingHTTPResp, existingErr := r.findInstalledAddon(ctx, projectID, clusterID, addonType)
	if existingHTTPResp != nil {
		defer existingHTTPResp.Body.Close()
	}
	if existingErr != nil {
		resp.Diagnostics.AddError("Failed to create Kubernetes add-on",
			fmt.Sprintf("Failed to check for an existing add-on: %s.", common.UnpackAPIError(existingErr)))

		return
	}
	if exists {
		resp.Diagnostics.AddError("Failed to create Kubernetes add-on",
			fmt.Sprintf("The add-on %q is already installed on this cluster. "+
				"Use terraform import to manage it.", addonType))

		return
	}

	_, httpResp, err := r.client.APIClient.KubernetesAddonsApi.CreateAddons(ctx, swagger.KubernetesAddonsPostRequest{
		Addons: []swagger.KubernetesAddonSpec{
			{
				AddonType: addonType,
				// Empty is omitted; the backend picks the catalog default.
				Version: plan.Version.ValueString(),
			},
		},
	}, projectID, clusterID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Kubernetes add-on",
			fmt.Sprintf("Failed to install add-on: %s.", common.UnpackAPIError(err)))

		return
	}

	// Install is asynchronous; computed fields settle on a later refresh.
	addon, found, listHTTPResp, listErr := r.findInstalledAddon(ctx, projectID, clusterID, addonType)
	if listHTTPResp != nil {
		defer listHTTPResp.Body.Close()
	}
	if listErr != nil {
		resp.Diagnostics.AddError("Failed to create Kubernetes add-on",
			fmt.Sprintf("The add-on was installed, but reading it back failed: %s.", common.UnpackAPIError(listErr)))

		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to create Kubernetes add-on",
			fmt.Sprintf("The add-on %q was installed but did not appear in the cluster's add-on list.", addonType))

		return
	}

	var state kubernetesAddonResourceModel
	installedAddonToModel(&addon, clusterID, projectID, &state)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesAddonResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var stored kubernetesAddonResourceModel
	if err := common.GetResourceModel(ctx, req.State, &stored, &resp.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, stored.ProjectID.ValueString())
	clusterID := stored.ClusterID.ValueString()
	addonType := stored.AddonType.ValueString()

	addon, found, httpResp, err := r.findInstalledAddon(ctx, projectID, clusterID, addonType)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to get Kubernetes add-on",
			fmt.Sprintf("Failed to get add-on: %s.", common.UnpackAPIError(err)))

		return
	}
	if !found {
		resp.State.RemoveResource(ctx)

		return
	}

	var state kubernetesAddonResourceModel
	installedAddonToModel(&addon, clusterID, projectID, &state)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update is effectively unreachable: every configurable attribute is ForceNew
// or immutable. It only refreshes computed fields.
//
//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesAddonResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan kubernetesAddonResourceModel
	if err := common.GetResourceModel(ctx, req.Plan, &plan, &resp.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())
	clusterID := plan.ClusterID.ValueString()
	addonType := plan.AddonType.ValueString()

	addon, found, httpResp, err := r.findInstalledAddon(ctx, projectID, clusterID, addonType)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to update Kubernetes add-on",
			fmt.Sprintf("Failed to get add-on: %s.", common.UnpackAPIError(err)))

		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to update Kubernetes add-on",
			fmt.Sprintf("The add-on %q was not found in the cluster's add-on list.", addonType))

		return
	}

	var state kubernetesAddonResourceModel
	installedAddonToModel(&addon, clusterID, projectID, &state)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesAddonResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var stored kubernetesAddonResourceModel
	if err := common.GetResourceModel(ctx, req.State, &stored, &resp.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, stored.ProjectID.ValueString())

	_, httpResp, err := r.client.APIClient.KubernetesAddonsApi.DeleteAddon(ctx, projectID,
		stored.ClusterID.ValueString(), stored.AddonType.ValueString())
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		// A 404 means the add-on is already gone; deleting an absent resource is
		// not an error.
		if httpResp != nil && httpResp.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Failed to delete Kubernetes add-on",
			fmt.Sprintf("Failed to remove add-on: %s.", common.UnpackAPIError(err)))

		return
	}
}

func (r *kubernetesAddonResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	clusterID, addonType, projectID, errMsg := parseAddonImportID(req, r.client)
	if errMsg != "" {
		resp.Diagnostics.AddError("Invalid resource identifier", errMsg)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), buildAddonID(clusterID, addonType))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("cluster_id"), clusterID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("addon_type"), addonType)...)
}

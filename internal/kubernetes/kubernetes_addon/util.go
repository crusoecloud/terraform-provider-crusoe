package kubernetes_addon

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// addonIDSeparator joins cluster_id and addon_type into the composite resource id.
const addonIDSeparator = ":"

// apiDesc* — schema descriptions derived from the client-go swagger spec.
const (
	apiDescClusterID         = "ID of the Kubernetes cluster the add-on belongs to."
	apiDescDescription       = "Human-readable description of the add-on."
	apiDescRemovable         = "Whether the customer may remove the add-on once installed."
	apiDescVersions          = "Installable versions for the requested Kubernetes version."
	apiDescKubernetesVersion = "Kubernetes version (major.minor) to list available add-ons for."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescResource = common.LimitedAvailabilityMessage + "\n\nManages an add-on on a Crusoe Kubernetes cluster."

	providerDescAddonsDataSource = common.LimitedAvailabilityMessage + "\n\nFetches the add-ons installed on a Crusoe Kubernetes cluster."

	providerDescAvailableAddonsDataSource = common.LimitedAvailabilityMessage +
		"\n\nFetches the add-ons available for a Kubernetes version."

	providerDescID = "Composite ID of the add-on, in the form `cluster_id" + addonIDSeparator + "addon_type`."

	providerDescAddonsID = "ID of the cluster the add-ons were listed for."

	providerDescStatus = "Current installation status of the add-on. Empty until installation starts."

	providerDescAddonType = "Type of the add-on, e.g. `cert-manager`."

	providerDescVersion = "Version of the add-on. If omitted, the default version for the cluster's Kubernetes version is used."

	providerDescInstalledVersion = "Version installed on the cluster. Empty until installation completes."

	providerDescReady = "Whether the add-on is ready."

	providerDescCategory = "Category of the add-on. Possible values: `preinstalled`, `optional`."

	providerDescProjectID = "ID of the project the add-on belongs to. " + project.ProviderDescProjectIDFallback

	providerDescAddons = "List of add-ons."

	providerDescAvailableClusterID = "ID of the cluster whose Kubernetes version is used to list available add-ons. " +
		"Exactly one of cluster_id or kubernetes_version must be set."

	providerDescAvailableKubernetesVersion = apiDescKubernetesVersion +
		" Exactly one of cluster_id or kubernetes_version must be set."
)

// kubernetesAddonResourceModel is the tfsdk model for the crusoe_kubernetes_addon resource.
type kubernetesAddonResourceModel struct {
	ID               types.String `tfsdk:"id"`
	ProjectID        types.String `tfsdk:"project_id"`
	ClusterID        types.String `tfsdk:"cluster_id"`
	AddonType        types.String `tfsdk:"addon_type"`
	Version          types.String `tfsdk:"version"`
	InstalledVersion types.String `tfsdk:"installed_version"`
	Status           types.String `tfsdk:"status"`
	Ready            types.Bool   `tfsdk:"ready"`
	Category         types.String `tfsdk:"category"`
}

// installedAddonModel is one element of the crusoe_kubernetes_addons data source list.
type installedAddonModel struct {
	AddonType        types.String `tfsdk:"addon_type"`
	Version          types.String `tfsdk:"version"`
	InstalledVersion types.String `tfsdk:"installed_version"`
	Status           types.String `tfsdk:"status"`
	Ready            types.Bool   `tfsdk:"ready"`
	Category         types.String `tfsdk:"category"`
}

// kubernetesAddonsDataSourceModel is the tfsdk model for the crusoe_kubernetes_addons data source.
type kubernetesAddonsDataSourceModel struct {
	ID        types.String          `tfsdk:"id"`
	ProjectID types.String          `tfsdk:"project_id"`
	ClusterID types.String          `tfsdk:"cluster_id"`
	Addons    []installedAddonModel `tfsdk:"addons"`
}

// availableAddonModel is one element of the crusoe_kubernetes_available_addons data source list.
type availableAddonModel struct {
	AddonType   types.String `tfsdk:"addon_type"`
	Description types.String `tfsdk:"description"`
	Category    types.String `tfsdk:"category"`
	Removable   types.Bool   `tfsdk:"removable"`
	Versions    types.List   `tfsdk:"versions"`
}

// kubernetesAvailableAddonsDataSourceModel is the tfsdk model for the
// crusoe_kubernetes_available_addons data source.
type kubernetesAvailableAddonsDataSourceModel struct {
	ProjectID         types.String          `tfsdk:"project_id"`
	KubernetesVersion types.String          `tfsdk:"kubernetes_version"`
	ClusterID         types.String          `tfsdk:"cluster_id"`
	Addons            []availableAddonModel `tfsdk:"addons"`
}

// buildAddonID joins the cluster ID and add-on type into the composite resource id.
func buildAddonID(clusterID, addonType string) string {
	return clusterID + addonIDSeparator + addonType
}

// parseAddonImportID parses "cluster_id:addon_type" or
// "cluster_id:addon_type,project_id", matching common.ParseResourceIdentifiers.
func parseAddonImportID(req resource.ImportStateRequest, client *common.CrusoeClient) (
	clusterID, addonType, projectID, errMsg string,
) {
	expected := fmt.Sprintf("Expected format cluster_id%saddon_type or cluster_id%saddon_type,project_id, got %q",
		addonIDSeparator, addonIDSeparator, req.ID)

	identifiers := strings.Split(req.ID, ",")
	if len(identifiers) != 1 && len(identifiers) != 2 {
		return "", "", "", expected
	}

	parts := strings.Split(identifiers[0], addonIDSeparator)
	if len(parts) != 2 {
		return "", "", "", expected
	}

	clusterID = parts[0]
	addonType = parts[1]
	projectID = client.ProjectID
	if len(identifiers) == 2 {
		projectID = identifiers[1]
	}

	if _, parseErr := uuid.Parse(clusterID); parseErr != nil {
		return "", "", "", fmt.Sprintf("Failed to parse cluster_id: %v", parseErr)
	}

	if addonType == "" {
		return "", "", "", fmt.Sprintf("Failed to parse addon_type: must not be empty, got %q", req.ID)
	}

	if _, parseErr := uuid.Parse(projectID); parseErr != nil {
		return "", "", "", fmt.Sprintf("Failed to parse project ID: %v", parseErr)
	}

	return clusterID, addonType, projectID, ""
}

// installedAddonToModel maps an installed add-on onto a resource model.
// ListAddons doesn't echo clusterID or projectID, so the caller supplies them.
func installedAddonToModel(addon *swagger.KubernetesInstalledAddon, clusterID, projectID string, model *kubernetesAddonResourceModel) {
	model.ID = types.StringValue(buildAddonID(clusterID, addon.AddonType))
	model.ProjectID = types.StringValue(projectID)
	model.ClusterID = types.StringValue(clusterID)
	model.AddonType = types.StringValue(addon.AddonType)
	model.Version = common.StringOrNull(addon.Version)
	model.InstalledVersion = common.StringOrNull(addon.InstalledVersion)
	model.Status = common.StringOrNull(addon.Status)
	model.Ready = types.BoolValue(addon.Ready)
	model.Category = common.StringOrNull(addon.Category)
}

// installedAddonToListModel maps an installed add-on onto a data source list element.
func installedAddonToListModel(addon *swagger.KubernetesInstalledAddon) installedAddonModel {
	return installedAddonModel{
		AddonType:        types.StringValue(addon.AddonType),
		Version:          common.StringOrNull(addon.Version),
		InstalledVersion: common.StringOrNull(addon.InstalledVersion),
		Status:           common.StringOrNull(addon.Status),
		Ready:            types.BoolValue(addon.Ready),
		Category:         common.StringOrNull(addon.Category),
	}
}

// availableAddonToModel maps a catalog add-on onto a data source list element.
func availableAddonToModel(addon *swagger.KubernetesAvailableAddon) (availableAddonModel, error) {
	versions := make([]string, 0, len(addon.Versions))
	for _, v := range addon.Versions {
		versions = append(versions, v.Version)
	}

	versionList, diags := common.StringSliceToTFList(versions)
	if diags.HasError() {
		return availableAddonModel{}, fmt.Errorf("failed to convert add-on versions to a list")
	}

	return availableAddonModel{
		AddonType:   types.StringValue(addon.AddonType),
		Description: common.StringOrNull(addon.Description),
		Category:    common.StringOrNull(addon.Category),
		Removable:   types.BoolValue(addon.Removable),
		Versions:    versionList,
	}, nil
}

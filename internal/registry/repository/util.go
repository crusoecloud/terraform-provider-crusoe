package repository

import (
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec
// (Repository, with nested UpstreamRegistry and UpstreamRegistryCredentials).
const (
	apiDescLocation         = "Location the repository is hosted in."
	apiDescName             = "Name of the repository."
	apiDescMode             = "Mode of the repository, which determines how images are stored and served."
	apiDescState            = "State of the repository."
	apiDescURL              = "URL at which the repository can be accessed."
	apiDescUpstreamProvider = "Provider of the upstream registry."
	apiDescUpstreamURL      = "Base URL of the upstream registry that the repository caches images from."
	//nolint:gosec // G101: This is a description string, not actual credentials
	apiDescUpstreamCredsUsername = "Username used to authenticate to the upstream registry."
	//nolint:gosec // G101: This is a description string, not actual credentials
	apiDescUpstreamCredsPassword = "Password used to authenticate to the upstream registry."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project the repository belongs to. " + project.ProviderDescProjectIDFallback
)

// repositoryToResourceModel maps the API-owned fields of a repository onto model:
// location, name, mode, and the upstream registry provider/url all come from the
// API response. The upstream registry credentials do not — the password is
// write-only and the API never echoes it back — so the model's existing
// credentials are preserved. Callers must seed those first: from the plan in
// Create, from prior state in Read.
func repositoryToResourceModel(repository *swagger.Repository, model *repositoryResourceModel, projectID string) {
	model.Location = types.StringValue(repository.Location)
	model.Name = types.StringValue(repository.Name)
	model.Mode = types.StringValue(repository.Mode)
	model.ProjectID = types.StringValue(projectID)

	if repository.UpstreamRegistry == nil {
		model.UpstreamRegistry = nil

		return
	}

	// Preserve the caller-seeded credentials; the API supplies only provider/url.
	var credentials *upstreamRegistryCredentialsResourceModel
	if model.UpstreamRegistry != nil {
		credentials = model.UpstreamRegistry.UpstreamRegistryCrdentials
	}

	model.UpstreamRegistry = &upstreamRegistryResourceModel{
		Provider:                   types.StringValue(repository.UpstreamRegistry.Provider),
		Url:                        types.StringValue(repository.UpstreamRegistry.Url),
		UpstreamRegistryCrdentials: credentials,
	}
}

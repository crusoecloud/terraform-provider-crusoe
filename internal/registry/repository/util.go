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
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project the repository belongs to. " + project.ProviderDescProjectIDFallback

	providerDescResource = "Manages a Crusoe Container Registry repository.\n\n" +
		"A `pull-through-cache` repository whose upstream is a private AWS ECR registry " +
		"(`https://<account>.dkr.ecr.<region>.amazonaws.com`, provider `aws-ecr`) requires " +
		"`upstream_registry_credentials` when it is created. A repository created without them cannot be given " +
		"credentials later and must be recreated with credentials.\n\n" +
		"Credentials are rotated in place: change `credentials_version` to send the configured `username` and " +
		"`password`. Adding or removing the `upstream_registry` block, removing the credentials, or changing " +
		"`provider`, `url`, `name`, `location`, `mode` or `project_id`, replaces the repository.\n\n" +
		"`username` and `password` are write-only: they are never stored in the Terraform plan or state, and " +
		"setting them requires Terraform 1.11 or later."

	//nolint:gosec // G101: This is a description string, not actual credentials
	providerDescUpstreamCredsUsername = "Username used to authenticate to the upstream registry. Write-only: " +
		"never stored in the Terraform plan or state; requires Terraform 1.11 or later. A changed value is sent " +
		"only when `credentials_version` changes."
	//nolint:gosec // G101: This is a description string, not actual credentials
	providerDescUpstreamCredsPassword = "Password used to authenticate to the upstream registry. Write-only: " +
		"never stored in the Terraform plan or state; requires Terraform 1.11 or later. A changed value is sent " +
		"only when `credentials_version` changes."
	//nolint:gosec // G101: This is a description string, not actual credentials
	providerDescUpstreamCredsVersion = "Version of the upstream credentials. Change this value (for example, " +
		"increment it) to send the configured `username` and `password` and rotate them in place."
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

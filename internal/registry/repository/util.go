package repository

import (
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
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

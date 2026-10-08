package repository

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
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

// upstreamRegistryCredentialsFromModel returns the API credentials for an upstream
// registry model, or nil when the model has no credentials or both values are empty.
func upstreamRegistryCredentialsFromModel(upstreamRegistry *upstreamRegistryResourceModel) *swagger.UpstreamRegistryCredentials {
	if upstreamRegistry == nil || upstreamRegistry.UpstreamRegistryCrdentials == nil {
		return nil
	}

	username := upstreamRegistry.UpstreamRegistryCrdentials.Username.ValueString()
	password := upstreamRegistry.UpstreamRegistryCrdentials.Password.ValueString()
	if username == "" && password == "" {
		return nil
	}

	return &swagger.UpstreamRegistryCredentials{
		Username: username,
		Password: password,
	}
}

func upstreamRegistryCredentialsEqual(a, b *swagger.UpstreamRegistryCredentials) bool {
	if a == nil || b == nil {
		return a == b
	}

	return *a == *b
}

// requiresReplaceIfAddedOrRemoved replaces the repository when an object goes from
// set to unset or back. Changes inside a set object are left to its attributes.
//
//nolint:gocritic // hugeParam: req signature required by objectplanmodifier.RequiresReplaceIfFunc
func requiresReplaceIfAddedOrRemoved(_ context.Context, req planmodifier.ObjectRequest, resp *objectplanmodifier.RequiresReplaceIfFuncResponse) {
	resp.RequiresReplace = req.StateValue.IsNull() != req.PlanValue.IsNull()
}

// credentialFieldNames are the upstream registry credential attributes, in the
// resource schema and in the API request body alike.
var credentialFieldNames = []string{"username", "password"}

// requiresReplaceIfCredentialsCleared replaces the repository when a credential value
// is taken away: the whole object unset, or a value that was set emptied. The API can
// set or change upstream registry credentials in place, but not remove them. An empty
// value is omitted from the request body, so the repository would keep the value it
// has while state records the empty one — and since the credentials are write-only,
// no later read corrects that.
//
//nolint:gocritic // hugeParam: req signature required by objectplanmodifier.RequiresReplaceIfFunc
func requiresReplaceIfCredentialsCleared(_ context.Context, req planmodifier.ObjectRequest, resp *objectplanmodifier.RequiresReplaceIfFuncResponse) {
	resp.RequiresReplace = credentialsObjectCleared(req.StateValue, req.PlanValue)
}

// credentialsObjectCleared reports whether the plan empties any credential value the
// prior state had. An unknown value is never treated as emptied: it is not decided yet.
func credentialsObjectCleared(state, plan types.Object) bool {
	if state.IsNull() || state.IsUnknown() || plan.IsUnknown() {
		return false
	}

	for _, name := range credentialFieldNames {
		stateValue, ok := credentialsObjectField(state, name)
		if !ok || stateValue == "" {
			continue // nothing there to take away
		}
		if plan.IsNull() {
			return true
		}
		if planValue, ok := credentialsObjectField(plan, name); ok && planValue == "" {
			return true
		}
	}

	return false
}

// credentialsObjectField returns a credential value, and whether it is a known string.
func credentialsObjectField(credentials types.Object, name string) (string, bool) {
	value, ok := credentials.Attributes()[name].(types.String)
	if !ok || value.IsUnknown() {
		return "", false
	}

	return value.ValueString(), true
}

// credentialsCleared reports whether plan empties any credential value that state had.
func credentialsCleared(state, plan *swagger.UpstreamRegistryCredentials) bool {
	if state == nil {
		return false
	}
	if plan == nil {
		return true
	}

	return (state.Username != "" && plan.Username == "") ||
		(state.Password != "" && plan.Password == "")
}

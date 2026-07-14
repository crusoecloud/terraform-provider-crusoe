package token

import "github.com/crusoecloud/terraform-provider-crusoe/internal/common"

// apiDesc* — schema descriptions derived from the client-go swagger spec (CcrTokenResponse.token).
const (
	//nolint:gosec // G101: This is a description string, not actual credentials
	apiDescToken = "Token used to authenticate to the container registry."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	//nolint:gosec // G101: This is a description string, not actual credentials
	providerDescTokenID = "Unique identifier for the token."
	//nolint:gosec // G101: This is a description string, not actual credentials
	providerDescTokensList = "List of container registry tokens."
)

// providerDescProjectIDDeprecated marks the deprecated, no-op project_id on the data
// source. Registry tokens are org-scoped, not project-scoped, so the attribute has no
// effect; kept for backwards compatibility.
var providerDescProjectIDDeprecated = common.FormatDeprecation("v0.6.0") +
	" This field has no effect; registry tokens are org-scoped, not project-scoped."

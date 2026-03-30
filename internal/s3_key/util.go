package s3_key

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
)

var errGetResourceModel = errors.New("unable to get resource model")

// tfDataGetter is implemented by req.Plan and req.State in CRUD methods.
type tfDataGetter interface {
	Get(ctx context.Context, target interface{}) diag.Diagnostics
}

// getResourceModel extracts the resource model from plan/state with error handling.
func getResourceModel(ctx context.Context, source tfDataGetter, dest *s3KeyResourceModel, respDiags *diag.Diagnostics) error {
	diags := source.Get(ctx, dest)
	respDiags.Append(diags...)

	if respDiags.HasError() {
		return errGetResourceModel
	}

	return nil
}

// Description constants for S3 key schema attributes.
const (
	descKeyID       = "Unique identifier of the S3 key."
	descAccessKeyID = "S3-compatible access key ID (format: `S3U_...`)."
	//nolint:gosec // G101: This is a description string, not actual credentials
	descSecretAccessKey = "S3-compatible secret access key. **Only returned once on creation** - store securely."
	descAlias           = "Human-readable alias for the key."
	descStatus          = "Status of the key. Possible values: `enabled`, `disabled`."
	descCreatedAt       = "Timestamp when the key was created (RFC3339 format)."
	descExpireAt        = "Expiration timestamp for the key (RFC3339 format)."
	descUserID          = "ID of the user who owns the key."
	descOrganizationID  = "ID of the organization the key belongs to."
)

var (
	errNoOrganization        = errors.New("user does not belong to any organizations")
	errMultipleOrganizations = errors.New("user belongs to multiple organizations - specify organization_id explicitly")
)

// getUserOrg fetches the user's organization ID.
// Returns an error if the user belongs to zero or multiple organizations.
func getUserOrg(ctx context.Context, apiClient *swagger.APIClient) (string, error) {
	dataResp, httpResp, err := apiClient.EntitiesApi.GetOrganizations(ctx)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		return "", err
	}

	entities := dataResp.Items
	switch len(entities) {
	case 0:
		return "", errNoOrganization
	case 1:
		return entities[0].Id, nil
	default:
		return "", errMultipleOrganizations
	}
}

// zeroTime is the Unix epoch, returned by the API when no expiration is set.
const zeroTime = "1970-01-01T00:00:00Z"

// isValidExpireAt returns true if the expire_at value is set and not the zero time.
func isValidExpireAt(expireAt string) bool {
	return expireAt != "" && expireAt != zeroTime
}

// s3KeyAPIToDataSourceModel converts a swagger S3Key to the data source model.
func s3KeyAPIToDataSourceModel(key *swagger.S3Key) s3KeyModel {
	expireAt := ""
	if isValidExpireAt(key.ExpireAt) {
		expireAt = key.ExpireAt
	}

	return s3KeyModel{
		KeyID:       key.KeyUuid,
		AccessKeyID: key.AccessKeyId,
		Alias:       key.Alias,
		Status:      key.Status,
		CreatedAt:   key.CreatedAt,
		ExpireAt:    expireAt,
		UserID:      key.UserId,
	}
}

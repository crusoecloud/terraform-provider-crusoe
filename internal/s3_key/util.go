package s3_key

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

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

// findKeyByAccessKeyID returns the key in the list matching accessKeyID, or nil
// if none match.
func findKeyByAccessKeyID(keys []swagger.S3Key, accessKeyID string) *swagger.S3Key {
	for i := range keys {
		if keys[i].AccessKeyId == accessKeyID {
			return &keys[i]
		}
	}

	return nil
}

// s3KeyToResourceModel populates the API-backed computed fields of the resource
// model from a listed S3 key. It deliberately does not touch access_key_id /
// secret_access_key (set once on create and preserved thereafter) or
// organization_id (set by the caller from the resolved org). alias and expire_at
// are only overwritten when the API returns a meaningful value, preserving the
// configured value otherwise.
func s3KeyToResourceModel(key *swagger.S3Key, model *s3KeyResourceModel) {
	model.KeyID = types.StringValue(key.KeyUuid)
	model.Status = types.StringValue(key.Status)
	model.CreatedAt = types.StringValue(key.CreatedAt)
	model.UserID = types.StringValue(key.UserId)
	if key.Alias != "" {
		model.Alias = types.StringValue(key.Alias)
	}
	if isValidExpireAt(key.ExpireAt) {
		model.ExpireAt = types.StringValue(key.ExpireAt)
	}
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

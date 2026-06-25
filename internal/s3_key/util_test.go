package s3_key

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

func Test_findKeyByAccessKeyID(t *testing.T) {
	keys := []swagger.S3Key{
		{AccessKeyId: "AK1", KeyUuid: "u1"},
		{AccessKeyId: "AK2", KeyUuid: "u2"},
	}

	if got := findKeyByAccessKeyID(keys, "AK2"); got == nil || got.KeyUuid != "u2" {
		t.Errorf("findKeyByAccessKeyID(AK2) = %v, want key u2", got)
	}
	if got := findKeyByAccessKeyID(keys, "AK3"); got != nil {
		t.Errorf("findKeyByAccessKeyID(AK3) = %v, want nil", got)
	}
}

// Test_s3KeyToResourceModel verifies the shared mapper (used by Create and Read)
// populates computed fields from the API, preserves the configured alias/expire_at
// when the API value is empty/zero, and leaves access_key_id/secret_access_key/
// organization_id for the caller.
func Test_s3KeyToResourceModel(t *testing.T) {
	t.Run("maps API fields and overwrites alias/expire_at", func(t *testing.T) {
		model := &s3KeyResourceModel{
			AccessKeyID:     types.StringValue("AK1"),
			SecretAccessKey: types.StringValue("secret"),
			OrganizationID:  types.StringValue("org-1"),
			Alias:           types.StringValue("planned-alias"),
		}
		key := &swagger.S3Key{
			KeyUuid:   "u1",
			Status:    "enabled",
			CreatedAt: "2026-01-01T00:00:00Z",
			UserId:    "user-1",
			Alias:     "api-alias",
			ExpireAt:  "2026-12-31T23:59:59Z",
		}

		s3KeyToResourceModel(key, model)

		if got := model.KeyID.ValueString(); got != "u1" {
			t.Errorf("key_id = %q, want %q", got, "u1")
		}
		if got := model.Status.ValueString(); got != "enabled" {
			t.Errorf("status = %q, want %q", got, "enabled")
		}
		if got := model.UserID.ValueString(); got != "user-1" {
			t.Errorf("user_id = %q, want %q", got, "user-1")
		}
		if got := model.Alias.ValueString(); got != "api-alias" {
			t.Errorf("alias = %q, want %q (from API)", got, "api-alias")
		}
		if got := model.ExpireAt.ValueString(); got != "2026-12-31T23:59:59Z" {
			t.Errorf("expire_at = %q, want API value", got)
		}
		// Caller-owned fields must be left untouched by the transform.
		if got := model.AccessKeyID.ValueString(); got != "AK1" {
			t.Errorf("access_key_id = %q, want %q (untouched)", got, "AK1")
		}
		if got := model.SecretAccessKey.ValueString(); got != "secret" {
			t.Errorf("secret_access_key = %q, want preserved", got)
		}
		if got := model.OrganizationID.ValueString(); got != "org-1" {
			t.Errorf("organization_id = %q, want %q (untouched)", got, "org-1")
		}
	})

	t.Run("preserves configured alias/expire_at when API empty/zero", func(t *testing.T) {
		model := &s3KeyResourceModel{
			Alias:    types.StringValue("planned-alias"),
			ExpireAt: types.StringValue("2027-01-01T00:00:00Z"),
		}
		key := &swagger.S3Key{
			KeyUuid:  "u1",
			Alias:    "",       // API returns no alias
			ExpireAt: zeroTime, // API zero time means no expiry
		}

		s3KeyToResourceModel(key, model)

		if got := model.Alias.ValueString(); got != "planned-alias" {
			t.Errorf("alias = %q, want %q (preserved)", got, "planned-alias")
		}
		if got := model.ExpireAt.ValueString(); got != "2027-01-01T00:00:00Z" {
			t.Errorf("expire_at = %q, want preserved configured value", got)
		}
	})
}

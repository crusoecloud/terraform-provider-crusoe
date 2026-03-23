package s3_bucket

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

var errGetResourceModel = errors.New("unable to get resource model")

// tfDataGetter is implemented by tfsdk.State and tfsdk.Plan
type tfDataGetter interface {
	Get(ctx context.Context, target interface{}) diag.Diagnostics
}

// getResourceModel extracts the resource model from state or plan.
// Returns errGetResourceModel if there were errors (diagnostics already appended to respDiags).
func getResourceModel(ctx context.Context, source tfDataGetter, dest *s3BucketResourceModel, respDiags *diag.Diagnostics) error {
	diags := source.Get(ctx, dest)
	respDiags.Append(diags...)

	if respDiags.HasError() {
		return errGetResourceModel
	}

	return nil
}

const (
	descName                = "Name of the bucket. Must be DNS-compliant: 3-63 characters, lowercase letters, numbers, and hyphens."
	descProjectID           = "The project ID. If not specified, uses the provider's default project."
	descLocation            = "Location/region where the bucket will be created."
	descVersioningEnabled   = "Enable versioning for the bucket. Once enabled, cannot be disabled."
	descVersioningState     = "Versioning state of the bucket: `disabled`, `enabled`, or `suspended`."
	descObjectLockEnabled   = "Enable object lock for the bucket. Requires `versioning_enabled` to be set. Once enabled, cannot be disabled."
	descRetentionPeriod     = "Retention period for object lock. Only applicable when `object_lock_enabled` is `true`."
	descRetentionPeriodUnit = "Unit for retention period: `days` or `years`. Only applicable when `object_lock_enabled` is `true`."
	descTags                = "Tags applied to the bucket as key-value pairs."
	descCreatedAt           = "Timestamp when the bucket was created (RFC3339 format)."
	descUpdatedAt           = "Timestamp when the bucket was last updated (RFC3339 format)."
	descS3URL               = "S3 endpoint URL for accessing the bucket."
)

// bucketNameRegex validates DNS-compliant bucket names.
// Must be 3-63 characters, lowercase letters, numbers, hyphens, and periods.
// Cannot start or end with hyphen or period.
var bucketNameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)

// s3BucketToTerraformResourceModel converts an API S3Bucket response to the Terraform resource model.
func s3BucketToTerraformResourceModel(bucket *swagger.S3Bucket, model *s3BucketResourceModel) {
	model.Name = types.StringValue(bucket.Name)
	model.Location = types.StringValue(bucket.Location)
	model.CreatedAt = types.StringValue(bucket.CreatedAt)
	model.UpdatedAt = types.StringValue(bucket.UpdatedAt)

	// Convert versioning state to boolean
	model.VersioningEnabled = types.BoolValue(bucket.VersioningState == versioningStateEnabled)

	// Object lock enabled
	model.ObjectLockEnabled = types.BoolValue(bucket.ObjectLockEnabled)

	// Retention settings
	switch {
	case bucket.ObjectLockEnabled && bucket.RetentionPeriod > 0:
		// API returned retention info - use it
		model.RetentionPeriod = types.Int64Value(int64(bucket.RetentionPeriod))
		model.RetentionPeriodUnit = types.StringValue(bucket.RetentionPeriodUnit)
	case bucket.ObjectLockEnabled:
		// Object lock enabled but API didn't return retention info -
		// preserve existing model values if they're known, otherwise set null
		if model.RetentionPeriod.IsUnknown() {
			model.RetentionPeriod = types.Int64Null()
		}
		if model.RetentionPeriodUnit.IsUnknown() {
			model.RetentionPeriodUnit = types.StringNull()
		}
	default:
		// Object lock not enabled - retention settings don't apply
		model.RetentionPeriod = types.Int64Null()
		model.RetentionPeriodUnit = types.StringNull()
	}

	// Convert tags
	if len(bucket.Tags) > 0 {
		tagsMap, _ := common.StringMapToTFMap(bucket.Tags)
		model.Tags = tagsMap
	} else {
		model.Tags = types.MapNull(types.StringType)
	}

	// S3 URL
	if bucket.S3Url != "" {
		model.S3URL = types.StringValue(bucket.S3Url)
	} else {
		model.S3URL = types.StringNull()
	}
}

// parseS3BucketImportID parses the import ID which can be:
// - "bucket_name" (uses provider's default project)
// - "bucket_name,project_id" (explicit project)
func parseS3BucketImportID(importID string, client *common.CrusoeClient) (bucketName, projectID string) {
	parts := strings.Split(importID, ",")

	bucketName = parts[0]
	if len(parts) > 1 {
		projectID = parts[1]
	} else {
		projectID = client.ProjectID
	}

	return bucketName, projectID
}

// s3BucketAPIToDataSourceModel converts an API bucket to a data source model.
func s3BucketAPIToDataSourceModel(bucket *swagger.S3Bucket) s3BucketModel {
	model := s3BucketModel{
		Name:                bucket.Name,
		ProjectID:           bucket.ProjectId,
		Location:            bucket.Location,
		VersioningState:     bucket.VersioningState,
		ObjectLockEnabled:   bucket.ObjectLockEnabled,
		RetentionPeriod:     int64(bucket.RetentionPeriod),
		RetentionPeriodUnit: bucket.RetentionPeriodUnit,
		Tags:                bucket.Tags,
		CreatedAt:           bucket.CreatedAt,
		UpdatedAt:           bucket.UpdatedAt,
		S3URL:               bucket.S3Url,
	}

	return model
}

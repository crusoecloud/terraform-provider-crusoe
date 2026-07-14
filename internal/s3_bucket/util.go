package s3_bucket

import (
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec (S3Bucket;
// the name constraint is sourced from the CreateS3BucketRequest name property).
const (
	apiDescName            = "Name of the bucket."
	apiDescNameConstraint  = "Must be DNS-compliant: 3-63 characters, using lowercase letters, numbers, and hyphens."
	apiDescLocation        = "Location where the bucket is hosted."
	apiDescRetentionPeriod = "Length of the object lock retention period, in the unit given by retention_period_unit."
	apiDescTags            = "Tags applied to the bucket as key-value pairs."
	apiDescCreatedAt       = "Creation timestamp of the bucket, in RFC3339 format."
	apiDescUpdatedAt       = "Last update timestamp of the bucket, in RFC3339 format."
	apiDescS3URL           = "Endpoint URL for accessing the bucket."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	// The S3Bucket read model describes project_id as "ID of the project that owns the
	// bucket."; the fallback clause is provider-specific.
	providerDescProjectID           = "ID of the project that owns the bucket. " + project.ProviderDescProjectIDFallback
	providerDescVersioningEnabled   = "Enable versioning for the bucket. Once enabled, cannot be disabled."
	providerDescVersioningState     = "Versioning state of the bucket: `disabled`, `enabled`, or `suspended`."
	providerDescObjectLockEnabled   = "Enable object lock for the bucket. Requires `versioning_enabled` to be set. Once enabled, cannot be disabled."
	providerDescObjectLockClause    = "Only applicable when `object_lock_enabled` is `true`."
	providerDescRetentionPeriodUnit = "Unit for retention period: `days` or `years`. " + providerDescObjectLockClause
	providerDescBuckets             = "List of buckets in the project."
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

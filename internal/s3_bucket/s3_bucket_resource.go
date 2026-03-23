package s3_bucket

import (
	"context"
	"fmt"
	"sync"

	"github.com/antihax/optional"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

// bucketMutex serializes bucket operations to avoid API conflicts.
// The Crusoe API locks during bucket operations, so concurrent creates/updates/deletes
// will fail with conflict errors. This mutex ensures operations happen one at a time.
var bucketMutex sync.Mutex

const (
	versioningStateEnabled = "enabled"
)

type s3BucketResource struct {
	client *common.CrusoeClient
}

type s3BucketResourceModel struct {
	Name                types.String `tfsdk:"name"`
	ProjectID           types.String `tfsdk:"project_id"`
	Location            types.String `tfsdk:"location"`
	VersioningEnabled   types.Bool   `tfsdk:"versioning_enabled"`
	ObjectLockEnabled   types.Bool   `tfsdk:"object_lock_enabled"`
	RetentionPeriod     types.Int64  `tfsdk:"retention_period"`
	RetentionPeriodUnit types.String `tfsdk:"retention_period_unit"`
	Tags                types.Map    `tfsdk:"tags"`
	CreatedAt           types.String `tfsdk:"created_at"`
	UpdatedAt           types.String `tfsdk:"updated_at"`
	S3URL               types.String `tfsdk:"s3_url"`
}

func NewS3BucketResource() resource.Resource {
	return &s3BucketResource{}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *s3BucketResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*common.CrusoeClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", common.ErrorMsgProviderInitFailed)

		return
	}

	r.client = client
}

//nolint:gocritic // Implements Terraform defined interface
func (r *s3BucketResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_storage_s3_bucket"
}

//nolint:gocritic // Implements Terraform defined interface
func (r *s3BucketResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: common.DevelopmentMessage + "\n\nManages an S3-compatible storage bucket.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: descName,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(3, 63),
					stringvalidator.RegexMatches(bucketNameRegex, "bucket name must be DNS-compliant: lowercase letters, numbers, hyphens, and periods"),
				},
			},
			"project_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: descProjectID,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"location": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: descLocation,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"versioning_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: descVersioningEnabled,
			},
			"object_lock_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: descObjectLockEnabled,
			},
			"retention_period": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: descRetentionPeriod,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"retention_period_unit": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: descRetentionPeriodUnit,
				Validators: []validator.String{
					stringvalidator.OneOf("days", "years"),
				},
			},
			"tags": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: descTags,
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: descCreatedAt,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: descUpdatedAt,
			},
			"s3_url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: descS3URL,
			},
		},
	}
}

func (r *s3BucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	bucketName, projectID := parseS3BucketImportID(req.ID, r.client)

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), bucketName)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *s3BucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Serialize bucket operations to avoid API conflicts
	bucketMutex.Lock()
	defer bucketMutex.Unlock()

	var plan s3BucketResourceModel
	if err := getResourceModel(ctx, req.Plan, &plan, &resp.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())

	// Build create request
	createReq := swagger.CreateS3BucketRequest{
		Name:     plan.Name.ValueString(),
		Location: plan.Location.ValueString(),
	}

	// Set versioning state if enabled
	if plan.VersioningEnabled.ValueBool() {
		createReq.VersioningState = versioningStateEnabled
	}

	// Set object lock if enabled
	if plan.ObjectLockEnabled.ValueBool() {
		createReq.ObjectLockEnabled = true
		if !plan.RetentionPeriod.IsNull() && !plan.RetentionPeriod.IsUnknown() {
			createReq.RetentionPeriod = int32(plan.RetentionPeriod.ValueInt64())
		}
		if !plan.RetentionPeriodUnit.IsNull() && !plan.RetentionPeriodUnit.IsUnknown() {
			createReq.RetentionPeriodUnit = plan.RetentionPeriodUnit.ValueString()
		}
	}

	// Set tags if provided
	if !plan.Tags.IsNull() && !plan.Tags.IsUnknown() {
		tags, err := common.TFMapToStringMap(plan.Tags)
		if err != nil {
			resp.Diagnostics.AddError("Failed to process tags", err.Error())

			return
		}
		createReq.Tags = tags
	}

	_, httpResp, err := r.client.APIClient.S3BucketsApi.CreateS3Bucket(ctx, createReq, projectID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to create S3 bucket",
			fmt.Sprintf("There was an error creating the S3 bucket: %s", common.UnpackAPIError(err)))

		return
	}

	// Fetch the created bucket to get computed fields (API may not return full resource on create)
	bucket, getResp, err := r.client.APIClient.S3BucketsApi.GetS3Bucket(ctx, projectID, plan.Name.ValueString())
	if getResp != nil {
		defer getResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to read S3 bucket after creation",
			fmt.Sprintf("The bucket was created but could not be read: %s", common.UnpackAPIError(err)))

		return
	}

	// Update state from GET response
	s3BucketToTerraformResourceModel(&bucket, &plan)
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *s3BucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state s3BucketResourceModel
	if err := getResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())

	bucket, httpResp, err := r.client.APIClient.S3BucketsApi.GetS3Bucket(ctx, projectID, state.Name.ValueString())
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		// Check if bucket was deleted out of band
		if httpResp != nil && httpResp.StatusCode == 404 {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to read S3 bucket",
			fmt.Sprintf("There was an error reading the S3 bucket: %s", common.UnpackAPIError(err)))

		return
	}

	s3BucketToTerraformResourceModel(&bucket, &state)
	state.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *s3BucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Serialize bucket operations to avoid API conflicts
	bucketMutex.Lock()
	defer bucketMutex.Unlock()

	var state s3BucketResourceModel
	if err := getResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	var plan s3BucketResourceModel
	if err := getResourceModel(ctx, req.Plan, &plan, &resp.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())
	bucketName := state.Name.ValueString()

	// Check for invalid state transitions (trying to disable versioning or object lock)
	if state.VersioningEnabled.ValueBool() && !plan.VersioningEnabled.ValueBool() {
		resp.Diagnostics.AddError("Cannot disable versioning",
			"Versioning cannot be disabled once enabled. This is an irreversible operation.")

		return
	}
	if state.ObjectLockEnabled.ValueBool() && !plan.ObjectLockEnabled.ValueBool() {
		resp.Diagnostics.AddError("Cannot disable object lock",
			"Object lock cannot be disabled once enabled. This is an irreversible operation.")

		return
	}

	// Handle tag updates
	if !plan.Tags.Equal(state.Tags) {
		tags, err := common.TFMapToStringMap(plan.Tags)
		if err != nil {
			resp.Diagnostics.AddError("Failed to process tags", err.Error())

			return
		}

		updateTagsReq := swagger.UpdateS3BucketTagsRequest{
			Tags: tags,
		}

		_, httpResp, err := r.client.APIClient.S3BucketsApi.UpdateS3BucketTags(ctx, updateTagsReq, projectID, bucketName)
		if httpResp != nil {
			defer httpResp.Body.Close()
		}
		if err != nil {
			resp.Diagnostics.AddError("Failed to update bucket tags",
				fmt.Sprintf("There was an error updating the bucket tags: %s", common.UnpackAPIError(err)))

			return
		}
	}

	// Handle versioning enable
	if !state.VersioningEnabled.ValueBool() && plan.VersioningEnabled.ValueBool() {
		_, httpResp, err := r.client.APIClient.S3BucketsApi.EnableS3BucketVersioning(ctx, projectID, bucketName)
		if httpResp != nil {
			defer httpResp.Body.Close()
		}
		if err != nil {
			resp.Diagnostics.AddError("Failed to enable versioning",
				fmt.Sprintf("There was an error enabling versioning: %s", common.UnpackAPIError(err)))

			return
		}
	}

	// Handle object lock enable
	if !state.ObjectLockEnabled.ValueBool() && plan.ObjectLockEnabled.ValueBool() {
		var opts *swagger.S3BucketsApiEnableS3BucketObjectLockOpts
		if !plan.RetentionPeriod.IsNull() && !plan.RetentionPeriod.IsUnknown() {
			lockReq := swagger.EnableS3BucketObjectLockRequest{
				RetentionPeriod:     int32(plan.RetentionPeriod.ValueInt64()),
				RetentionPeriodUnit: plan.RetentionPeriodUnit.ValueString(),
			}
			opts = &swagger.S3BucketsApiEnableS3BucketObjectLockOpts{
				Body: optional.NewInterface(lockReq),
			}
		}

		_, httpResp, err := r.client.APIClient.S3BucketsApi.EnableS3BucketObjectLock(ctx, projectID, bucketName, opts)
		if httpResp != nil {
			defer httpResp.Body.Close()
		}
		if err != nil {
			resp.Diagnostics.AddError("Failed to enable object lock",
				fmt.Sprintf("There was an error enabling object lock: %s", common.UnpackAPIError(err)))

			return
		}
	}

	// Read back the current state
	bucket, httpResp, err := r.client.APIClient.S3BucketsApi.GetS3Bucket(ctx, projectID, bucketName)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to read S3 bucket after update",
			fmt.Sprintf("There was an error reading the S3 bucket: %s", common.UnpackAPIError(err)))

		return
	}

	s3BucketToTerraformResourceModel(&bucket, &plan)
	plan.ProjectID = types.StringValue(projectID)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *s3BucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Serialize bucket operations to avoid API conflicts
	bucketMutex.Lock()
	defer bucketMutex.Unlock()

	var state s3BucketResourceModel
	if err := getResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())

	httpResp, err := r.client.APIClient.S3BucketsApi.DeleteS3Bucket(ctx, projectID, state.Name.ValueString())
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete S3 bucket",
			fmt.Sprintf("There was an error deleting the S3 bucket: %s", common.UnpackAPIError(err)))

		return
	}
}

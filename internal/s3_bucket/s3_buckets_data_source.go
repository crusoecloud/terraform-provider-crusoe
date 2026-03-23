package s3_bucket

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type s3BucketsDataSource struct {
	client *common.CrusoeClient
}

type s3BucketsDataSourceModel struct {
	Buckets []s3BucketModel `tfsdk:"buckets"`
}

type s3BucketModel struct {
	Name                string            `tfsdk:"name"`
	ProjectID           string            `tfsdk:"project_id"`
	Location            string            `tfsdk:"location"`
	VersioningState     string            `tfsdk:"versioning_state"`
	ObjectLockEnabled   bool              `tfsdk:"object_lock_enabled"`
	RetentionPeriod     int64             `tfsdk:"retention_period"`
	RetentionPeriodUnit string            `tfsdk:"retention_period_unit"`
	Tags                map[string]string `tfsdk:"tags"`
	CreatedAt           string            `tfsdk:"created_at"`
	UpdatedAt           string            `tfsdk:"updated_at"`
	S3URL               string            `tfsdk:"s3_url"`
}

func NewS3BucketsDataSource() datasource.DataSource {
	return &s3BucketsDataSource{}
}

// Configure adds the provider configured client to the data source.
func (ds *s3BucketsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*common.CrusoeClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", common.ErrorMsgProviderInitFailed)

		return
	}

	ds.client = client
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *s3BucketsDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_storage_s3_buckets"
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *s3BucketsDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{
		MarkdownDescription: common.DevelopmentMessage + "\n\nFetches the list of S3-compatible storage buckets.",
		Attributes: map[string]schema.Attribute{
			"buckets": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of S3 buckets.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descName,
						},
						"project_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descProjectID,
						},
						"location": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descLocation,
						},
						"versioning_state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descVersioningState,
						},
						"object_lock_enabled": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: descObjectLockEnabled,
						},
						"retention_period": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: descRetentionPeriod,
						},
						"retention_period_unit": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descRetentionPeriodUnit,
						},
						"tags": schema.MapAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: descTags,
						},
						"created_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: descCreatedAt,
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
				},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *s3BucketsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	resp.Diagnostics.AddWarning("Development Feature", common.DevelopmentMessage)

	dataResp, httpResp, err := ds.client.APIClient.S3BucketsApi.ListS3Buckets(ctx, ds.client.ProjectID, nil)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch S3 buckets",
			fmt.Sprintf("Could not fetch S3 bucket data: %s", common.UnpackAPIError(err)))

		return
	}

	var state s3BucketsDataSourceModel
	for i := range dataResp.Items {
		state.Buckets = append(state.Buckets, s3BucketAPIToDataSourceModel(&dataResp.Items[i]))
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

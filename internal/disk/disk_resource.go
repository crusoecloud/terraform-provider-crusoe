package disk

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	validators "github.com/crusoecloud/terraform-provider-crusoe/internal/validators"
)

const (
	defaultDiskType    = "<tf-default-disk-type>" // this is not a real value, used for validation logic to determine correct value
	persistentSSD      = "persistent-ssd"
	sharedVolume       = "shared-volume"
	gibInTib           = 1024
	alternateBlockSize = 512
	defaultBlockSize   = 4096
)

type diskResource struct {
	client *common.CrusoeClient
}

type diskResourceModel struct {
	ID           types.String `tfsdk:"id"`
	ProjectID    types.String `tfsdk:"project_id"`
	Location     types.String `tfsdk:"location"`
	Name         types.String `tfsdk:"name"`
	Type         types.String `tfsdk:"type"`
	Size         types.String `tfsdk:"size"`
	SerialNumber types.String `tfsdk:"serial_number"`
	BlockSize    types.Int64  `tfsdk:"block_size"`
}

func NewDiskResource() resource.Resource {
	return &diskResource{}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *diskResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *diskResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_storage_disk"
}

//nolint:gocritic,gomnd // Implements Terraform defined interface
func (r *diskResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"project_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"location": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"type": schema.StringAttribute{
				Optional:   true,
				Computed:   true,
				Default:    stringdefault.StaticString(defaultDiskType),
				Validators: []validator.String{stringvalidator.OneOf(persistentSSD, sharedVolume)},
				PlanModifiers: []planmodifier.String{
					diskTypeModifier{},
					stringplanmodifier.RequiresReplace(), // cannot be updated in place
				},
			},
			"size": schema.StringAttribute{
				Required:   true,
				Validators: []validator.String{validators.StorageSizeValidator{}},
			},
			"serial_number": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"block_size": schema.Int64Attribute{
				Optional:   true,
				Computed:   true,
				Validators: []validator.Int64{int64validator.OneOf(alternateBlockSize, defaultBlockSize)}, // we support either 512 or 4096 bits
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplaceIfConfigured(), // cannot be updated in place
					int64planmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *diskResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	diskID, projectID, err := common.ParseResourceIdentifiers(req, r.client, "disk_id")

	if err != "" {
		resp.Diagnostics.AddError("Invalid resource identifier", err)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), diskID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *diskResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan diskResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diskType := plan.Type.ValueString()
	if diskType == "" || diskType == defaultDiskType {
		resp.Diagnostics.AddError("Disk type should be specified",
			"Disk type was not specified and Crusoe terraform module failed to set default. This is an internal Crusoe terraform error and you should not see this.")

		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())

	blockSize := plan.BlockSize.ValueInt64()
	if blockSize == 0 && diskType == persistentSSD {
		blockSize = defaultBlockSize
	}

	dataResp, httpResp, err := r.client.APIClient.DisksApi.CreateDisk(ctx, swagger.DisksPostRequestV1Alpha5{
		Name:      plan.Name.ValueString(),
		Location:  plan.Location.ValueString(),
		Type_:     diskType,
		Size:      plan.Size.ValueString(),
		BlockSize: blockSize,
	}, projectID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to create disk",
			fmt.Sprintf("There was an error starting a create disk operation (%s): %s", projectID, common.UnpackAPIError(err)))

		return
	}

	disk, _, err := common.AwaitOperationAndResolve[swagger.DiskV1Alpha5](ctx, dataResp.Operation, projectID, r.client.APIClient.DiskOperationsApi.GetStorageDisksOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create disk",
			fmt.Sprintf("There was an error creating a disk: %s", common.UnpackAPIError(err)))

		return
	}

	plan.ID = types.StringValue(disk.Id)
	plan.Type = types.StringValue(disk.Type_)
	plan.Location = types.StringValue(disk.Location)
	plan.SerialNumber = types.StringValue(disk.SerialNumber)
	plan.Size = types.StringValue(formatSize(plan.Size.ValueString(), disk.Size))
	plan.ProjectID = types.StringValue(projectID)
	plan.BlockSize = types.Int64Value(disk.BlockSize)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *diskResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state diskResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// We only have this parsing for transitioning from v1alpha4 to v1alpha5 because old tf state files will not
	// have project ID stored. So we will try to get a fallback project to pass to the API.
	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())

	disk, httpResp, err := r.client.APIClient.DisksApi.GetDisk(ctx, projectID, state.ID.ValueString())
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to get disks",
			fmt.Sprintf("Fetching Crusoe disks failed: %s\n\nIf the problem persists, contact support@crusoecloud.com", common.UnpackAPIError(err)))

		return
	}

	if httpResp.StatusCode == 404 {
		// disk has most likely been deleted out of band, so we update Terraform state to match
		resp.State.RemoveResource(ctx)

		return
	}

	diskToTerraformResourceModel(&disk, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *diskResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state diskResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan diskResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dataResp, httpResp, err := r.client.APIClient.DisksApi.ResizeDisk(ctx,
		swagger.DisksPatchRequest{Size: plan.Size.ValueString()},
		plan.ProjectID.ValueString(),
		plan.ID.ValueString(),
	)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to resize disk",
			fmt.Sprintf("There was an error starting a resize operation: %s.\n\n"+
				"Make sure the disk still exists, you are enlarging the disk,"+
				" and if the disk is attached to a VM, the VM is powered off.", common.UnpackAPIError(err)))

		return
	}

	_, _, err = common.AwaitOperationAndResolve[swagger.DiskV1Alpha5](ctx, dataResp.Operation, plan.ProjectID.ValueString(), r.client.APIClient.DiskOperationsApi.GetStorageDisksOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to resize disk",
			fmt.Sprintf("There was an error resizing a disk: %s.\n\n"+
				"Make sure the disk still exists, you are enlarging the disk,"+
				" and if the disk is attached to a VM, the VM is powered off.", common.UnpackAPIError(err)))

		return
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *diskResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state diskResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dataResp, httpResp, err := r.client.APIClient.DisksApi.DeleteDisk(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete disk",
			fmt.Sprintf("There was an error starting a delete disk operation: %s", common.UnpackAPIError(err)))

		return
	}

	_, err = common.AwaitOperation(ctx, dataResp.Operation, state.ProjectID.ValueString(), r.client.APIClient.DiskOperationsApi.GetStorageDisksOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete disk",
			fmt.Sprintf("There was an error deleting a disk: %s", common.UnpackAPIError(err)))

		return
	}
}

// formatSize takes a format size to use as a pattern and converts the sizeStr to match it.
func formatSize(format, sizeStr string) string {
	lowerFormatSize := strings.ToLower(format)
	lowerSize := strings.ToLower(sizeStr)
	if strings.HasSuffix(lowerFormatSize, "tib") && strings.HasSuffix(lowerSize, "gib") {
		if size, err := strconv.Atoi(sizeStr[:len(sizeStr)-3]); err == nil &&
			size >= gibInTib && size%gibInTib == 0 {

			return strconv.Itoa(size/gibInTib) + "TiB"
		}

		return sizeStr
	}

	if strings.HasSuffix(lowerFormatSize, "gib") && strings.HasSuffix(lowerSize, "tib") {
		if size, err := strconv.Atoi(sizeStr[:len(sizeStr)-3]); err == nil {
			return strconv.Itoa(size*gibInTib) + "GiB"
		}

		return sizeStr
	}

	return sizeStr
}

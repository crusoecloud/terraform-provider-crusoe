package disk

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	swagger "gitlab.com/crusoeenergy/island/external/client-go/swagger/v1alpha4"

	"github.com/crusoecloud/terraform-provider-crusoe/internal"
	validators "github.com/crusoecloud/terraform-provider-crusoe/internal/validators"
)

const (
	defaultDiskLocation = "mtkn-cdp-prod"
	defaultDiskType     = "persistent-ssd"
)

type diskResource struct {
	client *swagger.APIClient
}

type diskResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Location types.String `tfsdk:"location"`
	Name     types.String `tfsdk:"name"`
	Type     types.String `tfsdk:"type"`
	Size     types.String `tfsdk:"size"`
}

func NewDiskResource() resource.Resource {
	return &diskResource{}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *diskResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*swagger.APIClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", internal.ErrorMsgProviderInitFailed)

		return
	}

	r.client = client
}

//nolint:gocritic // Implements Terraform defined interface
func (r *diskResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_storage_disk"
}

//nolint:gocritic // Implements Terraform defined interface
func (r *diskResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"location": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),    // cannot be updated in place
					stringplanmodifier.UseStateForUnknown(), // maintain across updates if not explicitly changed
				},
			},
			"size": schema.StringAttribute{
				Required:   true,
				Validators: []validator.String{validators.StorageSizeValidator{}},
			},
		},
	}
}

func (r *diskResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *diskResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan diskResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diskLocation := plan.Location.ValueString()
	if diskLocation == "" {
		diskLocation = defaultDiskLocation
	}

	diskType := plan.Type.ValueString()
	if diskType == "" {
		diskType = defaultDiskType
	}

	roleID, err := internal.GetRole(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Role ID", err.Error())

		return
	}

	dataResp, httpResp, err := r.client.DisksApi.CreateDisk(ctx, swagger.DisksPostRequest{
		RoleId:   roleID,
		Location: diskLocation,
		Name:     plan.Name.ValueString(),
		Type_:    diskType,
		Size:     plan.Size.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create disk",
			fmt.Sprintf("There was an error starting a create disk operation: %s", err.Error()))

		return
	}
	defer httpResp.Body.Close()

	disk, _, err := internal.AwaitOperationAndResolve[swagger.Disk](ctx, dataResp.Operation, r.client.DiskOperationsApi.GetStorageDisksOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create disk",
			fmt.Sprintf("There was an error creating a disk: %s", err.Error()))

		return
	}

	plan.ID = types.StringValue(disk.Id)
	plan.Type = types.StringValue(disk.Type_)
	plan.Location = types.StringValue(disk.Location)

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

	dataResp, httpResp, err := r.client.DisksApi.GetDisks(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get disks",
			fmt.Sprintf("Fetching Crusoe disks failed: %s\n\nIf the problem persists, contact support@crusoeenergy.com", err.Error()))

		return
	}
	defer httpResp.Body.Close()

	var disk *swagger.Disk
	for i := range dataResp.Disks {
		if dataResp.Disks[i].Id == state.ID.ValueString() {
			disk = &dataResp.Disks[i]
		}
	}

	if disk == nil {
		// disk has most likely been deleted out of band, so we update Terraform state to match
		resp.State.RemoveResource(ctx)

		return
	}

	state.Name = types.StringValue(disk.Name)
	state.Type = types.StringValue(disk.Type_)
	state.Size = types.StringValue(disk.Size)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
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

	dataResp, httpResp, err := r.client.DisksApi.ResizeDisk(ctx,
		plan.ID.ValueString(),
		swagger.DisksPatchRequest{Size: plan.Size.ValueString()},
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to resize disk",
			fmt.Sprintf("There was an error starting a resize operation: %s.\n\n"+
				"Make sure the disk still exists, you are englarging the disk,"+
				" and if the disk is attached to a VM, the VM is powered off.", err.Error()))

		return
	}
	defer httpResp.Body.Close()

	_, _, err = internal.AwaitOperationAndResolve[swagger.Disk](ctx, dataResp.Operation, r.client.DiskOperationsApi.GetStorageDisksOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to resize disk",
			fmt.Sprintf("There was an error resizing a disk: %s.\n\n"+
				"Make sure the disk still exists, you are englarging the disk,"+
				" and if the disk is attached to a VM, the VM is powered off.", err.Error()))

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

	dataResp, httpResp, err := r.client.DisksApi.DeleteDisk(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete disk",
			fmt.Sprintf("There was an error starting a delete disk operation: %s", err.Error()))

		return
	}
	defer httpResp.Body.Close()

	_, err = internal.AwaitOperation(ctx, dataResp.Operation, r.client.DiskOperationsApi.GetStorageDisksOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete disk",
			fmt.Sprintf("There was a deleting a disk: %s", err.Error()))

		return
	}
}

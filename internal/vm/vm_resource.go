package vm

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha4"
	"github.com/crusoecloud/terraform-provider-crusoe/internal"
	validators "github.com/crusoecloud/terraform-provider-crusoe/internal/validators"
)

const defaultVMLocation = "us-northcentral1-a"

type vmResource struct {
	client *swagger.APIClient
}

type vmResourceModel struct {
	ID                types.String          `tfsdk:"id"`
	Name              types.String          `tfsdk:"name"`
	Type              types.String          `tfsdk:"type"`
	SSHKey            types.String          `tfsdk:"ssh_key"`
	Location          types.String          `tfsdk:"location"`
	Image             types.String          `tfsdk:"image"`
	StartupScript     types.String          `tfsdk:"startup_script"`
	ShutdownScript    types.String          `tfsdk:"shutdown_script"`
	IBPartitionID     types.String          `tfsdk:"ib_partition_id"`
	Disks             []vmDiskResourceModel `tfsdk:"disks"`
	NetworkInterfaces types.List            `tfsdk:"network_interfaces"`
}

type vmNetworkInterfaceResourceModel struct {
	ID            types.String        `tfsdk:"id"`
	Name          types.String        `tfsdk:"name"`
	Network       types.String        `tfsdk:"network"`
	Subnet        types.String        `tfsdk:"subnet"`
	InterfaceType types.String        `tfsdk:"interface_type"`
	PrivateIpv4   vmIPv4ResourceModel `tfsdk:"private_ipv4"`
	PublicIpv4    vmIPv4ResourceModel `tfsdk:"public_ipv4"`
}

type vmIPv4ResourceModel struct {
	Address types.String `tfsdk:"address"`
}

type vmDiskResourceModel struct {
	ID string `tfsdk:"id"`
}

func NewVMResource() resource.Resource {
	return &vmResource{}
}

func (r *vmResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *vmResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compute_instance"
}

func (r *vmResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"type": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
				Validators:    []validator.String{
					// TODO: re-enable once instance types are stabilized
					// validators.RegexValidator{RegexPattern: "^a40\\.(1|2|4|8)x|a100\\.(1|2|4|8)x|a100\\.(1|2|4|8)x|a100-80gb\\.(1|2|4|8)x$"},
				},
			},
			"ssh_key": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
				Validators:    []validator.String{validators.SSHKeyValidator{}},
			},
			"location": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"image": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"startup_script": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"shutdown_script": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"disks": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
			"network_interfaces": schema.ListNestedAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}, // maintain across updates
				NestedObject: schema.NestedAttributeObject{
					PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, // maintain across updates
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:      true,
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
						"name": schema.StringAttribute{
							Computed:      true,
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
						"network": schema.StringAttribute{
							Computed:      true,
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
						"subnet": schema.StringAttribute{
							Computed:      true,
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
						"interface_type": schema.StringAttribute{
							Computed:      true,
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
						"public_ipv4": schema.ObjectAttribute{
							Computed: true,
							AttributeTypes: map[string]attr.Type{
								"address": types.StringType,
							},
							PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
						"private_ipv4": schema.ObjectAttribute{
							Computed: true,
							AttributeTypes: map[string]attr.Type{
								"address": types.StringType,
							},
						},
					},
				},
			},
			"ib_partition_id": schema.StringAttribute{
				Optional:      true,
				Description:   "Infiniband Partition ID",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
		},
	}
}

func (r *vmResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vmResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vmResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	roleID, err := internal.GetRole(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get Role ID", err.Error())

		return
	}

	diskIds := make([]string, 0, len(plan.Disks))
	for _, d := range plan.Disks {
		diskIds = append(diskIds, d.ID)
	}

	vmLocation := plan.Location.ValueString()
	if vmLocation == "" {
		vmLocation = defaultVMLocation
	}

	dataResp, httpResp, err := r.client.VMsApi.CreateInstance(ctx, swagger.InstancesPostRequestV1Alpha3{
		RoleId:         roleID,
		Name:           plan.Name.ValueString(),
		ProductName:    plan.Type.ValueString(),
		Location:       vmLocation,
		Image:          plan.Image.ValueString(),
		SshPublicKey:   plan.SSHKey.ValueString(),
		StartupScript:  plan.StartupScript.ValueString(),
		ShutdownScript: plan.ShutdownScript.ValueString(),
		IbPartitionId:  plan.IBPartitionID.ValueString(),
		Disks:          diskIds,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create instance",
			fmt.Sprintf("There was an error starting a create instance operation: %s", err.Error()))

		return
	}
	defer httpResp.Body.Close()

	instance, _, err := internal.AwaitOperationAndResolve[swagger.InstanceV1Alpha4](
		ctx, dataResp.Operation, r.client.VMOperationsApi.GetComputeVMsInstancesOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create instance",
			fmt.Sprintf("There was an error creating a instance: %s", err.Error()))

		return
	}

	plan.ID = types.StringValue(instance.Id)

	networkInterfaces := make([]vmNetworkInterfaceResourceModel, 0, len(instance.NetworkInterfaces))
	for _, networkInterface := range instance.NetworkInterfaces {
		networkInterfaces = append(networkInterfaces, vmNetworkInterfaceResourceModel{
			ID:            types.StringValue(networkInterface.Id),
			Name:          types.StringValue(networkInterface.Name),
			Network:       types.StringValue(networkInterface.Network),
			Subnet:        types.StringValue(networkInterface.Subnet),
			InterfaceType: types.StringValue(networkInterface.InterfaceType),
			PrivateIpv4: vmIPv4ResourceModel{
				Address: types.StringValue(networkInterface.Ips[0].PrivateIpv4.Address),
			},
			PublicIpv4: vmIPv4ResourceModel{
				Address: types.StringValue(networkInterface.Ips[0].PublicIpv4.Address),
			},
		})
	}

	tNetworkInterfaces, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: vmNetworkTypeAttributes}, networkInterfaces)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.NetworkInterfaces = tNetworkInterfaces

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vmResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vmResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	instance, err := getVM(ctx, r.client, state.ID.ValueString())
	if err != nil || instance == nil {
		// instance has most likely been deleted out of band, so we update Terraform state to match
		resp.State.RemoveResource(ctx)

		return
	}

	state.ID = types.StringValue(instance.Id)
	state.Name = types.StringValue(instance.Name)
	state.Type = types.StringValue(instance.ProductName)

	disks := make([]vmDiskResourceModel, 0, len(instance.Disks))
	for _, disk := range instance.Disks {
		if !disk.IsBootDisk {
			disks = append(disks, vmDiskResourceModel{ID: disk.Id})
		}
	}
	if len(disks) > 0 {
		// only assign if disks is not empty. otherwise, intentionally keep this nil, for future comparisons
		state.Disks = disks
	}

	networkInterfaces := vmNetworkInterfacesToTerraformResourceModel(instance.NetworkInterfaces)
	tNetworkInterfaces, diags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: vmNetworkTypeAttributes,
	}, networkInterfaces)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.NetworkInterfaces = tNetworkInterfaces

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update attempts to update a VM. Currently only supports attaching/detaching disks, and requires that the
// VM be stopped.
//
//nolint:gocritic // Implements Terraform defined interface
func (r *vmResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state vmResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan vmResourceModel
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// attach/detach disks if requested
	addedDisks, removedDisks := getDisksDiff(state.Disks, plan.Disks)
	if len(addedDisks) > 0 {
		attachResp, httpResp, err := r.client.VMsApi.UpdateInstanceAttachDisks(ctx, swagger.InstancesAttachDiskPostRequest{
			AttachDisks: addedDisks,
		}, state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to attach disk",
				fmt.Sprintf("There was an error starting an attach disk operation: %s", err.Error()))

			return
		}
		defer httpResp.Body.Close()

		_, err = internal.AwaitOperation(ctx, attachResp.Operation, r.client.VMOperationsApi.GetComputeVMsInstancesOperation)
		if err != nil {
			resp.Diagnostics.AddError("Failed to attach disk",
				fmt.Sprintf("There was an error attaching a disk: %s", err.Error()))
		}
	}

	if len(removedDisks) > 0 {
		detachResp, httpResp, err := r.client.VMsApi.UpdateInstanceDetachDisks(ctx, swagger.InstancesDetachDiskPostRequest{
			DetachDisks: removedDisks,
		}, state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to detach disk",
				fmt.Sprintf("There was an error starting a detach disk operation: %s", err.Error()))
		}
		defer httpResp.Body.Close()

		_, err = internal.AwaitOperation(ctx, detachResp.Operation, r.client.VMOperationsApi.GetComputeVMsInstancesOperation)
		if err != nil {
			resp.Diagnostics.AddError("Failed to detach disk",
				fmt.Sprintf("There was an error detaching a disk: %s", err.Error()))

			return
		}
	}

	state.Disks = plan.Disks
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vmResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vmResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	instance, err := getVM(ctx, r.client, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to find instance", "Could not find a matching VM instance.")

		return
	}
	if instance.State != vmStateShutOff {
		resp.Diagnostics.AddError("Instance is running",
			"Instances must be shut off before they can be deleted. This will be changed in a future release.")

		return
	}

	delDataResp, delHttpResp, err := r.client.VMsApi.DeleteInstance(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete instance",
			fmt.Sprintf("There was an error starting a delete instance operation: %s", err.Error()))

		return
	}
	defer delHttpResp.Body.Close()

	_, _, err = internal.AwaitOperationAndResolve[interface{}](ctx, delDataResp.Operation, r.client.VMOperationsApi.GetComputeVMsInstancesOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete instance",
			fmt.Sprintf("There was an error deleting an instance: %s", err.Error()))

		return
	}
}

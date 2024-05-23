package vm

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	validators "github.com/crusoecloud/terraform-provider-crusoe/internal/validators"
)

var errProjectNotFound = errors.New("project for instance not found")

type vmResource struct {
	client *swagger.APIClient
}

type vmResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	ProjectID           types.String `tfsdk:"project_id"`
	Name                types.String `tfsdk:"name"`
	Type                types.String `tfsdk:"type"`
	SSHKey              types.String `tfsdk:"ssh_key"`
	Location            types.String `tfsdk:"location"`
	Image               types.String `tfsdk:"image"`
	StartupScript       types.String `tfsdk:"startup_script"`
	ShutdownScript      types.String `tfsdk:"shutdown_script"`
	FQDN                types.String `tfsdk:"fqdn"`
	Disks               types.List   `tfsdk:"disks"`
	NetworkInterfaces   types.List   `tfsdk:"network_interfaces"`
	HostChannelAdapters types.List   `tfsdk:"host_channel_adapters"`
}

type vmNetworkInterfaceResourceModel struct {
	ID            types.String              `tfsdk:"id"`
	Name          types.String              `tfsdk:"name"`
	Network       types.String              `tfsdk:"network"`
	Subnet        types.String              `tfsdk:"subnet"`
	InterfaceType types.String              `tfsdk:"interface_type"`
	PrivateIpv4   types.Object              `tfsdk:"private_ipv4"`
	PublicIpv4    vmPublicIPv4ResourceModel `tfsdk:"public_ipv4"`
}

type vmPublicIPv4ResourceModel struct {
	ID      types.String `tfsdk:"id"`
	Address types.String `tfsdk:"address"`
	Type    types.String `tfsdk:"type"`
}

type vmDiskResourceModel struct {
	ID             string `tfsdk:"id"`
	AttachmentType string `tfsdk:"attachment_type"`
	Mode           string `tfsdk:"mode"`
}

type vmHostChannelAdapterResourceModel struct {
	IBPartitionID string `tfsdk:"ib_partition_id"`
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
		resp.Diagnostics.AddError("Failed to initialize provider", common.ErrorMsgProviderInitFailed)

		return
	}

	r.client = client
}

func (r *vmResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compute_instance"
}

func (r *vmResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"project_id": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
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
							Required: true,
						},
						"attachment_type": schema.StringAttribute{
							Required: true,
						},
						"mode": schema.StringAttribute{
							Required:   true,
							Validators: []validator.String{validators.StorageModeValidator{}},
						},
					},
				},
			},
			"fqdn": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"network_interfaces": schema.ListNestedAttribute{
				Computed:      true,
				Optional:      true,
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
							Computed: true,
							Optional: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
								stringplanmodifier.RequiresReplace(),
							}, // cannot be updated in place
						},
						"interface_type": schema.StringAttribute{
							Computed:      true,
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
						"public_ipv4": schema.SingleNestedAttribute{
							Computed: true,
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Computed:      true,
									PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
								},
								"address": schema.StringAttribute{
									Computed:      true,
									PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
								},
								"type": schema.StringAttribute{
									Computed: true,
									Optional: true,
								},
							},
							PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
						"private_ipv4": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"address": schema.StringAttribute{
									Computed: true,
								},
							},
							PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
					},
				},
			},
			"host_channel_adapters": schema.ListNestedAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()}, // maintain across updates
				NestedObject: schema.NestedAttributeObject{
					PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, // maintain across updates
					Attributes: map[string]schema.Attribute{
						"ib_partition_id": schema.StringAttribute{
							Optional:      true,
							Description:   "Infiniband Partition ID",
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
					},
				},
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

	tDisks := make([]vmDiskResourceModel, 0, len(plan.Disks.Elements()))
	diags = plan.Disks.ElementsAs(ctx, &tDisks, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := ""
	if plan.ProjectID.ValueString() == "" {
		project, err := common.GetFallbackProject(ctx, r.client, &resp.Diagnostics)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create instance",
				fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

			return
		}
		projectID = project
	} else {
		projectID = plan.ProjectID.ValueString()
	}

	diskIds := make([]swagger.DiskAttachment, 0, len(tDisks))
	for _, d := range tDisks {
		diskIds = append(diskIds, swagger.DiskAttachment{
			AttachmentType: d.AttachmentType,
			DiskId:         d.ID,
			Mode:           d.Mode,
		})
	}

	// public static IPs
	newNetworkInterfaces := make([]swagger.NetworkInterface, 0)
	if !plan.NetworkInterfaces.IsUnknown() && !plan.NetworkInterfaces.IsNull() {
		tNetworkInterfaces := make([]vmNetworkInterfaceResourceModel, 0, len(plan.NetworkInterfaces.Elements()))
		diags = plan.NetworkInterfaces.ElementsAs(ctx, &tNetworkInterfaces, true)
		resp.Diagnostics.Append(diags...)

		for _, networkInterface := range tNetworkInterfaces {
			newNetworkInterfaces = append(newNetworkInterfaces, swagger.NetworkInterface{
				Subnet: networkInterface.Subnet.ValueString(),
				Ips: []swagger.IpAddresses{{
					PublicIpv4: &swagger.PublicIpv4Address{
						Type_: networkInterface.PublicIpv4.Type.ValueString(),
					},
				}},
			})
		}
	}

	var hostChannelAdapters []swagger.PartialHostChannelAdapter
	if !plan.HostChannelAdapters.IsUnknown() && !plan.HostChannelAdapters.IsNull() {
		tHostChannelAdapters := make([]vmHostChannelAdapterResourceModel, 0, len(plan.HostChannelAdapters.Elements()))
		diags = plan.HostChannelAdapters.ElementsAs(ctx, &tHostChannelAdapters, true)
		resp.Diagnostics.Append(diags...)

		for _, hca := range tHostChannelAdapters {
			hostChannelAdapters = []swagger.PartialHostChannelAdapter{
				{
					IbPartitionId: hca.IBPartitionID,
				},
			}
		}
	} else {
		// explicitly set a null value for non IB enabled VMs
		plan.HostChannelAdapters = types.ListNull(vmHostChannelAdapterSchema)
	}

	dataResp, httpResp, err := r.client.VMsApi.CreateInstance(ctx, swagger.InstancesPostRequestV1Alpha5{
		Name:                plan.Name.ValueString(),
		Type_:               plan.Type.ValueString(),
		Location:            plan.Location.ValueString(),
		Image:               plan.Image.ValueString(),
		SshPublicKey:        plan.SSHKey.ValueString(),
		StartupScript:       plan.StartupScript.ValueString(),
		ShutdownScript:      plan.ShutdownScript.ValueString(),
		NetworkInterfaces:   newNetworkInterfaces,
		Disks:               diskIds,
		HostChannelAdapters: hostChannelAdapters,
	}, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create instance",
			fmt.Sprintf("There was an error starting a create instance operation: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	instance, _, err := common.AwaitOperationAndResolve[swagger.InstanceV1Alpha5](
		ctx, dataResp.Operation, projectID, r.client.VMOperationsApi.GetComputeVMsInstancesOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create instance",
			fmt.Sprintf("There was an error creating a instance: %s", common.UnpackAPIError(err)))

		return
	}

	plan.ID = types.StringValue(instance.Id)
	plan.FQDN = types.StringValue(fmt.Sprintf("%s.%s.compute.internal", instance.Name, instance.Location))
	plan.ProjectID = types.StringValue(projectID)

	networkInterfaces, networkDiags := vmNetworkInterfacesToTerraformResourceModel(instance.NetworkInterfaces)
	resp.Diagnostics.Append(networkDiags...)
	plan.NetworkInterfaces = networkInterfaces
	if len(diskIds) > 0 {
		disks, diag := vmDiskAttachmentToTerraformResourceModel(diskIds)
		resp.Diagnostics.Append(diag...)
		if resp.Diagnostics.HasError() {
			return
		}

		plan.Disks = disks
	}

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

	// We only have this parsing for transitioning from v1alpha4 to v1alpha5 because old tf state files will not
	// have project ID stored. So we will try to get a fallback project to pass to the API.
	projectID := ""
	if state.ProjectID.ValueString() == "" {
		project, err := common.GetFallbackProject(ctx, r.client, &resp.Diagnostics)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create disk",
				fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

			return
		}
		projectID = project
	} else {
		projectID = state.ProjectID.ValueString()
	}

	instance, err := getVM(ctx, r.client, projectID, state.ID.ValueString())
	if err != nil || instance == nil {
		// instance has most likely been deleted out of band, so we update Terraform state to match
		resp.State.RemoveResource(ctx)

		return
	}

	vmUpdateTerraformState(instance, &state)

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
	tPlanDisks := make([]vmDiskResourceModel, 0, len(plan.Disks.Elements()))
	diags = plan.Disks.ElementsAs(ctx, &tPlanDisks, true)
	resp.Diagnostics.Append(diags...)

	tStateDisks := make([]vmDiskResourceModel, 0, len(state.Disks.Elements()))
	diags = state.Disks.ElementsAs(ctx, &tStateDisks, true)
	resp.Diagnostics.Append(diags...)

	addedDisks, removedDisks := getDisksDiff(tStateDisks, tPlanDisks)

	if len(removedDisks) > 0 {
		detachResp, httpResp, err := r.client.VMsApi.UpdateInstanceDetachDisks(ctx, swagger.InstancesDetachDiskPostRequest{
			DetachDisks: removedDisks,
		}, state.ProjectID.ValueString(), state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to detach disk",
				fmt.Sprintf("There was an error starting a detach disk operation: %s", common.UnpackAPIError(err)))
		}
		defer httpResp.Body.Close()

		_, err = common.AwaitOperation(ctx, detachResp.Operation, plan.ProjectID.ValueString(), r.client.VMOperationsApi.GetComputeVMsInstancesOperation)
		if err != nil {
			resp.Diagnostics.AddError("Failed to detach disk",
				fmt.Sprintf("There was an error detaching a disk: %s", common.UnpackAPIError(err)))

			return
		}
	}

	if len(addedDisks) > 0 {
		attachResp, httpResp, err := r.client.VMsApi.UpdateInstanceAttachDisks(ctx, swagger.InstancesAttachDiskPostRequestV1Alpha5{
			AttachDisks: addedDisks,
		}, state.ProjectID.ValueString(), state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to attach disk",
				fmt.Sprintf("There was an error starting an attach disk operation: %s", common.UnpackAPIError(err)))

			return
		}
		defer httpResp.Body.Close()

		_, err = common.AwaitOperation(ctx, attachResp.Operation, plan.ProjectID.ValueString(), r.client.VMOperationsApi.GetComputeVMsInstancesOperation)
		if err != nil {
			resp.Diagnostics.AddError("Failed to attach disk",
				fmt.Sprintf("There was an error attaching a disk: %s", common.UnpackAPIError(err)))
		}
	}

	// save intermediate results
	if len(addedDisks) > 0 || len(removedDisks) > 0 {
		state.Disks = plan.Disks
		diags = resp.State.Set(ctx, &state)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// handle toggling static/dynamic public IPs
	if !plan.NetworkInterfaces.IsUnknown() && len(plan.NetworkInterfaces.Elements()) == 1 {
		// instances must be running to toggle static public IP
		instance, httpResp, err := r.client.VMsApi.GetInstance(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to update instance network interface",
				fmt.Sprintf("There was an error fetching the instance's current state: %v", err))

			return
		}
		defer httpResp.Body.Close()
		if instance.State != StateRunning {
			resp.Diagnostics.AddError("Cannot update instance network interface",
				"The instance needs to be running before updating its public IP address.")

			return
		}

		var hostChannelAdapters []swagger.PartialHostChannelAdapter
		if !plan.HostChannelAdapters.IsUnknown() && !plan.HostChannelAdapters.IsNull() {
			tHostChannelAdapters := make([]vmHostChannelAdapterResourceModel, 0, len(plan.HostChannelAdapters.Elements()))
			diags = plan.HostChannelAdapters.ElementsAs(ctx, &tHostChannelAdapters, true)
			resp.Diagnostics.Append(diags...)

			for _, hca := range tHostChannelAdapters {
				hostChannelAdapters = []swagger.PartialHostChannelAdapter{
					{
						IbPartitionId: hca.IBPartitionID,
					},
				}
			}
		}

		var tNetworkInterfaces []vmNetworkInterfaceResourceModel
		diags = plan.NetworkInterfaces.ElementsAs(ctx, &tNetworkInterfaces, true)
		resp.Diagnostics.Append(diags...)
		patchResp, httpResp, err := r.client.VMsApi.UpdateInstance(ctx, swagger.InstancesPatchRequestV1Alpha5{
			Action: "UPDATE",
			NetworkInterfaces: []swagger.NetworkInterface{{
				Ips: []swagger.IpAddresses{{
					PublicIpv4: &swagger.PublicIpv4Address{
						Id:    tNetworkInterfaces[0].PublicIpv4.ID.ValueString(),
						Type_: tNetworkInterfaces[0].PublicIpv4.Type.ValueString(),
					},
				}},
			}},
			HostChannelAdapters: hostChannelAdapters,
		}, state.ProjectID.ValueString(), state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to update instance network interface",
				fmt.Sprintf("There was an error requesting to update the instance's network interface: %v", err))

			return
		}
		defer httpResp.Body.Close()

		_, err = common.AwaitOperation(ctx, patchResp.Operation, state.ProjectID.ValueString(), r.client.VMOperationsApi.GetComputeVMsInstancesOperation)
		if err != nil {
			resp.Diagnostics.AddError("Failed to update instance network interface",
				fmt.Sprintf("There was an error updating the instance's network interfaces: %s", common.UnpackAPIError(err)))

			return
		}

		state.NetworkInterfaces = plan.NetworkInterfaces
		diags = resp.State.Set(ctx, &state)
		resp.Diagnostics.Append(diags...)
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vmResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vmResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := getVM(ctx, r.client, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to find instance", "Could not find a matching VM instance.")

		return
	}

	delDataResp, delHttpResp, err := r.client.VMsApi.DeleteInstance(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete instance",
			fmt.Sprintf("There was an error starting a delete instance operation: %s", common.UnpackAPIError(err)))

		return
	}
	defer delHttpResp.Body.Close()

	_, _, err = common.AwaitOperationAndResolve[interface{}](ctx, delDataResp.Operation, state.ProjectID.ValueString(),
		r.client.VMOperationsApi.GetComputeVMsInstancesOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete instance",
			fmt.Sprintf("There was an error deleting an instance: %s", common.UnpackAPIError(err)))

		return
	}
}

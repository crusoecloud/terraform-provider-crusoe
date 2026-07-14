package vm

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	validators "github.com/crusoecloud/terraform-provider-crusoe/internal/validators"
)

var (
	errProjectNotFound = errors.New("project for instance not found")
	diskDetachWarning  = "To avoid potential data loss, it is critical to unmount any disks attached to the instance before detachment."
)

type vmResource struct {
	client *common.CrusoeClient
}

type vmResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	ProjectID               types.String `tfsdk:"project_id"`
	Name                    types.String `tfsdk:"name"`
	Type                    types.String `tfsdk:"type"`
	SSHKey                  types.String `tfsdk:"ssh_key"`
	Location                types.String `tfsdk:"location"`
	Image                   types.String `tfsdk:"image"`
	CustomImage             types.String `tfsdk:"custom_image"`
	StartupScript           types.String `tfsdk:"startup_script"`
	ShutdownScript          types.String `tfsdk:"shutdown_script"`
	FQDN                    types.String `tfsdk:"fqdn"`
	InternalDNSName         types.String `tfsdk:"internal_dns_name"`
	ExternalDNSName         types.String `tfsdk:"external_dns_name"`
	Disks                   types.Set    `tfsdk:"disks"`
	NetworkInterfaces       types.List   `tfsdk:"network_interfaces"`
	HostChannelAdapters     types.List   `tfsdk:"host_channel_adapters"`
	ReservationID           types.String `tfsdk:"reservation_id"`
	NvlinkDomainID          types.String `tfsdk:"nvlink_domain_id"`
	InstallCrusoeWatchAgent types.Bool   `tfsdk:"install_crusoe_watch_agent"`
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

	client, ok := req.ProviderData.(*common.CrusoeClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", common.ErrorMsgProviderInitFailed)

		return
	}

	r.client = client
}

func (r *vmResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compute_instance"
}

// resizeRequiresReplace forces a resource replacement only when an instance type change
// crosses product families (e.g. c1a -> a100). Changes within the same family (e.g.
// c1a.2x -> c1a.4x) are applied in place via the Update method; the backend validates
// whether the specific size is supported.
//
//nolint:gocritic // hugeParam: req signature required by stringplanmodifier.RequiresReplaceIfFunc
func resizeRequiresReplace(_ context.Context, req planmodifier.StringRequest,
	resp *stringplanmodifier.RequiresReplaceIfFuncResponse,
) {
	// Only relevant on update with a known, changing value.
	if req.StateValue.IsNull() || req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	if req.StateValue.ValueString() == req.PlanValue.ValueString() {
		return
	}

	oldFamily, oldOK := instanceTypeFamily(req.StateValue.ValueString())
	newFamily, newOK := instanceTypeFamily(req.PlanValue.ValueString())
	if !oldOK || !newOK || oldFamily != newFamily {
		resp.RequiresReplace = true // different family -> destroy & recreate (preserves prior behavior)

		return
	}

	// Same family -> in-place resize. The API requires the VM to be stopped first.
	resp.Diagnostics.AddWarning(
		"VM will be stopped to resize",
		"Changing the instance type resizes the VM in place. The VM will be stopped to "+
			"apply the resize, then started again if it was running before the update.",
	)
}

func (r *vmResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 2,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: apiDescID,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescName,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"project_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: providerDescProjectID,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescType,
				// Resize in place within the same product family; recreate the VM when the family changes.
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplaceIf(
					resizeRequiresReplace,
					"Recreates the VM when the instance type's product family changes; resizes in place within the same family.",
					"Recreates the VM when the instance type's product family changes; resizes in place within the same family.",
				)},
			},
			"ssh_key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescSSHKey,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
				Validators:          []validator.String{validators.SSHKeyValidator{}},
			},
			"location": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: apiDescLocation,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"image": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: apiDescImage,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"custom_image": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: apiDescCustomImage,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"startup_script": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: apiDescStartupScript,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"shutdown_script": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: apiDescShutdownScript,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"disks": schema.SetNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: apiDescDisks,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: apiDescDiskID,
						},
						"attachment_type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: apiDescDiskAttachmentType,
						},
						"mode": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: apiDescDiskMode,
							Validators:          []validator.String{validators.StorageModeValidator{}},
						},
					},
				},
				// Empty set must carry the correct element type; an untyped types.Set{}
				// fails schema validation in terraform-plugin-framework >= v1.15.
				Default: setdefault.StaticValue(types.SetValueMust(vmDiskAttachmentSchema, nil)),
			},
			"fqdn": schema.StringAttribute{
				Computed:           true,
				PlanModifiers:      []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
				DeprecationMessage: FQDNDeprecationMessage,
			},
			"internal_dns_name": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"external_dns_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: apiDescExternalDNSName,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"network_interfaces": schema.ListNestedAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: apiDescNetworkInterfaces,
				PlanModifiers:       []planmodifier.List{listplanmodifier.UseStateForUnknown()}, // maintain across updates
				NestedObject: schema.NestedAttributeObject{
					PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, // maintain across updates
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: apiDescNIID,
							PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: apiDescNIName,
							PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
						"network": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: apiDescNINetwork,
							PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
						"subnet": schema.StringAttribute{
							Computed:            true,
							Optional:            true,
							MarkdownDescription: apiDescNISubnet,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
								stringplanmodifier.RequiresReplace(),
							}, // cannot be updated in place
						},
						"interface_type": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: apiDescNIInterfaceType,
							PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
						"public_ipv4": schema.SingleNestedAttribute{
							Computed: true,
							Optional: true,
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: apiDescPublicIpv4ID,
									PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
								},
								"address": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: apiDescPublicIpv4Address,
									PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
								},
								"type": schema.StringAttribute{
									Computed:            true,
									Optional:            true,
									MarkdownDescription: apiDescPublicIpv4Type,
								},
							},
							PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
						"private_ipv4": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"address": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: apiDescPrivateIpv4Address,
								},
							},
							PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()}, // maintain across updates
						},
					},
				},
			},
			"host_channel_adapters": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: apiDescHostChannelAdapters,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"ib_partition_id": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: providerDescIBPartitionID,
						},
					},
				},
			},
			"reservation_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
				MarkdownDescription: providerDescReservationID,
				DeprecationMessage:  "This field is deprecated and will be removed in a future release. Please remove it from your configuration.",
			},
			"nvlink_domain_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: apiDescNvlinkDomainID,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"install_crusoe_watch_agent": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: apiDescInstallCrusoeWatchAgent,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.RequiresReplace(), boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *vmResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceID, projectID, errMsg := common.ParseResourceIdentifiers(req, r.client, "vm_id")
	if errMsg != "" {
		resp.Diagnostics.AddError("Failed to import VM", errMsg)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), resourceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vmResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vmResourceModel
	if err := common.GetResourceModel(ctx, req.Plan, &plan, &resp.Diagnostics); err != nil {
		return
	}

	tDisks := make([]vmDiskResourceModel, 0, len(plan.Disks.Elements()))
	diags := plan.Disks.ElementsAs(ctx, &tDisks, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())

	diskIds := make([]swagger.DiskAttachment, 0, len(tDisks))
	for _, d := range tDisks {
		diskIds = append(diskIds, swagger.DiskAttachment{
			AttachmentType: d.AttachmentType,
			DiskId:         d.ID,
			Mode:           d.Mode,
		})
	}

	// public IP types
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

	var installCrusoeWatchAgent *bool
	if !plan.InstallCrusoeWatchAgent.IsNull() && !plan.InstallCrusoeWatchAgent.IsUnknown() {
		v := plan.InstallCrusoeWatchAgent.ValueBool()
		installCrusoeWatchAgent = &v
	}

	dataResp, httpResp, err := r.client.APIClient.VMsApi.CreateInstance(ctx, swagger.InstancesPostRequestV1{
		Name:                    plan.Name.ValueString(),
		Type_:                   plan.Type.ValueString(),
		Location:                plan.Location.ValueString(),
		Image:                   plan.Image.ValueString(),
		CustomImage:             plan.CustomImage.ValueString(),
		SshPublicKey:            plan.SSHKey.ValueString(),
		StartupScript:           plan.StartupScript.ValueString(),
		ShutdownScript:          plan.ShutdownScript.ValueString(),
		NetworkInterfaces:       newNetworkInterfaces,
		Disks:                   diskIds,
		HostChannelAdapters:     hostChannelAdapters,
		NvlinkDomainId:          plan.NvlinkDomainID.ValueString(),
		InstallCrusoeWatchAgent: installCrusoeWatchAgent,
	}, projectID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to create instance",
			fmt.Sprintf("There was an error starting a create instance operation: %s", common.UnpackAPIError(err)))

		return
	}

	instance, _, err := common.AwaitOperationAndResolve[swagger.InstanceV1](
		ctx, dataResp.Operation, projectID, r.client.APIClient.VMOperationsApi.GetComputeVMsInstancesOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create instance",
			fmt.Sprintf("There was an error creating an instance: %s", common.UnpackAPIError(err)))

		return
	}

	// Capture the requested reservation_id before the transform overwrites it: it is
	// deprecated and plan-owned, so a requested-but-ignored value must be preserved
	// below to avoid an inconsistent-result error.
	requestedReservationID := plan.ReservationID

	vmToTerraformResourceModel(instance, &plan)

	// reservation_id is deprecated: when the backend ignores a requested reservation
	// (the transform then stored the empty API value), warn and keep the requested
	// value so it still matches the practitioner's config.
	if instance.ReservationId == "" &&
		!requestedReservationID.IsNull() && !requestedReservationID.IsUnknown() && requestedReservationID.ValueString() != "" {

		resp.Diagnostics.AddWarning("Reservation Assignment Deprecated",
			"Reservation assignment during VM creation is deprecated. The requested reservation_id was ignored by the backend. Please remove reservation_id from your configuration to suppress this warning.")
		plan.ReservationID = requestedReservationID
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vmResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vmResourceModel
	if err := common.GetResourceModel(ctx, req.State, &state, &resp.Diagnostics); err != nil {
		return
	}

	// We only have this parsing for transitioning from v1alpha4 to V1 because old tf state files will not
	// have project ID stored. So we will try to get a fallback project to pass to the API.
	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())

	instance, err := getVM(ctx, r.client.APIClient, projectID, state.ID.ValueString())
	if err != nil || instance == nil {
		// instance has most likely been deleted out of band, so we update Terraform state to match
		resp.State.RemoveResource(ctx)

		return
	}

	vmToTerraformResourceModel(instance, &state)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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
		resp.Diagnostics.AddWarning("Disk Detachment", diskDetachWarning)
		detachResp, httpResp, err := r.client.APIClient.VMsApi.UpdateInstanceDetachDisks(ctx, swagger.InstancesDetachDiskPostRequest{
			DetachDisks: removedDisks,
		}, state.ProjectID.ValueString(), state.ID.ValueString())
		if httpResp != nil {
			defer httpResp.Body.Close()
		}
		if err != nil {
			resp.Diagnostics.AddError("Failed to detach disk",
				fmt.Sprintf("There was an error starting a detach disk operation: %s", common.UnpackAPIError(err)))
		}

		_, err = common.AwaitOperation(ctx, detachResp.Operation, plan.ProjectID.ValueString(), r.client.APIClient.VMOperationsApi.GetComputeVMsInstancesOperation)
		if err != nil {
			resp.Diagnostics.AddError("Failed to detach disk",
				fmt.Sprintf("There was an error detaching a disk: %s", common.UnpackAPIError(err)))

			return
		}
	}

	if len(addedDisks) > 0 {
		attachResp, httpResp, err := r.client.APIClient.VMsApi.UpdateInstanceAttachDisks(ctx, swagger.InstancesAttachDiskPostRequestV1{
			AttachDisks: addedDisks,
		}, state.ProjectID.ValueString(), state.ID.ValueString())
		if httpResp != nil {
			defer httpResp.Body.Close()
		}
		if err != nil {
			resp.Diagnostics.AddError("Failed to attach disk",
				fmt.Sprintf("There was an error starting an attach disk operation: %s", common.UnpackAPIError(err)))

			return
		}

		_, err = common.AwaitOperation(ctx, attachResp.Operation, plan.ProjectID.ValueString(), r.client.APIClient.VMOperationsApi.GetComputeVMsInstancesOperation)
		if err != nil {
			resp.Diagnostics.AddError("Failed to attach disk",
				fmt.Sprintf("There was an error attaching a disk: %s", common.UnpackAPIError(err)))

			return
		}
	}

	// save disk results
	state.Disks = plan.Disks
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// handle updating public IP type, but only when the network interface configuration
	// actually changed. This avoids a redundant (and running-only) public IP update on
	// every apply - e.g. a type-only resize, which would otherwise fail here if the VM
	// is stopped (resizing leaves the VM stopped).
	if !plan.NetworkInterfaces.IsUnknown() && len(plan.NetworkInterfaces.Elements()) == 1 &&
		!plan.NetworkInterfaces.Equal(state.NetworkInterfaces) {
		// instances must be running to update public IP type
		instance, httpResp, err := r.client.APIClient.VMsApi.GetInstance(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
		if httpResp != nil {
			defer httpResp.Body.Close()
		}
		if err != nil {
			resp.Diagnostics.AddError("Failed to update instance network interface",
				fmt.Sprintf("There was an error fetching the instance's current state: %v", err))

			return
		}
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
		patchResp, httpResp, err := r.client.APIClient.VMsApi.UpdateInstance(ctx, swagger.InstancesPatchRequestV1{
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
		if httpResp != nil {
			defer httpResp.Body.Close()
		}
		if err != nil {
			resp.Diagnostics.AddError("Failed to update instance network interface",
				fmt.Sprintf("There was an error requesting to update the instance's network interface: %v", err))

			return
		}

		_, err = common.AwaitOperation(ctx, patchResp.Operation, state.ProjectID.ValueString(), r.client.APIClient.VMOperationsApi.GetComputeVMsInstancesOperation)
		if err != nil {
			resp.Diagnostics.AddError("Failed to update instance network interface",
				fmt.Sprintf("There was an error updating the instance's network interfaces: %s", common.UnpackAPIError(err)))

			return
		}

		state.NetworkInterfaces = plan.NetworkInterfaces
		diags = resp.State.Set(ctx, &state)
		resp.Diagnostics.Append(diags...)
	}

	// resize the instance in place if the type changed within the same product family.
	// (Cross-family changes trigger a replace via the schema plan modifier and never reach here.)
	if !plan.Type.IsUnknown() && !plan.Type.IsNull() && plan.Type.ValueString() != state.Type.ValueString() {
		// fetch the current power state; the backend requires the VM to be stopped before a resize.
		instance, httpResp, err := r.client.APIClient.VMsApi.GetInstance(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
		if httpResp != nil {
			defer httpResp.Body.Close()
		}
		if err != nil {
			resp.Diagnostics.AddError("Failed to resize instance",
				fmt.Sprintf("There was an error fetching the instance's current state: %s", common.UnpackAPIError(err)))

			return
		}

		// stop the VM first if it isn't already stopped.
		wasRunning := instance.State != StateStopped && instance.State != StateShutoff
		if wasRunning {
			stopResp, stopHTTPResp, stopErr := r.client.APIClient.VMsApi.UpdateInstance(ctx, swagger.InstancesPatchRequestV1{
				Action: "STOP",
			}, state.ProjectID.ValueString(), state.ID.ValueString())
			if stopHTTPResp != nil {
				defer stopHTTPResp.Body.Close()
			}
			if stopErr != nil {
				resp.Diagnostics.AddError("Failed to resize instance",
					fmt.Sprintf("There was an error stopping the instance before resizing: %s", common.UnpackAPIError(stopErr)))

				return
			}

			_, stopErr = common.AwaitOperation(ctx, stopResp.Operation, state.ProjectID.ValueString(), r.client.APIClient.VMOperationsApi.GetComputeVMsInstancesOperation)
			if stopErr != nil {
				resp.Diagnostics.AddError("Failed to resize instance",
					fmt.Sprintf("There was an error stopping the instance before resizing: %s", common.UnpackAPIError(stopErr)))

				return
			}
		}

		resizeResp, httpResp, err := r.client.APIClient.VMsApi.UpdateInstance(ctx, swagger.InstancesPatchRequestV1{
			Action: "UPDATE",
			Type_:  plan.Type.ValueString(),
		}, state.ProjectID.ValueString(), state.ID.ValueString())
		if httpResp != nil {
			defer httpResp.Body.Close()
		}
		if err != nil {
			resp.Diagnostics.AddError("Failed to resize instance",
				fmt.Sprintf("There was an error requesting to resize the instance: %s", common.UnpackAPIError(err)))

			return
		}

		_, err = common.AwaitOperation(ctx, resizeResp.Operation, state.ProjectID.ValueString(), r.client.APIClient.VMOperationsApi.GetComputeVMsInstancesOperation)
		if err != nil {
			resp.Diagnostics.AddError("Failed to resize instance",
				fmt.Sprintf("There was an error resizing the instance: %s", common.UnpackAPIError(err)))

			return
		}

		// Persist the new type before attempting the restart, so a restart failure
		// does not lose the completed resize from state.
		state.Type = plan.Type
		diags = resp.State.Set(ctx, &state)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		// restore the prior power state: resizing leaves the VM stopped, so start it
		// again if it was running before the resize.
		if wasRunning {
			startResp, startHTTPResp, startErr := r.client.APIClient.VMsApi.UpdateInstance(ctx, swagger.InstancesPatchRequestV1{
				Action: "START",
			}, state.ProjectID.ValueString(), state.ID.ValueString())
			if startHTTPResp != nil {
				defer startHTTPResp.Body.Close()
			}
			if startErr != nil {
				resp.Diagnostics.AddError("Failed to start instance after resize",
					fmt.Sprintf("The instance was resized but could not be restarted: %s", common.UnpackAPIError(startErr)))

				return
			}

			_, startErr = common.AwaitOperation(ctx, startResp.Operation, state.ProjectID.ValueString(), r.client.APIClient.VMOperationsApi.GetComputeVMsInstancesOperation)
			if startErr != nil {
				resp.Diagnostics.AddError("Failed to start instance after resize",
					fmt.Sprintf("The instance was resized but could not be restarted: %s", common.UnpackAPIError(startErr)))

				return
			}
		}
	}

	//  Reservation ID is deprecated
	if !plan.ReservationID.IsNull() && !plan.ReservationID.IsUnknown() && plan.ReservationID.ValueString() != "" {
		resp.Diagnostics.AddWarning("Reservation Assignment Deprecated",
			"Reservation assignment during VM creation is deprecated. The requested reservation_id was ignored by the backend. Please remove reservation_id from your configuration to suppress this warning.")
	}

	debugMsg := "Setting state Reservation ID equal to plan Reservation ID, since the field is deprecated"
	tflog.Debug(ctx, debugMsg, map[string]interface{}{})
	state.ReservationID = plan.ReservationID
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vmResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vmResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := getVM(ctx, r.client.APIClient, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to find instance", "Could not find a matching VM instance.")

		return
	}

	delDataResp, delHttpResp, err := r.client.APIClient.VMsApi.DeleteInstance(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if delHttpResp != nil {
		defer delHttpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete instance",
			fmt.Sprintf("There was an error starting a delete instance operation: %s", common.UnpackAPIError(err)))

		return
	}

	_, _, err = common.AwaitOperationAndResolve[interface{}](ctx, delDataResp.Operation, state.ProjectID.ValueString(),
		r.client.APIClient.VMOperationsApi.GetComputeVMsInstancesOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete instance",
			fmt.Sprintf("There was an error deleting an instance: %s", common.UnpackAPIError(err)))

		return
	}
}

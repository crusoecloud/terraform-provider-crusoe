package vm

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	validators "github.com/crusoecloud/terraform-provider-crusoe/internal/validators"
)

type vmByTemplateResource struct {
	client *common.CrusoeClient
}

type vmByTemplateResourceModel struct {
	NamePrefix          types.String `tfsdk:"name_prefix"`
	InstanceTemplateID  types.String `tfsdk:"instance_template"`
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
	InternalDNSName     types.String `tfsdk:"internal_dns_name"`
	ExternalDNSName     types.String `tfsdk:"external_dns_name"`
	Disks               types.Set    `tfsdk:"disks"`
	NetworkInterfaces   types.List   `tfsdk:"network_interfaces"`
	HostChannelAdapters types.List   `tfsdk:"host_channel_adapters"`
	ReservationID       types.String `tfsdk:"reservation_id"`
	NvlinkDomainID      types.String `tfsdk:"nvlink_domain_id"`
}

func NewVMByTemplateResource() resource.Resource {
	return &vmByTemplateResource{}
}

func (r *vmByTemplateResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *vmByTemplateResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compute_instance_by_template"
}

func (r *vmByTemplateResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 2,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"instance_template": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"name_prefix": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"name": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"project_id": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"type": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"ssh_key": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"location": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"image": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"startup_script": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"shutdown_script": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"disks": schema.SetNestedAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.Set{setplanmodifier.UseStateForUnknown()}, // maintain across updates
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
				Computed:           true,
				PlanModifiers:      []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
				DeprecationMessage: FQDNDeprecationMessage,
			},
			"internal_dns_name": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"external_dns_name": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
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
							Optional:      true,
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
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
				Computed:      true,
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
			"reservation_id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
				Description:   "(Deprecated) ID of the reservation to which the VM belongs. If not provided or null, the lowest-cost reservation will be used by default. To opt out of using a reservation, set this to an empty string.",
			},
			"nvlink_domain_id": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
				Description:   "NVLink domain ID to use for NVLink communication.",
			},
		},
	}
}

func (r *vmByTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vmByTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan vmByTemplateResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())
	instanceTemplateID := plan.InstanceTemplateID.ValueString()
	if _, err := uuid.Parse(instanceTemplateID); err != nil {
		resp.Diagnostics.AddError("Failed to create instance",
			fmt.Sprintf("The instance template ID is not a valid UUID: %v", err))

		return
	}
	instanceTemplateResp, httpResp, err := r.client.APIClient.InstanceTemplatesApi.GetInstanceTemplate(ctx, instanceTemplateID, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create instance",
			fmt.Sprintf("There was an error fetching the instance template: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	dataResp, httpResp, err := r.client.APIClient.VMsApi.BulkCreateInstance(ctx, swagger.BulkInstancePostRequestV1Alpha5{
		NamePrefix:         plan.NamePrefix.ValueString(),
		Count:              1,
		InstanceTemplateId: instanceTemplateID,
	}, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create instance",
			fmt.Sprintf("There was an error starting a create instance operation: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	instances, _, err := common.AwaitOperationAndResolve[[]swagger.InstanceV1Alpha5](
		ctx, dataResp.Operation, projectID, r.client.APIClient.VMOperationsApi.GetComputeVMsInstancesOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create instance",
			fmt.Sprintf("There was an error creating an instance: %s", common.UnpackAPIError(err)))

		return
	}
	instancesList := *instances
	if len(instancesList) < 1 {
		resp.Diagnostics.AddError("Failed to create instance",
			"Failed to create instance: no instance was created")

		return
	}
	instance := instancesList[0]

	plan.ID = types.StringValue(instance.Id)
	plan.Name = types.StringValue(instance.Name)
	plan.ProjectID = types.StringValue(projectID)
	plan.Type = types.StringValue(instance.Type_)
	plan.Location = types.StringValue(instance.Location)

	if instance.ReservationId != "" {
		plan.ReservationID = types.StringValue(instance.ReservationId)
	} else if !plan.ReservationID.IsNull() && !plan.ReservationID.IsUnknown() && plan.ReservationID.ValueString() != "" {
		resp.Diagnostics.AddWarning("Reservation Assignment Deprecated",
			"Reservation assignment during VM creation is deprecated. The requested reservation_id was ignored by the backend. Please remove reservation_id from your configuration to suppress this warning.")
	} else {
		plan.ReservationID = types.StringNull()
	}

	plan.Image = types.StringValue(instanceTemplateResp.ImageName)
	plan.SSHKey = types.StringValue(instanceTemplateResp.SshPublicKey)
	plan.StartupScript = types.StringValue(instanceTemplateResp.StartupScript)
	plan.ShutdownScript = types.StringValue(instanceTemplateResp.ShutdownScript)

	if instanceTemplateResp.NvlinkDomainId != "" {
		plan.NvlinkDomainID = types.StringValue(instanceTemplateResp.NvlinkDomainId)
	} else {
		plan.NvlinkDomainID = types.StringNull()
	}

	internalDNSName := types.StringValue(fmt.Sprintf("%s.%s.compute.internal", instance.Name, instance.Location))
	plan.InternalDNSName = internalDNSName
	plan.FQDN = internalDNSName // fqdn is deprecated but kept for backward compatibility

	if len(instance.NetworkInterfaces) > 0 {
		plan.ExternalDNSName = types.StringValue(instance.NetworkInterfaces[0].ExternalDnsName)
	} else {
		plan.ExternalDNSName = types.StringNull()
	}

	networkInterfaces, _ := vmNetworkInterfacesToTerraformResourceModel(instance.NetworkInterfaces)
	plan.NetworkInterfaces = networkInterfaces

	hostChannelAdapters := make([]vmHostChannelAdapterResourceModel, 0, len(instance.HostChannelAdapters))
	for _, hca := range instance.HostChannelAdapters {
		hostChannelAdapters = append(hostChannelAdapters, vmHostChannelAdapterResourceModel{IBPartitionID: hca.IbPartitionId})
	}
	hostChannelAdaptersList, _ := types.ListValueFrom(context.Background(), vmHostChannelAdapterSchema, hostChannelAdapters)
	plan.HostChannelAdapters = hostChannelAdaptersList

	if len(instance.Disks) > 0 {
		attachments := []vmDiskResourceModel{}
		for _, diskAttachment := range instance.Disks {
			attachments = append(attachments, vmDiskResourceModel{
				ID:             diskAttachment.Id,
				AttachmentType: diskAttachment.AttachmentType,
				Mode:           diskAttachment.Mode,
			})
		}

		diskAttachmentsSet, diskDiags := types.SetValueFrom(context.Background(), vmDiskAttachmentSchema, attachments)
		resp.Diagnostics.Append(diskDiags...)
		if resp.Diagnostics.HasError() {
			return
		}

		plan.Disks = diskAttachmentsSet
	}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vmByTemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state vmByTemplateResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// We only have this parsing for transitioning from v1alpha4 to v1alpha5 because old tf state files will not
	// have project ID stored. So we will try to get a fallback project to pass to the API.
	projectID := common.GetProjectIDOrFallback(r.client, state.ProjectID.ValueString())

	instance, err := getVM(ctx, r.client.APIClient, projectID, state.ID.ValueString())
	if err != nil || instance == nil {
		// instance has most likely been deleted out of band, so we update Terraform state to match
		resp.State.RemoveResource(ctx)

		return
	}

	var vmState vmResourceModel
	vmToTerraformResourceModel(instance, &vmState)
	resp.State.Set(ctx, &vmState)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update attempts to update a VM. Currently only supports attaching/detaching disks, and requires that the
// VM be stopped.
//
//nolint:gocritic // Implements Terraform defined interface
func (r *vmByTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state vmByTemplateResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var plan vmByTemplateResourceModel
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
		detachResp, httpResp, err := r.client.APIClient.VMsApi.UpdateInstanceDetachDisks(ctx, swagger.InstancesDetachDiskPostRequest{
			DetachDisks: removedDisks,
		}, state.ProjectID.ValueString(), state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to detach disk",
				fmt.Sprintf("There was an error starting a detach disk operation: %s", common.UnpackAPIError(err)))
		}
		defer httpResp.Body.Close()

		_, err = common.AwaitOperation(ctx, detachResp.Operation, plan.ProjectID.ValueString(), r.client.APIClient.VMOperationsApi.GetComputeVMsInstancesOperation)
		if err != nil {
			resp.Diagnostics.AddError("Failed to detach disk",
				fmt.Sprintf("There was an error detaching a disk: %s", common.UnpackAPIError(err)))

			return
		}
	}

	if len(addedDisks) > 0 {
		attachResp, httpResp, err := r.client.APIClient.VMsApi.UpdateInstanceAttachDisks(ctx, swagger.InstancesAttachDiskPostRequestV1Alpha5{
			AttachDisks: addedDisks,
		}, state.ProjectID.ValueString(), state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to attach disk",
				fmt.Sprintf("There was an error starting an attach disk operation: %s", common.UnpackAPIError(err)))

			return
		}
		defer httpResp.Body.Close()

		_, err = common.AwaitOperation(ctx, attachResp.Operation, plan.ProjectID.ValueString(), r.client.APIClient.VMOperationsApi.GetComputeVMsInstancesOperation)
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

	// handle updating public IP type
	if !plan.NetworkInterfaces.IsUnknown() && len(plan.NetworkInterfaces.Elements()) == 1 {
		// instances must be running to change public IP type
		instance, httpResp, err := r.client.APIClient.VMsApi.GetInstance(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
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
		patchResp, httpResp, err := r.client.APIClient.VMsApi.UpdateInstance(ctx, swagger.InstancesPatchRequestV1Alpha5{
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
	// add a reservation ID
	if plan.ReservationID.ValueString() != "" && state.ReservationID.ValueString() == "" {
		patchResp, httpResp, err := r.client.APIClient.VMsApi.UpdateInstance(ctx, swagger.InstancesPatchRequestV1Alpha5{
			Action:        "RESERVE",
			ReservationId: plan.ReservationID.String(),
		}, state.ProjectID.ValueString(), state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to add vm to reservation",
				fmt.Sprintf("There was an error requesting add vm to reservation: %v", err))

			return
		}
		defer httpResp.Body.Close()

		instance, _, err := common.AwaitOperationAndResolve[swagger.InstanceV1Alpha5](ctx, patchResp.Operation, state.ProjectID.ValueString(), r.client.APIClient.VMOperationsApi.GetComputeVMsInstancesOperation)
		if err != nil {
			resp.Diagnostics.AddError("Failed to update reservation ID",
				fmt.Sprintf("There was an error reserving the vm: %s", common.UnpackAPIError(err)))

			return
		}

		if instance.ReservationId == "" && plan.ReservationID.ValueString() != "" {
			resp.Diagnostics.AddWarning("Reservation Assignment Deprecated",
				"Reservation assignment during VM update is deprecated. The requested reservation_id was ignored by the backend. Please remove reservation_id from your configuration to suppress this warning.")
		}

		state.ReservationID = plan.ReservationID
		diags = resp.State.Set(ctx, &state)
		resp.Diagnostics.Append(diags...)
	} else if plan.ReservationID.ValueString() == "" && state.ReservationID.ValueString() != "" {
		// remove reservation ID
		patchResp, httpResp, err := r.client.APIClient.VMsApi.UpdateInstance(ctx, swagger.InstancesPatchRequestV1Alpha5{
			Action: "UNRESERVE",
		}, state.ProjectID.ValueString(), state.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to remove vm from reservation",
				fmt.Sprintf("There was an error requesting remove vm from reservation: %v", err))

			return
		}
		defer httpResp.Body.Close()

		_, err = common.AwaitOperation(ctx, patchResp.Operation, state.ProjectID.ValueString(), r.client.APIClient.VMOperationsApi.GetComputeVMsInstancesOperation)
		if err != nil {
			resp.Diagnostics.AddError("Failed to update reservation ID",
				fmt.Sprintf("There was an error unreserving the vm: %s", common.UnpackAPIError(err)))

			return
		}

		state.ReservationID = plan.ReservationID
		diags = resp.State.Set(ctx, &state)
		resp.Diagnostics.Append(diags...)
	} else if plan.ReservationID.ValueString() != "" && state.ReservationID.ValueString() != "" && plan.ReservationID.String() != state.ReservationID.String() {
		resp.Diagnostics.AddError("Failed to update reservation ID",
			"Reservation ID cannot be updated in-place. Please remove the reservation ID and re-add it.")

		return
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *vmByTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vmByTemplateResourceModel
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
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete instance",
			fmt.Sprintf("There was an error starting a delete instance operation: %s", common.UnpackAPIError(err)))

		return
	}
	defer delHttpResp.Body.Close()

	_, _, err = common.AwaitOperationAndResolve[interface{}](ctx, delDataResp.Operation, state.ProjectID.ValueString(),
		r.client.APIClient.VMOperationsApi.GetComputeVMsInstancesOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete instance",
			fmt.Sprintf("There was an error deleting an instance: %s", common.UnpackAPIError(err)))

		return
	}
}

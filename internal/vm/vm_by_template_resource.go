package vm

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-uuid"
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

type vmByTemplateResource struct {
	client *swagger.APIClient
}

type vmByTemplateResourceModel struct {
	vmResourceModel
	InstanceTemplateID types.String `tfsdk:"instance_template"`
}

func NewvmByTemplateResource() resource.Resource {
	return &vmByTemplateResource{}
}

func (r *vmByTemplateResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *vmByTemplateResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compute_instance"
}

func (r *vmByTemplateResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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
			"instance_template": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"type": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
				Validators:    []validator.String{
					// TODO: re-enable once instance types are stabilized
					// validators.RegexValidator{RegexPattern: "^a40\\.(1|2|4|8)x|a100\\.(1|2|4|8)x|a100\\.(1|2|4|8)x|a100-80gb\\.(1|2|4|8)x$"},
				},
			},
			"ssh_key": schema.StringAttribute{
				Optional:      true,
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
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
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
	instanceTemplateID := plan.InstanceTemplateID.ValueString()
	if _, err := uuid.ParseUUID(instanceTemplateID); err != nil {
		resp.Diagnostics.AddError("Failed to create instance",
			fmt.Sprintf("The instance template ID is not a valid UUID: %v", err))

		return
	}
	instanceTemplateResp, httpResp, err := r.client.InstanceTemplatesApi.GetInstanceTemplate(ctx, projectID, instanceTemplateID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create instance",
			fmt.Sprintf("There was an error fetching the instance template: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	dataResp, httpResp, err := r.client.VMsApi.BulkCreateInstance(ctx, swagger.BulkInstancePostRequestV1Alpha5{
		NamePrefix:         plan.Name.ValueString(),
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
		ctx, dataResp.Operation, projectID, r.client.VMOperationsApi.GetComputeVMsInstancesOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create instance",
			fmt.Sprintf("There was an error creating a instance: %s", common.UnpackAPIError(err)))

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
	plan.FQDN = types.StringValue(fmt.Sprintf("%s.%s.compute.internal", instance.Name, instance.Location))
	plan.ProjectID = types.StringValue(projectID)
	plan.Type = types.StringValue(instance.Type_)
	plan.Location = types.StringValue(instance.Location)
	plan.Image = types.StringValue(instanceTemplateResp.Image)
	plan.SSHKey = types.StringValue(instanceTemplateResp.SshPublicKey)
	plan.StartupScript = types.StringValue(instanceTemplateResp.StartupScript)
	plan.ShutdownScript = types.StringValue(instanceTemplateResp.ShutdownScript)

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

		diskAttachmentsList, diags := types.ListValueFrom(context.Background(), vmDiskAttachmentSchema, attachments)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		plan.Disks = diskAttachmentsList
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

	vmToTerraformResourceModel(instance, &state.vmResourceModel)

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
func (r *vmByTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state vmByTemplateResourceModel
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

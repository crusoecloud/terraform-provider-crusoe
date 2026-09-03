package instance_template

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	validators "github.com/crusoecloud/terraform-provider-crusoe/internal/validators"
)

const (
	spreadPlacementPolicy      = "spread"
	unspecifiedPlacementPolicy = "unspecified"
	persistentSSD              = "persistent-ssd"
	sharedVolume               = "shared-volume"
)

type instanceTemplateResource struct {
	client *common.CrusoeClient
}

type instanceTemplateResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	ProjectID            types.String `tfsdk:"project_id"`
	Name                 types.String `tfsdk:"name"`
	Type                 types.String `tfsdk:"type"`
	SSHKey               types.String `tfsdk:"ssh_key"`
	Location             types.String `tfsdk:"location"`
	Image                types.String `tfsdk:"image"`
	StartupScript        types.String `tfsdk:"startup_script"`
	ShutdownScript       types.String `tfsdk:"shutdown_script"`
	Subnet               types.String `tfsdk:"subnet"`
	IBPartition          types.String `tfsdk:"ib_partition"`
	TransportPartitionID types.String `tfsdk:"transport_partition_id"`
	PublicIpAddressType  types.String `tfsdk:"public_ip_address_type"`
	DisksToCreate        types.Set    `tfsdk:"disks"`
	SharedVolumes        types.Set    `tfsdk:"shared_volume_attachments"`
	ReservationID        types.String `tfsdk:"reservation_id"`
	PlacementPolicy      types.String `tfsdk:"placement_policy"`
	NvlinkDomainID       types.String `tfsdk:"nvlink_domain_id"`
}

type diskToCreateResourceModel struct {
	Size types.String `tfsdk:"size"`
	Type types.String `tfsdk:"type"`
}

var diskToCreateSchema = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"size": types.StringType,
		"type": types.StringType,
	},
}

func NewInstanceTemplateResource() resource.Resource {
	return &instanceTemplateResource{}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceTemplateResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
func (r *instanceTemplateResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_instance_template"
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceTemplateResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   apiDescID,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   apiDescName,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"project_id": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   providerDescProjectID,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"type": schema.StringAttribute{
				Required:      true,
				Description:   apiDescType,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
				Validators:    []validator.String{
					// TODO: re-enable once instance types are stabilized
					// validators.RegexValidator{RegexPattern: "^a40\\.(1|2|4|8)x|a100\\.(1|2|4|8)x|a100\\.(1|2|4|8)x|a100-80gb\\.(1|2|4|8)x$"},
				},
			},
			"ssh_key": schema.StringAttribute{
				Required:      true,
				Description:   apiDescSSHKey,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
				Validators:    []validator.String{validators.SSHKeyValidator{}},
			},
			"location": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   apiDescLocation,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"image": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   apiDescImage,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"startup_script": schema.StringAttribute{
				Optional:      true,
				Description:   apiDescStartupScript,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"shutdown_script": schema.StringAttribute{
				Optional:      true,
				Description:   apiDescShutdownScript,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"subnet": schema.StringAttribute{
				Required:      true,
				Description:   apiDescSubnet,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			// ib_partition and transport_partition_id name the same partition, so renaming
			// one to the other is not a change to the template and must not recreate it.
			// A change to the partition itself still forces replacement.
			"ib_partition": schema.StringAttribute{
				Optional:           true,
				PlanModifiers:      []planmodifier.String{aliasPairReplaceIfModifier()},
				DeprecationMessage: providerDescIBPartitionDeprecated,
				Description:        providerDescIBPartitionDeprecated,
			},
			"transport_partition_id": schema.StringAttribute{
				Optional:      true,
				Description:   apiDescTransportPartitionID,
				PlanModifiers: []planmodifier.String{aliasPairReplaceIfModifier()},
			},
			"public_ip_address_type": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   apiDescPublicIPAddressType,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"disks": schema.SetNestedAttribute{
				Optional:    true,
				Description: apiDescDisks,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"size": schema.StringAttribute{
							Required:    true,
							Description: apiDescDiskSize,
							Validators:  []validator.String{validators.StorageSizeValidator{}},
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(), // cannot be updated in place
							},
						},
						"type": schema.StringAttribute{
							Required:    true,
							Description: apiDescDiskType,
							Validators:  []validator.String{stringvalidator.OneOf(persistentSSD, sharedVolume)},
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(), // cannot be updated in place
							},
						},
					},
				},
			},
			"shared_volume_attachments": schema.SetAttribute{
				Optional:      true,
				ElementType:   types.StringType,
				Description:   apiDescSharedVolumeAttachments,
				PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			// reservation_id and nvlink_domain_id are set only by the user: the backend
			// never supplies or normalizes either one, so both are null in state whenever
			// the configuration omits them. UseStateForUnknown does nothing with a null
			// prior value, which would leave the planned value unknown and make
			// RequiresReplace recreate the template on every plan.
			"reservation_id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					// Order matters. Modifiers run in slice order, the planned value is
					// threaded from one to the next, and RequiresReplace cannot be unset
					// once it fires. Resolving the unknown has to come first, or
					// RequiresReplace compares an unknown against the prior value.
					common.UseStateForUnknownIncludingNull(),
					stringplanmodifier.RequiresReplace(), // cannot be updated in place
				},
				Description: providerDescReservationID,
			},
			"placement_policy": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   apiDescPlacementPolicy,
				Default:       stringdefault.StaticString("unspecified"),
				Validators:    []validator.String{stringvalidator.OneOf(spreadPlacementPolicy, unspecifiedPlacementPolicy)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"nvlink_domain_id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: apiDescNvlinkDomainID,
				PlanModifiers: []planmodifier.String{
					// Resolving the unknown must precede RequiresReplace; see reservation_id.
					common.UseStateForUnknownIncludingNull(),
					stringplanmodifier.RequiresReplace(), // cannot be updated in place
				},
			},
		},
	}
}

// ValidateConfig runs at plan time and surfaces config-only violations before any API call.
//
//nolint:gocritic // Implements Terraform defined interface
func (r *instanceTemplateResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config instanceTemplateResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validateAliasPairConfig(config.IBPartition, config.TransportPartitionID, &resp.Diagnostics)
}

func (r *instanceTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceID, projectID, errMsg := common.ParseResourceIdentifiers(req, r.client, "instance_template_id")
	if errMsg != "" {
		resp.Diagnostics.AddError("Failed to import Instance Template", errMsg)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), resourceID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan instanceTemplateResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())

	var disksToCreate []swagger.DiskTemplate
	if !plan.DisksToCreate.IsNull() && !plan.DisksToCreate.IsUnknown() {
		tDisks := make([]diskToCreateResourceModel, 0, len(plan.DisksToCreate.Elements()))
		diags = plan.DisksToCreate.ElementsAs(ctx, &tDisks, true)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		disksToCreate = make([]swagger.DiskTemplate, 0, len(tDisks))
		for _, disk := range tDisks {
			disksToCreate = append(disksToCreate, swagger.DiskTemplate{
				Size:  disk.Size.ValueString(),
				Type_: disk.Type.ValueString(),
			})
		}
	}

	var sharedVolumes []string
	if !plan.SharedVolumes.IsNull() && !plan.SharedVolumes.IsUnknown() {
		diags = plan.SharedVolumes.ElementsAs(ctx, &sharedVolumes, true)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	dataResp, httpResp, err := r.client.APIClient.InstanceTemplatesApi.CreateInstanceTemplate(ctx, swagger.InstanceTemplatePostRequestV1{
		TemplateName:            plan.Name.ValueString(),
		Type_:                   plan.Type.ValueString(),
		Location:                plan.Location.ValueString(),
		ImageName:               plan.Image.ValueString(),
		SshPublicKey:            plan.SSHKey.ValueString(),
		StartupScript:           plan.StartupScript.ValueString(),
		ShutdownScript:          plan.ShutdownScript.ValueString(),
		SubnetId:                plan.Subnet.ValueString(),
		IbPartitionId:           plan.IBPartition.ValueString(),
		TransportPartitionId:    plan.TransportPartitionID.ValueString(),
		Disks:                   disksToCreate,
		SharedVolumeAttachments: sharedVolumes,
		PublicIpAddressType:     plan.PublicIpAddressType.ValueString(),
		PlacementPolicy:         plan.PlacementPolicy.ValueString(),
		NvlinkDomainId:          plan.NvlinkDomainID.ValueString(),
	}, projectID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to create instance template",
			fmt.Sprintf("There was an error creating the instance template (project %s): %s", projectID, common.UnpackAPIError(err)))

		return
	}

	instanceTemplateToResourceModel(ctx, &dataResp, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// reservation_id is deprecated and plan-owned, so it is handled here, not in the
	// shared transform: prefer the API value, warn if a requested reservation was
	// ignored, otherwise null it.
	if dataResp.ReservationId != "" {
		plan.ReservationID = types.StringValue(dataResp.ReservationId)
	} else if !plan.ReservationID.IsNull() && !plan.ReservationID.IsUnknown() && plan.ReservationID.ValueString() != "" {
		resp.Diagnostics.AddWarning("Reservation Assignment Deprecated",
			"Reservation assignment during instance template creation is deprecated. The requested reservation_id was ignored by the backend. Please remove reservation_id from your configuration to suppress this warning.")
	} else {
		plan.ReservationID = types.StringNull()
	}

	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceTemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state instanceTemplateResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	instanceTemplate, httpResp, err := r.client.APIClient.InstanceTemplatesApi.GetInstanceTemplate(ctx, state.ID.ValueString(), state.ProjectID.ValueString())
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to get instance template",
			fmt.Sprintf("Fetching Crusoe instance templates failed: %s\n\nIf the problem persists, contact support@crusoecloud.com", common.UnpackAPIError(err)))

		return
	}

	if httpResp.StatusCode == http.StatusNotFound {
		// instance template has most likely been deleted out of band, so we update Terraform state to match
		resp.State.RemoveResource(ctx)

		return
	}

	instanceTemplateToResourceModel(ctx, &instanceTemplate, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	hydrateAliasPairForImport(&state, &instanceTemplate)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// Update handles the only change an instance template permits in place: swapping the
// deprecated ib_partition attribute for its replacement transport_partition_id. Both names
// address the same partition, so the backend object does not change and this method makes
// no API call. It records the planned attribute names in state.
//
// Any other change is rejected, as it always was. aliasOnlyChange verifies that rather than
// trusting the schema's replace-forcing plan modifiers, so an attribute that does not force
// replacement degrades to the error below instead of silently corrupting state.
//
//nolint:gocritic // Implements Terraform defined interface
func (r *instanceTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	aliasOnly, err := aliasOnlyChange(ctx, &req.Plan, &req.State, &resp.Diagnostics)
	if err != nil {
		return
	}

	if !aliasOnly {
		resp.Diagnostics.AddError("Failed to update instance template",
			"Instance templates are immutable and cannot be updated. Please delete and recreate the resource instead.")

		return
	}

	var plan instanceTemplateResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The plan is written verbatim. It holds no unknown values, because id and every
	// Optional+Computed attribute carry UseStateForUnknown. The write has to be explicit:
	// the framework prepopulates resp.State with the prior state, so returning without
	// setting it would drop the rename.
	diags = resp.State.Set(ctx, &plan)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state instanceTemplateResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.APIClient.InstanceTemplatesApi.DeleteInstanceTemplate(ctx, state.ID.ValueString(), state.ProjectID.ValueString())
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete instance template",
			fmt.Sprintf("There was an error starting a delete instance template operation: %s", common.UnpackAPIError(err)))

		return
	}
}

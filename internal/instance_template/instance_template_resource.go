package instance_template

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	validators "github.com/crusoecloud/terraform-provider-crusoe/internal/validators"
)

type instanceTemplateResource struct {
	client *swagger.APIClient
}

type instanceTemplateResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	ProjectID           types.String `tfsdk:"project_id"`
	Name                types.String `tfsdk:"name"`
	Type                types.String `tfsdk:"type"`
	SSHKey              types.String `tfsdk:"ssh_key"`
	Location            types.String `tfsdk:"location"`
	Image               types.String `tfsdk:"image"`
	StartupScript       types.String `tfsdk:"startup_script"`
	ShutdownScript      types.String `tfsdk:"shutdown_script"`
	Subnet              types.String `tfsdk:"subnet"`
	IBPartition         types.String `tfsdk:"ib_partition"`
	PublicIpAddressType types.String `tfsdk:"public_ip_address_type"`
	DisksToCreate       types.List   `tfsdk:"disks_to_create"`
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

	client, ok := req.ProviderData.(*swagger.APIClient)
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
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place
			},
			"startup_script": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"shutdown_script": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"subnet": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"ib_partition": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"disks": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"size": schema.StringAttribute{
							Required:   true,
							Validators: []validator.String{validators.StorageSizeValidator{}},
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),    // cannot be updated in place
								stringplanmodifier.UseStateForUnknown(), // maintain across updates if not explicitly changed
							},
						},
						"type": schema.StringAttribute{
							Optional: true,
							Computed: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),    // cannot be updated in place
								stringplanmodifier.UseStateForUnknown(), // maintain across updates if not explicitly changed
							},
						},
					},
				},
			},
		},
	}
}

func (r *instanceTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan instanceTemplateResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := ""
	if plan.ProjectID.ValueString() == "" {
		project, err := common.GetFallbackProject(ctx, r.client, &resp.Diagnostics)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create instance template",
				fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

			return
		}
		projectID = project
	} else {
		projectID = plan.ProjectID.ValueString()
	}

	tDisks := make([]diskToCreateResourceModel, 0, len(plan.DisksToCreate.Elements()))
	diags = plan.DisksToCreate.ElementsAs(ctx, &tDisks, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diskToCreates := make([]swagger.DiskTemplate, 0, len(tDisks))
	for _, disk := range tDisks {
		diskToCreates = append(diskToCreates, swagger.DiskTemplate{
			Size:  disk.Size.ValueString(),
			Type_: disk.Type.ValueString(),
		})
	}

	dataResp, httpResp, err := r.client.InstanceTemplatesApi.CreateInstanceTemplate(ctx, swagger.InstanceTemplatePostRequestV1Alpha5{
		TemplateName:        plan.Name.ValueString(),
		Type_:               plan.Type.ValueString(),
		Location:            plan.Location.ValueString(),
		ImageName:           plan.Image.ValueString(),
		SshPublicKey:        plan.SSHKey.ValueString(),
		StartupScript:       plan.StartupScript.ValueString(),
		ShutdownScript:      plan.ShutdownScript.ValueString(),
		SubnetId:            plan.Subnet.ValueString(),
		IbPartitionId:       plan.IBPartition.ValueString(),
		Disks:               diskToCreates,
		PublicIpAddressType: plan.PublicIpAddressType.ValueString(),
	}, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create instance template",
			fmt.Sprintf("There was an error creating the instance template (project %s): %s", projectID, common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	plan.ID = types.StringValue(dataResp.Id)
	plan.ProjectID = types.StringValue(projectID)

	diags = resp.State.Set(ctx, plan)
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

	instanceTemplate, httpResp, err := r.client.InstanceTemplatesApi.GetInstanceTemplate(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get instance template",
			fmt.Sprintf("Fetching Crusoe instance templates failed: %s\n\nIf the problem persists, contact support@crusoecloud.com", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		// instance template has most likely been deleted out of band, so we update Terraform state to match
		resp.State.RemoveResource(ctx)

		return
	}

	disks := make([]diskToCreateResourceModel, 0, len(instanceTemplate.Disks))
	for i := range instanceTemplate.Disks {
		disk := instanceTemplate.Disks[i]
		disks = append(disks, diskToCreateResourceModel{
			Size: types.StringValue(disk.Size),
			Type: types.StringValue(disk.Type_),
		})
	}
	if len(disks) > 0 {
		tDisks, _ := types.ListValueFrom(context.Background(), diskToCreateSchema, disks)
		state.DisksToCreate = tDisks
	} else {
		state.DisksToCreate = types.ListNull(diskToCreateSchema)
	}

	state.Name = types.StringValue(instanceTemplate.Name)
	state.Location = types.StringValue(instanceTemplate.Location)
	state.Type = types.StringValue(instanceTemplate.Type_)
	state.Image = types.StringValue(instanceTemplate.ImageName)
	state.SSHKey = types.StringValue(instanceTemplate.SshPublicKey)
	state.StartupScript = types.StringValue(instanceTemplate.StartupScript)
	state.ShutdownScript = types.StringValue(instanceTemplate.ShutdownScript)
	state.Subnet = types.StringValue(instanceTemplate.SubnetId)
	state.IBPartition = types.StringValue(instanceTemplate.IbPartitionId)
	state.ProjectID = types.StringValue(instanceTemplate.ProjectId)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Failed to update instance template",
		"Instance templates are immutable and cannot be updated. Please delete and recreate the resource instead.")
}

//nolint:gocritic // Implements Terraform defined interface
func (r *instanceTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state instanceTemplateResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.InstanceTemplatesApi.DeleteInstanceTemplate(ctx, state.ID.ValueString(), state.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete instance template",
			fmt.Sprintf("There was an error starting a delete instance template operation: %s", common.UnpackAPIError(err)))

		return
	}
	defer httpResp.Body.Close()
}

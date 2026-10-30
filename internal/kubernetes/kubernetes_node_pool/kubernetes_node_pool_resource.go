package kubernetes_node_pool

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                   = &kubernetesNodePoolResource{}
	_ resource.ResourceWithValidateConfig = &kubernetesNodePoolResource{}
)

type kubernetesNodePoolResource struct {
	client *common.CrusoeClient
}

func NewKubernetesNodePoolResource() resource.Resource {
	return &kubernetesNodePoolResource{}
}

type kubernetesNodePoolResourceModel struct {
	ID                            types.String      `tfsdk:"id"`
	ProjectID                     types.String      `tfsdk:"project_id"`
	Version                       common.K8sVersion `tfsdk:"version"`
	Type                          types.String      `tfsdk:"type"`
	InstanceCount                 types.Int64       `tfsdk:"instance_count"`
	ClusterID                     types.String      `tfsdk:"cluster_id"`
	SubnetID                      types.String      `tfsdk:"subnet_id"`
	IBPartitionID                 types.String      `tfsdk:"ib_partition_id"`
	TransportPartitionID          types.String      `tfsdk:"transport_partition_id"`
	RequestedNodeLabels           types.Map         `tfsdk:"requested_node_labels"`
	AllNodeLabels                 types.Map         `tfsdk:"all_node_labels"`
	NodeTaints                    types.Set         `tfsdk:"node_taints"`
	InstanceIDs                   types.List        `tfsdk:"instance_ids"`
	SSHKey                        types.String      `tfsdk:"ssh_key"`
	State                         types.String      `tfsdk:"state"`
	Name                          types.String      `tfsdk:"name"`
	EphemeralStorageForContainerd types.Bool        `tfsdk:"ephemeral_storage_for_containerd"`
	BatchSize                     types.Int64       `tfsdk:"batch_size"`
	BatchPercentage               types.Int64       `tfsdk:"batch_percentage"`
	NvlinkDomainID                types.String      `tfsdk:"nvlink_domain_id"`
	PublicIPType                  types.String      `tfsdk:"public_ip_type"`
	ConsentMode                   types.String      `tfsdk:"consent_mode"`
	UpdateSettings                types.Object      `tfsdk:"update_settings"`
	Health                        types.Object      `tfsdk:"health"`
	Current                       types.Int64       `tfsdk:"current"`
	NodePoolGeneration            types.Int64       `tfsdk:"node_pool_generation"`
	AutoscalingConfig             types.Object      `tfsdk:"autoscaling_config"`
}

type autoscalingConfigModel struct {
	Enabled     types.Bool  `tfsdk:"enabled"`
	MinNodeSize types.Int64 `tfsdk:"min_node_size"`
	MaxNodeSize types.Int64 `tfsdk:"max_node_size"`
}

var autoscalingConfigAttrTypes = map[string]attr.Type{
	"enabled":       types.BoolType,
	"min_node_size": types.Int64Type,
	"max_node_size": types.Int64Type,
}

func (r *kubernetesNodePoolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *kubernetesNodePoolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_node_pool"
}

// ValidateConfig runs at plan time and surfaces config-only violations
// (no remote state needed) before any API call. Schema validators handle
// per-field shape; this catches list-level rules like duplicate taints.
func (r *kubernetesNodePoolResource) ValidateConfig(
	ctx context.Context,
	req resource.ValidateConfigRequest,
	resp *resource.ValidateConfigResponse,
) {
	var data kubernetesNodePoolResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validateAliasPairConfig(data.IBPartitionID, data.TransportPartitionID, &resp.Diagnostics)

	nodeTaints, err := tfSetToNodeTaints(ctx, data.NodeTaints)
	if err != nil {
		// Unresolved/unknown values; let plan/apply re-evaluate.
		return
	}
	if vErr := validateNodeTaintDuplicates(nodeTaints); vErr != nil {
		resp.Diagnostics.AddError("Invalid node_taints", vErr.Error())
	}

	r.validateAutoscalingConfig(ctx, data.AutoscalingConfig, resp)
}

// validateAutoscalingConfig enforces the cross-field rules of the block, mirroring
// the server's write modes: enabling is a full replace requiring both bounds;
// disabling is flag-only, so bounds must be omitted (the API ignores them and
// retains the stored bounds — leaving them in config would cause a permanent
// diff against the refreshed state).
func (r *kubernetesNodePoolResource) validateAutoscalingConfig(
	ctx context.Context,
	scalingConfig types.Object,
	resp *resource.ValidateConfigResponse,
) {
	if scalingConfig.IsNull() || scalingConfig.IsUnknown() {
		return
	}

	var sc autoscalingConfigModel
	resp.Diagnostics.Append(scalingConfig.As(ctx, &sc, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() || sc.Enabled.IsUnknown() {
		return
	}

	// enabled is schema-Optional only to keep block removal plannable (see
	// the schema comment); a present block must state it.
	if sc.Enabled.IsNull() {
		resp.Diagnostics.AddError("Invalid autoscaling_config",
			"enabled must be set when the autoscaling_config block is present")

		return
	}

	if !sc.Enabled.ValueBool() {
		if !sc.MinNodeSize.IsNull() || !sc.MaxNodeSize.IsNull() {
			resp.Diagnostics.AddError("Invalid autoscaling_config",
				"omit min_node_size and max_node_size when disabling autoscaling; "+
					"the stored bounds are retained — re-enable with new bounds to change them")
		}

		return
	}

	if sc.MinNodeSize.IsNull() || sc.MaxNodeSize.IsNull() {
		resp.Diagnostics.AddError("Invalid autoscaling_config",
			"enabling autoscaling requires both min_node_size and max_node_size")

		return
	}
	if !sc.MinNodeSize.IsUnknown() && !sc.MaxNodeSize.IsUnknown() &&
		sc.MinNodeSize.ValueInt64() > sc.MaxNodeSize.ValueInt64() {

		resp.Diagnostics.AddError("Invalid autoscaling_config",
			fmt.Sprintf("min_node_size (%d) must be <= max_node_size (%d)",
				sc.MinNodeSize.ValueInt64(), sc.MaxNodeSize.ValueInt64()))
	}
}

// validateAutoscalingUpdateCount catches at plan time an update the API would
// reject: while autoscaling is enabled, the count must lie within the bounds,
// and every update sends instance_count (the generated client's count has no
// omitempty). count is the planned instance_count, which under
// ignore_changes is the state value, i.e. the autoscaler's last count. Unknown
// values skip the check; the API still enforces it at apply.
func validateAutoscalingUpdateCount(ctx context.Context, count types.Int64, scalingConfig types.Object) diag.Diagnostics {
	if count.IsNull() || count.IsUnknown() || scalingConfig.IsNull() || scalingConfig.IsUnknown() {
		return nil
	}

	var sc autoscalingConfigModel
	if diags := scalingConfig.As(ctx, &sc, basetypes.ObjectAsOptions{}); diags.HasError() {
		return diags
	}
	if sc.Enabled.IsNull() || sc.Enabled.IsUnknown() || !sc.Enabled.ValueBool() ||
		sc.MinNodeSize.IsNull() || sc.MinNodeSize.IsUnknown() ||
		sc.MaxNodeSize.IsNull() || sc.MaxNodeSize.IsUnknown() {

		return nil
	}

	n, minSize, maxSize := count.ValueInt64(), sc.MinNodeSize.ValueInt64(), sc.MaxNodeSize.ValueInt64()
	if n >= minSize && n <= maxSize {
		return nil
	}

	var diags diag.Diagnostics
	diags.AddAttributeError(path.Root("instance_count"),
		"instance_count outside autoscaling bounds",
		fmt.Sprintf("This update would send instance_count = %d, outside the autoscaling bounds "+
			"[min_node_size = %d, max_node_size = %d], and the API rejects a count outside the bounds while "+
			"autoscaling is enabled. Every update sends instance_count; under "+
			"lifecycle { ignore_changes = [instance_count] } the value sent is the node pool's current count. "+
			"Set instance_count within the bounds in the same apply, temporarily removing it from "+
			"ignore_changes if needed.", n, minSize, maxSize))

	return diags
}

func (r *kubernetesNodePoolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: apiDescID,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"project_id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: providerDescProjectID,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplaceIfConfigured()}, // cannot be updated in place by user
			},
			"version": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				CustomType:          common.K8sVersionType{},
				MarkdownDescription: apiDescVersion,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
				Validators: []validator.String{stringvalidator.RegexMatches(
					regexp.MustCompile(`\d+\.\d+\.\d+-cmk\.\d+.*`), "must be in the format MAJOR.MINOR.BUGFIX-cmk.NUM (e.g 1.2.3-cmk.4)",
				)},
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescType,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"instance_count": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: apiDescInstanceCount + " " + providerDescAutoscalingOwnsCount,
			},
			"cluster_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescClusterID,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"subnet_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: apiDescSubnetID,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplaceIfConfigured()}, // cannot be updated in place by user
			},
			// ib_partition_id and transport_partition_id name the same partition, so renaming
			// one to the other is not a change to the node pool and must not recreate it.
			// A change to the partition itself still forces replacement.
			"ib_partition_id": schema.StringAttribute{
				Optional:           true,
				PlanModifiers:      []planmodifier.String{aliasPairReplaceIfModifier()},
				DeprecationMessage: providerDescIBPartitionIDDeprecated,
				Description:        providerDescIBPartitionIDDeprecated,
			},
			"transport_partition_id": schema.StringAttribute{
				Optional:      true,
				Description:   apiDescTransportPartitionID,
				PlanModifiers: []planmodifier.String{aliasPairReplaceIfModifier()},
			},
			"requested_node_labels": schema.MapAttribute{
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				MarkdownDescription: apiDescRequestedNodeLabels,
				PlanModifiers:       []planmodifier.Map{mapplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"all_node_labels": schema.MapAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: apiDescNodeLabels,
			},
			"instance_ids": schema.ListAttribute{
				ElementType:         types.StringType,
				Computed:            true,
				MarkdownDescription: apiDescInstanceIDs,
			},
			"ssh_key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescSSHKey,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"state": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: apiDescState,
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescName,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"ephemeral_storage_for_containerd": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: apiDescEphemeralStorageForContainerd,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"batch_size": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: providerDescBatchSize,
				Validators: []validator.Int64{
					int64validator.Between(1, 10),
				},
				PlanModifiers: []planmodifier.Int64{
					common.NewDevelopmentWarningInt64Modifier("", "Node pool rollout is currently in development. "+common.DevelopmentSupportMessage),
				},
			},
			"batch_percentage": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: providerDescBatchPercentage,
				Validators: []validator.Int64{
					int64validator.Between(1, 100),
				},
				PlanModifiers: []planmodifier.Int64{
					common.NewDevelopmentWarningInt64Modifier("", "Node pool rollout is currently in development. "+common.DevelopmentSupportMessage),
				},
			},
			"nvlink_domain_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: apiDescNvlinkDomainID,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace(), stringplanmodifier.UseStateForUnknown()}, // cannot be updated in place & maintain across updates
			},
			"public_ip_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: apiDescPublicIPType,
				Default:             stringdefault.StaticString("dynamic"), // Default to dynamic
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					common.NewPrivateNodePoolsWarningModifier(),
				}, // maintain across updates
			},
			"consent_mode": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: apiDescConsentMode + " " + providerDescConsentModeDefault + " " +
					providerDescLimitedAvailability,
				Validators:    []validator.String{stringvalidator.OneOf(consentModeValues...)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"update_settings": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: apiDescUpdateSettings + " " + providerDescLimitedAvailability,
				PlanModifiers:       []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					// The default applies only when the block is present and
					// this field is not — it never materializes the block
					// itself. A node pool that does not support update settings
					// keeps a null object, which UseStateForUnknown carries
					// forward, so the two never contend.
					"allow_scale_down": schema.BoolAttribute{
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(false),
						MarkdownDescription: apiDescAllowScaleDown,
					},
				},
			},
			"health": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: apiDescHealth + " " + providerDescLimitedAvailability,
				Attributes: map[string]schema.Attribute{
					"issues": schema.ListNestedAttribute{
						Computed:            true,
						MarkdownDescription: apiDescHealthIssues,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"code":              schema.StringAttribute{Computed: true, MarkdownDescription: apiDescIssueCode},
								"message":           schema.StringAttribute{Computed: true, MarkdownDescription: apiDescIssueMessage},
								"since":             schema.StringAttribute{Computed: true, MarkdownDescription: apiDescIssueSince},
								"affected_count":    schema.Int64Attribute{Computed: true, MarkdownDescription: apiDescAffectedCount},
								"affected_node_ids": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: apiDescAffectedNodeIDs},
							},
						},
					},
				},
			},
			"current": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: apiDescCurrent + " " + providerDescLimitedAvailability,
			},
			// generation changes only when the platform migrates the pool, which
			// the Read before every plan picks up, so carrying it forward keeps it
			// from showing as "known after apply" on every update.
			"node_pool_generation": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: apiDescGeneration + " " + providerDescGeneration,
				PlanModifiers:       []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
		},
		Blocks: map[string]schema.Block{
			"autoscaling_config": schema.SingleNestedBlock{
				MarkdownDescription: apiDescAutoscalingConfig + " " + providerDescAutoscalingRequiresAutoscaler + " " +
					providerDescAutoscalingConfig,
				Attributes: map[string]schema.Attribute{
					// Optional, not Required: the framework validates a Required
					// attribute inside a SingleNestedBlock even when the whole
					// block is absent from config, which breaks block removal.
					// Presence is enforced in ValidateConfig instead.
					"enabled": schema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: apiDescAutoscalingEnabled + " " + providerDescAutoscalingEnabled,
					},
					// The bounds are deliberately NOT Computed: a Computed
					// attribute inside the block makes the framework plan a
					// config-removed block as a partially-unknown object
					// instead of null, which breaks removal-means-disable
					// with "Missing Configuration for Required Attribute".
					"min_node_size": schema.Int64Attribute{
						Optional:            true,
						MarkdownDescription: apiDescAutoscalingMinNodeSize + " " + providerDescAutoscalingBounds,
						Validators: []validator.Int64{
							int64validator.AtLeast(0),
						},
					},
					"max_node_size": schema.Int64Attribute{
						Optional:            true,
						MarkdownDescription: apiDescAutoscalingMaxNodeSize + " " + providerDescAutoscalingBounds,
						Validators: []validator.Int64{
							// 0, not 1: [0, 0] is legal and pins the pool at zero nodes.
							int64validator.AtLeast(0),
						},
					},
				},
			},
			"node_taints": schema.SetNestedBlock{
				MarkdownDescription: apiDescNodeTaints,
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: apiDescTaintKey,
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
						},
						"value": schema.StringAttribute{
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString(""),
							MarkdownDescription: apiDescTaintValue,
							Validators: []validator.String{
								stringvalidator.LengthAtMost(63),
							},
						},
						"effect": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: apiDescTaintEffect,
							Validators: []validator.String{
								stringvalidator.OneOf("NoSchedule", "PreferNoSchedule", "NoExecute"),
							},
						},
					},
				},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesNodePoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesNodePoolResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, plan.ProjectID.ValueString())

	var nodeLabels map[string]string
	if !plan.RequestedNodeLabels.IsNull() && !plan.RequestedNodeLabels.IsUnknown() {
		var err error
		nodeLabels, err = common.TFMapToStringMap(plan.RequestedNodeLabels)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create node pool", fmt.Sprintf("error when parsing requested_node_labels as string map: %s", err))

			return
		}
	}

	nodeTaints, err := tfSetToNodeTaints(ctx, plan.NodeTaints)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create node pool", fmt.Sprintf("error when parsing node taints: %s", err))

		return
	}

	updateSettings, settingsDiags := tfObjectToUpdateSettings(ctx, plan.UpdateSettings)
	resp.Diagnostics.Append(settingsDiags...)
	scalingConfig, scDiags := tfObjectToAutoscalingConfig(ctx, plan.AutoscalingConfig)
	resp.Diagnostics.Append(scDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	asyncOperation, _, err := r.client.APIClient.KubernetesNodePoolsApi.CreateNodePool(ctx, swagger.KubernetesNodePoolPostRequest{
		ClusterId:                     plan.ClusterID.ValueString(),
		Count:                         plan.InstanceCount.ValueInt64(),
		IbPartitionId:                 plan.IBPartitionID.ValueString(), //nolint:staticcheck // SA1019: deprecated ib_partition_id is still accepted for backwards compatibility
		TransportPartitionId:          plan.TransportPartitionID.ValueString(),
		Name:                          plan.Name.ValueString(),
		NodeLabels:                    nodeLabels,
		NodeTaints:                    nodeTaints,
		NodePoolVersion:               plan.Version.ValueString(),
		ProductName:                   plan.Type.ValueString(),
		SshPublicKey:                  plan.SSHKey.ValueString(),
		SubnetId:                      plan.SubnetID.ValueString(),
		EphemeralStorageForContainerd: plan.EphemeralStorageForContainerd.ValueBool(),
		NvlinkDomainId:                plan.NvlinkDomainID.ValueString(),
		PublicIpType:                  plan.PublicIPType.ValueString(),
		// Both have omitempty, so an unset value is genuinely omitted and the
		// backend keeps its own default rather than being told "".
		ConsentMode:       plan.ConsentMode.ValueString(),
		UpdateSettings:    updateSettings,
		AutoscalingConfig: scalingConfig,
	}, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create node pool",
			fmt.Sprintf("Error starting a create node pool operation: %s", common.UnpackAPIError(err)))

		return
	}

	// Wait for operation to complete
	kubernetesNodePoolResponse, err := AwaitNodePoolOperation(ctx, asyncOperation.Operation, projectID, r.client.APIClient)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create node pool",
			fmt.Sprintf("Error creating a node pool: %s", common.UnpackAPIError(err)))

		return
	}

	if kubernetesNodePoolResponse.Details != nil && kubernetesNodePoolResponse.Details.Error_ != "" {
		// TODO: Return created count once NumVmsCreated is populated
		resp.Diagnostics.AddWarning("Unable to create desired number of VMs",
			fmt.Sprintf("Warning -- Unable to create desired number of VMs due to the following error: %s",
				kubernetesNodePoolResponse.Details.Error_))
	}

	// Re-read the node pool, as Update does. The operation result describes the
	// pool as the create built it and carries none of the fields derived at read
	// time — health and current among them — so mapping straight from it would
	// report a pool that came up short as healthy until the next refresh.
	//
	// Best-effort: the pool exists either way, and failing here would leave it
	// unmanaged and absent from state. Fall back to the operation result, whose
	// only gap is the derived fields.
	createdNodePool := kubernetesNodePoolResponse.NodePool
	refreshed, httpResp, refreshErr := r.client.APIClient.KubernetesNodePoolsApi.GetNodePool(
		ctx, projectID, createdNodePool.Id)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	switch {
	case refreshErr != nil:
		resp.Diagnostics.AddWarning("Could not read the node pool after creating it",
			fmt.Sprintf("The node pool was created, but reading it back failed: %s\n\nDerived values such as "+
				"health and current are not yet known and will be populated on the next refresh.",
				common.UnpackAPIError(refreshErr)))
	default:
		createdNodePool = &refreshed
	}

	var state kubernetesNodePoolResourceModel
	nodePoolToResourceModel(ctx, createdNodePool, &plan, &state, &resp.Diagnostics)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesNodePoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var stored kubernetesNodePoolResourceModel

	diags := req.State.Get(ctx, &stored)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, stored.ProjectID.ValueString())

	kubernetesNodePool, httpResp, err := r.client.APIClient.KubernetesNodePoolsApi.GetNodePool(ctx, projectID, stored.ID.ValueString())
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to get Kubernetes Node Pool", fmt.Sprintf("Failed to get node pool: %s.",
			common.UnpackAPIError(err)))

		return
	}

	var state kubernetesNodePoolResourceModel
	nodePoolToResourceModel(ctx, &kubernetesNodePool, &stored, &state, &resp.Diagnostics)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

// Handle validation at the resource level to prevent duplicate errors/warnings
// nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesNodePoolResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Only check during updates (skip creates and deletes)
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var plan kubernetesNodePoolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state kubernetesNodePoolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A plan equal to state sends no update, so only a planned change is
	// checked; an out-of-bounds count with nothing to apply is left alone.
	if !req.Plan.Raw.Equal(req.State.Raw) {
		resp.Diagnostics.Append(validateAutoscalingUpdateCount(ctx, plan.InstanceCount, plan.AutoscalingConfig)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Warn if instance count is decreasing. What actually happens depends on
	// update_settings.allow_scale_down, so the two outcomes are described
	// separately: with it off — the default, and the only behavior available on
	// a cluster that does not support the setting — the API records the new
	// target and reports a health
	// issue without removing anything, which is the case the single old message
	// described. With it on, the update drains and deletes nodes, which is
	// destructive and worth saying plainly.
	if !plan.InstanceCount.IsNull() && !state.InstanceCount.IsNull() &&
		plan.InstanceCount.ValueInt64() < state.InstanceCount.ValueInt64() {

		if planAllowsScaleDown(ctx, plan.UpdateSettings) {
			resp.Diagnostics.AddAttributeWarning(
				path.Root("instance_count"),
				"Node pool instance count decreased",
				"allow_scale_down is enabled, so this update will drain and delete existing nodes to reach "+
					"the new count. Workloads on those nodes will be evicted.",
			)
		} else {
			resp.Diagnostics.AddAttributeWarning(
				path.Root("instance_count"),
				"Node pool instance count decreased",
				"Decreasing the count records the new target but does not delete existing nodes, and the "+
					"node pool reports a health issue while it has more nodes than desired. To let updates "+
					"remove nodes, set update_settings.allow_scale_down where it is supported. Otherwise "+
					"delete the nodes manually.",
			)
		}
	}

	// Check for mutual exclusivity
	if !plan.BatchSize.IsNull() && !plan.BatchPercentage.IsNull() {
		resp.Diagnostics.AddError(
			"Conflicting Configuration",
			"batch_size and batch_percentage are mutually exclusive. "+
				"Please specify only one.",
		)

		return
	}

	needsRollout := ModifyPlanNodePoolNeedsRollout(ctx, &req, resp)

	if !needsRollout {
		return
	}

	// Check if both batch_size and batch_percentage are null
	if plan.BatchSize.IsNull() && plan.BatchPercentage.IsNull() {
		resp.Diagnostics.AddWarning(
			"Existing nodes will not be updated",
			"Existing nodes will not be updated to the new configuration, "+
				"but new nodes will use the new configuration.",
		)

		return
	}

	// Build warning message based on which parameter is set
	warningMsg := "Using batch_size or batch_percentage during updates will cause nodes to be deleted and recreated in batches. " +
		"This will result in temporary capacity reduction during the rollout process. "

	if !plan.BatchPercentage.IsNull() {
		warningMsg += "The calculated number of nodes based on the percentage cannot exceed 10 nodes per batch. "
	}

	resp.Diagnostics.AddWarning(
		"Batch update will delete and recreate nodes",
		warningMsg,
	)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesNodePoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan kubernetesNodePoolResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var stored kubernetesNodePoolResourceModel
	diags = req.State.Get(ctx, &stored)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, stored.ProjectID.ValueString())

	var nodeLabels map[string]string
	if !plan.RequestedNodeLabels.IsNull() && !plan.RequestedNodeLabels.IsUnknown() {
		var err error
		nodeLabels, err = common.TFMapToStringMap(plan.RequestedNodeLabels)
		if err != nil {
			resp.Diagnostics.AddError("Failed to update node pool", fmt.Sprintf("error when parsing requested_node_labels as string map: %s", err))

			return
		}
	}

	nodeTaints, err := tfSetToNodeTaints(ctx, plan.NodeTaints)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update node pool", fmt.Sprintf("error when parsing node_taints: %s", err))

		return
	}

	// If node_taints block is removed from config but state has taints, send empty list to clear all taints.
	if nodeTaints == nil && !stored.NodeTaints.IsNull() && len(stored.NodeTaints.Elements()) > 0 {
		nodeTaints = []swagger.KubernetesNodeTaint{}
	}

	planUpdateSettings, settingsDiags := tfObjectToUpdateSettings(ctx, plan.UpdateSettings)
	resp.Diagnostics.Append(settingsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	// A removed autoscaling_config block sends an explicit disable; see
	// autoscalingUpdateConfig.
	scalingConfig, scDiags := autoscalingUpdateConfig(ctx, plan.AutoscalingConfig, stored.AutoscalingConfig)
	resp.Diagnostics.Append(scDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	patchRequest := swagger.KubernetesNodePoolPatchRequest{
		Count:                         plan.InstanceCount.ValueInt64(),
		NodeLabels:                    nodeLabels,
		NodeTaints:                    nodeTaints,
		EphemeralStorageForContainerd: plan.EphemeralStorageForContainerd.ValueBool(),
		NodePoolVersion:               plan.Version.ValueString(),
		// ssh_public_key has no omitempty in the generated client, so leaving
		// this unset does not omit it — it sends the empty string, which the
		// backend reads as a request to change the key. The node pool v2
		// backend then rebuilds the pool's instance template with NO key, so
		// replacement nodes come up without the customer's key; v1 happens to
		// guard the empty string but still churns the template, marking every
		// existing node as not using the latest config.
		//
		// ssh_key is Required and RequiresReplace, so the plan always carries
		// the pool's standing key and the backend's diff is a no-op.
		SshPublicKey: plan.SSHKey.ValueString(),
		// Unlike the fields above, these two have omitempty: an unset value is
		// omitted and the API leaves the stored setting alone. update_settings
		// carries its presence on the pointer, because allow_scale_down's own
		// omitempty means a present block holding false marshals as {} — which
		// the API still reads as an assertion to turn scale-down off.
		ConsentMode:       plan.ConsentMode.ValueString(),
		UpdateSettings:    planUpdateSettings,
		AutoscalingConfig: scalingConfig,
	}

	updateAsyncOperation, _, err := r.client.APIClient.KubernetesNodePoolsApi.UpdateNodePool(
		ctx,
		patchRequest,
		projectID,
		stored.ID.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update node pool",
			fmt.Sprintf("Error starting an update node pool operation: %s", common.UnpackAPIError(err)))

		return
	}

	// Wait for update operation to complete
	kubernetesNodePoolResponse, err := AwaitNodePoolOperation(ctx, updateAsyncOperation.Operation, projectID, r.client.APIClient)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update node pool",
			fmt.Sprintf("Error updating a node pool: %s", common.UnpackAPIError(err)))

		return
	}

	if kubernetesNodePoolResponse.Details != nil && kubernetesNodePoolResponse.Details.Error_ != "" {
		// TODO: Return created count once NumVmsCreated is populated
		resp.Diagnostics.AddWarning("Unable to provision all instances",
			fmt.Sprintf("Warning -- Unable to create desired number of VMs due to the following error: %s",
				kubernetesNodePoolResponse.Details.Error_))
	}

	// Detect if changes require node rollout
	needsRollout := UpdateNodePoolNeedsRollout(ctx, &req, resp)

	// run rollout if batch_size or batch_percentage is specified
	batchSizeSet := !plan.BatchSize.IsNull() && plan.BatchSize.ValueInt64() > 0
	batchPercentageSet := !plan.BatchPercentage.IsNull() && plan.BatchPercentage.ValueInt64() > 0

	if needsRollout && (batchSizeSet || batchPercentageSet) {
		rolloutRequest := swagger.KubernetesNodePoolRotateStartRequest{}
		if batchSizeSet {
			rolloutRequest.Count = plan.BatchSize.ValueInt64()
			rolloutRequest.Strategy = "COUNT"
		} else {
			rolloutRequest.Percentage = plan.BatchPercentage.ValueInt64()
			rolloutRequest.Strategy = "PERCENTAGE"
		}

		_, _, rolloutAsyncOperationErr := r.client.APIClient.KubernetesNodePoolsApi.RotateNodePool(
			ctx,
			rolloutRequest,
			projectID,
			stored.ID.ValueString(),
		)
		if rolloutAsyncOperationErr != nil {
			// A warning, not an error: the node pool's own update already
			// succeeded above, so failing the apply here reports a change that
			// did happen as a change that did not, and leaves the customer with
			// an apply that fails on every run while batch_size or
			// batch_percentage stays in the configuration. Rollout is not
			// available for every node pool — it is feature-gated, and the node
			// pool v2 backend does not implement it at all — so an unavailable
			// rollout must not block managing the pool.
			resp.Diagnostics.AddWarning("Nodes were not rolled out",
				fmt.Sprintf("The node pool's configuration was updated, but rolling out the change to "+
					"existing nodes failed: %s\n\nNew nodes will use the new configuration. Existing nodes "+
					"keep their current configuration until they are replaced.",
					common.UnpackAPIError(rolloutAsyncOperationErr)))
		} else {
			resp.Diagnostics.AddWarning(
				"Rollout initiated",
				"A new rollout has been initiated to replace nodes with the latest config.",
			)
		}
	}

	// the async operation is returning the previous version of the node pool. query for the latest node pool.
	updatedNodePool, httpResp, err := r.client.APIClient.KubernetesNodePoolsApi.GetNodePool(ctx, projectID, kubernetesNodePoolResponse.NodePool.Id)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			resp.State.RemoveResource(ctx)

			return
		}
		resp.Diagnostics.AddError("Failed to get Kubernetes Node Pool", fmt.Sprintf("Failed to get node pool: %s.",
			common.UnpackAPIError(err)))

		return
	}

	var state kubernetesNodePoolResourceModel
	nodePoolToResourceModel(ctx, &updatedNodePool, &plan, &state, &resp.Diagnostics)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesNodePoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var stored kubernetesNodePoolResourceModel

	diags := req.State.Get(ctx, &stored)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(r.client, stored.ProjectID.ValueString())

	asyncOperation, _, err := r.client.APIClient.KubernetesNodePoolsApi.DeleteNodePool(ctx, projectID, stored.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete node pool",
			fmt.Sprintf("Error starting a delete node pool operation: %s", common.UnpackAPIError(err)))

		return
	}

	_, _, err = common.AwaitOperationAndResolve[swagger.KubernetesNodePool](ctx, asyncOperation.Operation, projectID, r.client.APIClient.KubernetesNodePoolOperationsApi.GetKubernetesNodePoolsOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete node pool",
			fmt.Sprintf("Error deleting a node pool: %s", common.UnpackAPIError(err)))

		return
	}
}

func (r *kubernetesNodePoolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	nodePoolID, projectID, err := common.ParseResourceIdentifiers(req, r.client, "node_pool_id")

	if err != "" {
		resp.Diagnostics.AddError("Invalid resource identifier", err)

		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), nodePoolID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), projectID)...)
}

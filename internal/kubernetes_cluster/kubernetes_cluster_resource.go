package kubernetes_cluster

import (
	"context"
	"fmt"
	"math"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

var emptyStringList, _ = types.ListValue(types.StringType, []attr.Value{})

type kubernetesClusterResource struct {
	client *swagger.APIClient
}

func NewKubernetesClusterResource() resource.Resource {
	return &kubernetesClusterResource{}
}

type kubernetesClusterResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	ProjectID             types.String `tfsdk:"project_id"`
	Name                  types.String `tfsdk:"name"`
	Version               types.String `tfsdk:"version"`
	SubnetID              types.String `tfsdk:"subnet_id"`
	ClusterCidr           types.String `tfsdk:"cluster_cidr"`
	NodeCidrMaskSize      types.Int64  `tfsdk:"node_cidr_mask_size"`
	ServiceClusterIpRange types.String `tfsdk:"service_cluster_ip_range"`
	AddOns                types.List   `tfsdk:"add_ons"`
	Location              types.String `tfsdk:"location"`
	DNSName               types.String `tfsdk:"dns_name"`
	NodePoolIds           types.List   `tfsdk:"nodepool_ids"`
}

func (r *kubernetesClusterResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}

	client, ok := request.ProviderData.(*swagger.APIClient)
	if !ok {
		response.Diagnostics.AddError("Failed to initialize provider", common.ErrorMsgProviderInitFailed)

		return
	}

	r.client = client
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesClusterResource) Metadata(ctx context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_kubernetes_cluster"
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesClusterResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, // maintain across updates
			},
			"project_id": schema.StringAttribute{
				Computed:      true,
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"version": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
				Validators: []validator.String{stringvalidator.RegexMatches(
					regexp.MustCompile(`\d+\.\d+\.\d+-cmk\.\d+.*`), "must be in the format MAJOR.MINOR.BUGFIX-cmk.NUM (e.g 1.2.3-cmk.4)",
				)},
			},
			"subnet_id": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplaceIfConfigured()}, // cannot be updated in place
			},
			"cluster_cidr": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"node_cidr_mask_size": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown(), int64planmodifier.RequiresReplace()}, // cannot be updated in place
				Validators:    []validator.Int64{int64validator.AtMost(math.MaxInt32)},
			},
			"service_cluster_ip_range": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"add_ons": schema.ListAttribute{
				Computed:      true,
				Optional:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()}, // cannot be updated in place
				Default:       listdefault.StaticValue(emptyStringList),
			},
			"location": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"dns_name": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{},
			},
			"nodepool_ids": schema.ListAttribute{
				ElementType:   types.StringType,
				Computed:      true,
				PlanModifiers: []planmodifier.List{},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesClusterResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesClusterResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state kubernetesClusterResourceModel

	projectID := plan.ProjectID.ValueString()

	if projectID == "" {
		fallbackProjectID, fallbackErr := common.GetFallbackProject(ctx, r.client, &resp.Diagnostics)
		if fallbackErr != nil {
			resp.Diagnostics.AddError("Failed to fetch Node Pools",
				fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", fallbackErr))

			return
		}
		projectID = fallbackProjectID
	}

	addOns, err := common.TFListToStringSlice(plan.AddOns)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create cluster", fmt.Sprintf("could not create add on list: %s", err))

		return
	}

	//nolint:gosec // Sanity check for int64 --> int32 narrowing performed at field level (see schema)
	asyncOperation, _, err := r.client.KubernetesClustersApi.CreateCluster(ctx, swagger.KubernetesClusterPostRequest{
		AddOns:                addOns,
		ClusterCidr:           plan.ClusterCidr.ValueString(),
		Location:              plan.Location.ValueString(),
		Name:                  plan.Name.ValueString(),
		NodeCidrMaskSize:      int32(plan.NodeCidrMaskSize.ValueInt64()),
		ServiceClusterIpRange: plan.ServiceClusterIpRange.ValueString(),
		SubnetId:              plan.SubnetID.ValueString(),
		Version:               plan.Version.ValueString(),
	}, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create cluster",
			fmt.Sprintf("Error starting a create cluster operation: %s", common.UnpackAPIError(err)))

		return
	}

	kubernetesCluster, _, err := common.AwaitOperationAndResolve[swagger.KubernetesCluster](ctx, asyncOperation.Operation, projectID, r.client.KubernetesClusterOperationsApi.GetKubernetesClustersOperation)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create cluster",
			fmt.Sprintf("Error creating the cluster: %s", common.UnpackAPIError(err)))

		return
	}

	state.ID = types.StringValue(kubernetesCluster.Id)
	state.ProjectID = types.StringValue(kubernetesCluster.ProjectId)
	state.Name = types.StringValue(kubernetesCluster.Name)
	state.Version = types.StringValue(kubernetesCluster.Version)
	state.SubnetID = types.StringValue(kubernetesCluster.SubnetId)
	state.NodeCidrMaskSize = types.Int64Value(int64(kubernetesCluster.NodeCidrMaskSize))
	state.ClusterCidr = types.StringValue(kubernetesCluster.ClusterCidr)
	state.ServiceClusterIpRange = types.StringValue(kubernetesCluster.ServiceClusterIpRange)
	state.AddOns, diags = common.StringSliceToTFList(kubernetesCluster.AddOns)
	resp.Diagnostics.Append(diags...)
	state.Location = types.StringValue(kubernetesCluster.Location)
	state.DNSName = types.StringValue(kubernetesCluster.DnsName)
	state.NodePoolIds, diags = common.StringSliceToTFList(kubernetesCluster.NodePools)
	resp.Diagnostics.Append(diags...)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesClusterResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var stored kubernetesClusterResourceModel

	diags := req.State.Get(ctx, &stored)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	var state kubernetesClusterResourceModel

	diags = resp.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	projectID, err := common.GetProjectIDOrFallback(ctx, r.client, &resp.Diagnostics, state.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch project ID",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	// Interact with 3rd party API to read data source.
	kubernetesCluster, httpResp, err := r.client.KubernetesClustersApi.GetCluster(ctx, projectID, state.ID.ValueString())
	if err != nil {
		if httpResp != nil && httpResp.StatusCode == 404 {
			resp.State.RemoveResource(ctx)

			return
		}

		resp.Diagnostics.AddError("Failed to read Kubernetes Cluster",
			fmt.Sprintf("Failed to get cluster: %s", common.UnpackAPIError(err)))

		return
	}

	state.ID = types.StringValue(kubernetesCluster.Id)
	state.ProjectID = types.StringValue(kubernetesCluster.ProjectId)
	state.Name = types.StringValue(kubernetesCluster.Name)
	state.Version = types.StringValue(kubernetesCluster.Version)
	state.SubnetID = types.StringValue(kubernetesCluster.SubnetId)
	state.NodeCidrMaskSize = types.Int64Value(int64(kubernetesCluster.NodeCidrMaskSize))
	state.ClusterCidr = types.StringValue(kubernetesCluster.ClusterCidr)
	state.ServiceClusterIpRange = types.StringValue(kubernetesCluster.ServiceClusterIpRange)
	state.AddOns, diags = common.StringSliceToTFList(kubernetesCluster.AddOns)
	resp.Diagnostics.Append(diags...)
	state.Location = types.StringValue(kubernetesCluster.Location)
	state.DNSName = types.StringValue(kubernetesCluster.DnsName)
	state.NodePoolIds, diags = common.StringSliceToTFList(kubernetesCluster.NodePools)
	resp.Diagnostics.Append(diags...)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesClusterResource) Update(
	ctx context.Context,
	request resource.UpdateRequest,
	response *resource.UpdateResponse,
) {
	panic("Upgrading standard clusters to HA clusters is not currently supported")
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubernetesClusterResource) Delete(
	ctx context.Context,
	request resource.DeleteRequest,
	response *resource.DeleteResponse,
) {
	var stored kubernetesClusterResourceModel

	diags := request.State.Get(ctx, &stored)
	response.Diagnostics.Append(diags...)

	if response.Diagnostics.HasError() {
		return
	}

	projectID, err := common.GetProjectIDOrFallback(ctx, r.client, &response.Diagnostics, stored.ProjectID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Failed to fetch project ID",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	asyncOperation, _, err := r.client.KubernetesClustersApi.DeleteCluster(ctx, projectID, stored.ID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Failed to delete cluster",
			fmt.Sprintf("Error starting a delete cluster operation: %s", common.UnpackAPIError(err)))

		return
	}

	_, _, err = common.AwaitOperationAndResolve[swagger.KubernetesCluster](
		ctx,
		asyncOperation.Operation,
		projectID,
		r.client.KubernetesClusterOperationsApi.GetKubernetesClustersOperation)
	if err != nil {
		response.Diagnostics.AddError("Failed to delete cluster",
			fmt.Sprintf("Error deleting the cluster: %s", common.UnpackAPIError(err)))

		return
	}
}

package kubeconfig

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"k8s.io/client-go/tools/clientcmd"
	k8sApi "k8s.io/client-go/tools/clientcmd/api"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource = &kubeConfigResource{}
)

type kubeConfigResource struct {
	client *swagger.APIClient
}

// NewKubeConfigResource is a helper function to simplify the provider implementation.
func NewKubeConfigResource() resource.Resource {
	return &kubeConfigResource{}
}

type kubeConfigResourceModel struct {
	ClusterID            types.String `tfsdk:"cluster_id"`
	ProjectID            types.String `tfsdk:"project_id"`
	ClusterAddress       types.String `tfsdk:"cluster_address"`
	ClusterCACertificate types.String `tfsdk:"cluster_ca_certificate"`
	ClusterName          types.String `tfsdk:"cluster_name"`
	ClientCertificate    types.String `tfsdk:"client_certificate"`
	ClientKey            types.String `tfsdk:"client_key"`
	UserName             types.String `tfsdk:"username"`
	KubeConfigYaml       types.String `tfsdk:"kubeconfig_yaml"`
}

func templateKubeConfig(params *swagger.KubernetesAuthenticationClientCertificateDetails) (*string, error) {
	kubeConfig := k8sApi.NewConfig()

	// Create a new cluster with the given address and CA certificate
	cluster := k8sApi.NewCluster()
	cluster.Server = params.ClusterAddress
	cluster.CertificateAuthorityData = []byte(params.ClusterCaCertificate)

	// Create a new auth info (user) with the given certificate and key
	authInfo := k8sApi.NewAuthInfo()
	authInfo.ClientCertificateData = []byte(params.UserClientCertificate)
	authInfo.ClientKeyData = []byte(params.UserClientKey)

	// Create a new context using the cluster and auth info
	kubeContext := k8sApi.NewContext()
	kubeContext.Cluster = params.ClusterName
	kubeContext.AuthInfo = params.UserName

	// Add the cluster to the config
	kubeConfig.Clusters[params.ClusterName] = cluster
	// Add the auth info to the config
	kubeConfig.AuthInfos[params.UserName] = authInfo
	// Add the context to the config and set it as the current context
	kubeConfig.Contexts[params.ClusterName] = kubeContext
	kubeConfig.CurrentContext = params.ClusterName

	kubeConfigYamlBytes, err := clientcmd.Write(*kubeConfig)
	if err != nil {
		return nil, err
	}

	kubeConfigYaml := string(kubeConfigYamlBytes)

	return &kubeConfigYaml, nil
}

func (r *kubeConfigResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
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

// Metadata returns the resource type name.
func (r *kubeConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubeconfig"
}

// Schema defines the schema for the resource.
func (r *kubeConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"cluster_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, // cannot be updated in place
			},
			"project_id": schema.StringAttribute{
				Computed:      true,
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplaceIfConfigured()}, // cannot be updated in place
			},
			"cluster_address": schema.StringAttribute{
				Computed: true,
			},
			"cluster_ca_certificate": schema.StringAttribute{
				Computed: true,
			},
			"cluster_name": schema.StringAttribute{
				Computed: true,
			},
			"client_certificate": schema.StringAttribute{
				Computed: true,
			},
			"client_key": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
			"username": schema.StringAttribute{
				Computed: true,
			},
			"kubeconfig_yaml": schema.StringAttribute{
				Computed:  true,
				Sensitive: true,
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubeConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubeConfigResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state kubeConfigResourceModel

	projectID, err := common.GetProjectIDOrFallback(ctx, r.client, &resp.Diagnostics, state.ProjectID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch project ID",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	res, _, err := r.client.KubernetesClustersApi.GetClusterCredentials(ctx, projectID, plan.ClusterID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to create kubeconfig",
			fmt.Sprintf("Error creating kubeconfig: %s", common.UnpackAPIError(err)))

		return
	}

	state.ClusterID = types.StringValue(plan.ClusterID.ValueString())
	state.ProjectID = types.StringValue(projectID)
	state.ClusterAddress = types.StringValue(res.ClusterAddress)
	state.ClusterCACertificate = types.StringValue(res.ClusterCaCertificate)
	state.ClusterName = types.StringValue(res.ClusterName)
	state.ClientCertificate = types.StringValue(res.UserClientCertificate)
	state.ClientKey = types.StringValue(res.UserClientKey)
	state.UserName = types.StringValue(res.UserName)

	kubeConfigYaml, err := templateKubeConfig(&res)
	if err != nil {
		resp.Diagnostics.AddError("Failed to template kubeconfig",
			fmt.Sprintf("Error templating kubeconfig: %s", err))

		return
	}
	state.KubeConfigYaml = types.StringValue(*kubeConfigYaml)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubeConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var stored kubeConfigResourceModel
	diags := req.State.Get(ctx, &stored)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state kubeConfigResourceModel

	if stored.ClusterID.IsUnknown() ||
		stored.ProjectID.IsUnknown() ||
		stored.ClusterAddress.IsUnknown() ||
		stored.ClusterCACertificate.IsUnknown() ||
		stored.ClusterName.IsUnknown() ||
		stored.ClientCertificate.IsUnknown() ||
		stored.ClientKey.IsUnknown() ||
		stored.UserName.IsUnknown() {

		resp.State.RemoveResource(ctx)

		return
	}

	state.ClusterID = stored.ClusterID
	state.ProjectID = stored.ProjectID
	state.ClusterAddress = stored.ClusterAddress
	state.ClusterCACertificate = stored.ClusterCACertificate
	state.ClusterName = stored.ClusterName
	state.ClientCertificate = stored.ClientCertificate
	state.ClientKey = stored.ClientKey
	state.UserName = stored.UserName

	kubeConfigYaml, err := templateKubeConfig(
		&swagger.KubernetesAuthenticationClientCertificateDetails{
			ClusterAddress:        state.ClusterAddress.ValueString(),
			ClusterCaCertificate:  state.ClusterCACertificate.ValueString(),
			ClusterName:           state.ClusterName.ValueString(),
			UserClientCertificate: state.ClientCertificate.ValueString(),
			UserClientKey:         state.ClientKey.ValueString(),
			UserName:              state.UserName.ValueString(),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to template kubeconfig",
			fmt.Sprintf("Error templating kubeconfig: %s", err))

		return
	}
	state.KubeConfigYaml = types.StringValue(*kubeConfigYaml)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubeConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	panic("Updating kubeconfig is not currently supported")
}

//nolint:gocritic // Implements Terraform defined interface
func (r *kubeConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.State.RemoveResource(ctx)
}

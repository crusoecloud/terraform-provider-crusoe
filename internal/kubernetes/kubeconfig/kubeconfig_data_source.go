package kubeconfig

import (
	"context"
	"fmt"

	"github.com/antihax/optional"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ datasource.DataSource = &kubeConfigDataSource{}
)

// NewKubeConfigDataSource is a helper function to simplify the provider implementation.
func NewKubeConfigDataSource() datasource.DataSource {
	return &kubeConfigDataSource{}
}

// kubeConfigDataSource is the data source implementation.
type kubeConfigDataSource struct {
	client *common.CrusoeClient
}

type kubeConfigDataSourceModel struct {
	ClusterID            types.String `tfsdk:"cluster_id"`
	ProjectID            types.String `tfsdk:"project_id"`
	ClusterAddress       types.String `tfsdk:"cluster_address"`
	ClusterCACertificate types.String `tfsdk:"cluster_ca_certificate"`
	ClusterName          types.String `tfsdk:"cluster_name"`
	ClientCertificate    types.String `tfsdk:"client_certificate"`
	ClientKey            types.String `tfsdk:"client_key"`
	UserName             types.String `tfsdk:"username"`
	KubeConfigYaml       types.String `tfsdk:"kubeconfig_yaml"`
	AuthType             types.String `tfsdk:"auth_type"`
}

func (ds *kubeConfigDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*common.CrusoeClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", common.ErrorMsgProviderInitFailed)

		return
	}

	ds.client = client
}

// Metadata returns the data source type name.
func (ds *kubeConfigDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubeconfig"
}

// Schema defines the schema for the data source.
func (ds *kubeConfigDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"cluster_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: apiDescClusterID,
			},
			"project_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: providerDescProjectID,
			},
			"cluster_address": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: apiDescClusterAddress,
			},
			"cluster_ca_certificate": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: apiDescClusterCACertificate,
			},
			"cluster_name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: apiDescClusterName,
			},
			"client_certificate": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: apiDescClientCertificate,
			},
			"client_key": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: apiDescClientKey,
			},
			"username": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: apiDescUserName,
			},
			"kubeconfig_yaml": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: apiDescKubeConfigYaml,
			},
			"auth_type": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: apiDescAuthType + " " + providerDescAuthTypeSuffix,
				Validators: []validator.String{
					stringvalidator.OneOf("admin_cert", "oidc"),
				},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *kubeConfigDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config kubeConfigDataSourceModel

	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(ds.client, config.ProjectID.ValueString())

	var opts *swagger.KubernetesClustersApiGetClusterCredentialsOpts
	if config.AuthType.ValueString() != "" {
		opts = &swagger.KubernetesClustersApiGetClusterCredentialsOpts{
			AuthType: optional.NewString(config.AuthType.ValueString()),
		}
	}

	res, httpResp, err := ds.client.APIClient.KubernetesClustersApi.GetClusterCredentials(ctx, projectID, config.ClusterID.ValueString(), opts)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to read kubeconfig",
			fmt.Sprintf("Error reading kubeconfig: %s", common.UnpackAPIError(err)))

		return
	}

	var state kubeConfigDataSourceModel

	state.ClusterID = config.ClusterID
	state.ProjectID = types.StringValue(projectID)
	state.AuthType = config.AuthType
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

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

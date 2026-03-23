package crusoe

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/antihax/optional"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/custom_image"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/disk"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/firewall_rule"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/ib_network"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/ib_partition"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/instance_group"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/instance_template"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/kubernetes/kubeconfig"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/kubernetes/kubernetes_cluster"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/kubernetes/kubernetes_node_pool"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/load_balancer"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/registry/image"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/registry/manifest"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/registry/repository"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/registry/token"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/s3_bucket"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/vm"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/vpc_network"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/vpc_subnet"
)

type crusoeProvider struct{}

type crusoeProviderModel struct {
	ApiEndpoint types.String `tfsdk:"api_endpoint"`
	Profile     types.String `tfsdk:"profile"`
	Project     types.String `tfsdk:"project"`
}

func New() provider.Provider {
	return &crusoeProvider{}
}

// Metadata returns the provider type name.
func (p *crusoeProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "crusoe"
}

// Schema defines the provider-level schema for configuration data.
func (p *crusoeProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The Crusoe Cloud provider enables management of Crusoe Cloud resources.",
		Attributes: map[string]schema.Attribute{
			"api_endpoint": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The Crusoe API endpoint. Defaults to `https://api.crusoecloud.com/v1alpha5`. Can also be set via `CRUSOE_API_ENDPOINT` environment variable.",
			},
			"profile": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The name of the profile to use from `~/.crusoe/config`. When specified, credentials and default_project are loaded from this profile. Takes precedence over `CRUSOE_PROFILE` environment variable.",
			},
			"project": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The default project for resources. Can be a project name or UUID (resolved to UUID internally). Can be overridden per-resource via `project_id`. Takes precedence over `CRUSOE_DEFAULT_PROJECT` environment variable.",
			},
		},
	}
}

// DataSources defines the data sources implemented in the provider.
func (p *crusoeProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		vm.NewVMDataSource,
		disk.NewDisksDataSource,
		ib_network.NewIBNetworkDataSource,
		project.NewProjectsDataSource,
		vpc_network.NewVPCNetworksDataSource,
		vpc_subnet.NewVPCSubnetsDataSource,
		instance_template.NewInstanceTemplatesDataSource,
		instance_group.NewInstanceGroupsDataSource,
		load_balancer.NewLoadBalancerDataSource,
		kubernetes_cluster.NewKubernetesClusterDataSource,
		kubernetes_node_pool.NewKubernetesNodePoolDataSource,
		custom_image.NewCustomImageDataSource,
		repository.NewRegistryRepositoriesDataSource,
		image.NewRegistryImagesDataSource,
		manifest.NewRegistryManifestsDataSource,
		token.NewRegistryTokensDataSource,
		s3_bucket.NewS3BucketsDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *crusoeProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		vm.NewVMResource,
		vm.NewVMByTemplateResource,
		disk.NewDiskResource,
		firewall_rule.NewFirewallRuleResource,
		ib_partition.NewIBPartitionResource,
		project.NewProjectResource,
		vpc_network.NewVPCNetworkResource,
		vpc_subnet.NewVPCSubnetResource,
		instance_template.NewInstanceTemplateResource,
		instance_group.NewInstanceGroupResource,
		load_balancer.NewLoadBalancerResource,
		kubeconfig.NewKubeConfigResource,
		kubernetes_cluster.NewKubernetesClusterResource,
		kubernetes_node_pool.NewKubernetesNodePoolResource,
		repository.NewRegistryRepositoryResource,
		token.NewRegistryTokenResource,
		s3_bucket.NewS3BucketResource,
	}
}

// Configure prepares a Crusoe API client for data sources and resources.
func (p *crusoeProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config crusoeProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Warn if empty string explicitly provided (helps catch config mistakes)
	if !config.Profile.IsNull() && config.Profile.ValueString() == "" {
		resp.Diagnostics.AddWarning("Empty profile value",
			"Empty string provided for 'profile' in provider block, falling back to CRUSOE_PROFILE env or config file default.")
	}
	if !config.Project.IsNull() && config.Project.ValueString() == "" {
		resp.Diagnostics.AddWarning("Empty project value",
			"Empty string provided for 'project' in provider block, falling back to CRUSOE_DEFAULT_PROJECT env or profile default.")
	}

	if updateMessage := common.GetUpdateMessageIfValid(context.Background()); updateMessage != "" {
		resp.Diagnostics.AddWarning("Update Available",
			fmt.Sprintf("There is a newer version available for the Crusoe Terraform Provider.\n%s", updateMessage))
	}

	// Build config options from provider block
	opts := common.ConfigOptions{
		Profile: config.Profile.ValueString(),
		Project: config.Project.ValueString(),
	}

	clientConfig, err := common.GetConfigWithOptions(opts)
	if err != nil {
		// only show a warning, since it's possible that we can't read their home dir (which is unexpected) but
		// they have everything set via env variables, so we can still proceed.
		resp.Diagnostics.AddWarning("Issue Reading Config",
			fmt.Sprintf("There was an issue reading your Crusoe Config. Terraform may not have permission to"+
				" read your home directory.\n\nWarning: %s", err.Error()))
	}

	if clientConfig.AccessKeyID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("profile"),
			"Missing Crusoe API Key",
			"The provider cannot create the Crusoe API client as there is a missing or empty value for the Crusoe API key. "+
				"Set the value in ~/.crusoe/config, use the CRUSOE_ACCESS_KEY_ID environment variable, "+
				"or specify a profile in the provider block. If already set, ensure the value is not empty.",
		)
	}

	if clientConfig.SecretKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("profile"),
			"Missing Crusoe API Secret",
			"The provider cannot create the Crusoe API client as there is a missing or empty value for the Crusoe API secret. "+
				"Set the value in ~/.crusoe/config, use the CRUSOE_SECRET_KEY environment variable, "+
				"or specify a profile in the provider block. If already set, ensure the value is not empty.",
		)
	}

	// Exit if there are missing required attributes
	if resp.Diagnostics.HasError() {
		return
	}

	// Create an API client and make it available during DataSource and Resource type Configure methods.
	apiClient := common.NewAPIClient(clientConfig.ApiEndpoint, clientConfig.AccessKeyID, clientConfig.SecretKey)

	var projectId string
	var getError error

	if clientConfig.DefaultProject != "" {
		// some users use the project id for default project, try parse it as a uuid and if no error use it as such
		_, uuidParseErr := uuid.Parse(clientConfig.DefaultProject)

		if uuidParseErr == nil {
			projectId = clientConfig.DefaultProject
			_, getError = getProjectById(ctx, apiClient.ProjectsApi, clientConfig.DefaultProject)
		} else {
			projectId, getError = getProjectByName(ctx, apiClient.ProjectsApi, clientConfig.DefaultProject)
		}
	} else {
		var projectName string
		projectId, projectName, getError = getDefaultProject(ctx, apiClient.ProjectsApi)

		if getError == nil {
			resp.Diagnostics.AddAttributeWarning(
				path.Root("project"),
				"Using Fallback Project",
				fmt.Sprintf("The provider did not find a default project specified and will use %q as the fallback project.\n\nSet 'project' in the provider block, CRUSOE_DEFAULT_PROJECT env, or default_project in ~/.crusoe/config.", projectName),
			)
		}
	}

	if getError != nil {
		userId := "unknown"
		user, _, err := apiClient.IdentitiesApi.GetUserIdentity(ctx)

		if err != nil {
			resp.Diagnostics.AddAttributeError(
				path.Root("profile"),
				"Failed to auth user",
				fmt.Sprintf("The provider failed to get the users identity for config profile %q. \n\nError: %s \n\nCheck config in ~/.crusoe/config or verify the profile in the provider block.", clientConfig.ProfileName, err.Error()))
		} else {
			userId = user.Identity.Email
		}

		if clientConfig.DefaultProject == "" {
			resp.Diagnostics.AddAttributeError(
				path.Root("project"),
				fmt.Sprintf("The provider did not find a default project specified and failed to infer a fallback project for the authenticated user (%s)", userId),
				fmt.Sprintf("Error: %s \n\nSet 'project' in the provider block, CRUSOE_DEFAULT_PROJECT env, or default_project in ~/.crusoe/config.", getError.Error()),
			)
		} else {
			resp.Diagnostics.AddAttributeError(
				path.Root("project"),
				fmt.Sprintf("Failed to resolve the project for the authenticated user (%s)", userId),
				fmt.Sprintf("Error: %s \n\nCheck the value of 'project' in the provider block or default_project in ~/.crusoe/config.", getError.Error()),
			)
		}

		return
	}

	client := &common.CrusoeClient{
		APIClient: apiClient,
		ProjectID: projectId,
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func getDefaultProject(ctx context.Context, projectsApiService *swagger.ProjectsApiService) (projectId, projectName string, err error) {
	opts := &swagger.ProjectsApiListProjectsOpts{
		OrgId: optional.EmptyString(),
	}

	dataResp, _, err := projectsApiService.ListProjects(ctx, opts)
	if err != nil {
		return "", "", fmt.Errorf("failed to list projects: %w", err)
	}

	if len(dataResp.Items) == 0 {
		return "", "", fmt.Errorf("no projects found")
	}

	if len(dataResp.Items) > 1 {
		var projectNames []string
		for _, project := range dataResp.Items {
			projectNames = append(projectNames, project.Name)
		}

		slices.Sort(projectNames)

		if len(projectNames) > 5 {
			projectNames = append(projectNames[:5], "...")
		}

		return "", "", fmt.Errorf("failed to infer default project as more than one project found (%s)", strings.Join(projectNames, ", "))
	}

	return dataResp.Items[0].Id, dataResp.Items[0].Name, nil
}

func getProjectByName(ctx context.Context, projectsApiService *swagger.ProjectsApiService, projectName string) (projectId string, err error) {
	opts := &swagger.ProjectsApiListProjectsOpts{
		OrgId:       optional.EmptyString(),
		ProjectName: optional.NewString(projectName),
	}

	dataResp, _, err := projectsApiService.ListProjects(ctx, opts)
	if err != nil {
		return "", fmt.Errorf("failed to get project by name %q: %w", projectName, err)
	}

	if len(dataResp.Items) == 0 {
		return "", fmt.Errorf("failed to find project with name %q", projectName)
	}

	if len(dataResp.Items) > 1 {
		return "", fmt.Errorf("internal error: got more than one project with name %q (%d)", projectName, len(dataResp.Items))
	}

	return dataResp.Items[0].Id, nil
}

func getProjectById(ctx context.Context, projectsApiService *swagger.ProjectsApiService, projectId string) (projectName string, err error) {
	dataResp, _, err := projectsApiService.GetProject(ctx, projectId)
	if err != nil {
		return "", fmt.Errorf("failed to get project by id %q: %w", projectId, err)
	}

	return dataResp.Name, nil
}

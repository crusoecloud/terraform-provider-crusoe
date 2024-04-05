package crusoe

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/disk"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/firewall_rule"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/ib_network"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/ib_partition"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/instance_template"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/vm"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/vpc_network"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/vpc_subnet"
)

type crusoeProvider struct{}

type crusoeProviderModel struct {
	ApiEndpoint types.String `tfsdk:"api_endpoint"`
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
		Attributes: map[string]schema.Attribute{
			"api_endpoint": schema.StringAttribute{
				Optional: true,
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

	if updateMessage := common.GetUpdateMessageIfValid(context.Background()); updateMessage != "" {
		resp.Diagnostics.AddWarning("Update Available",
			fmt.Sprintf("There is a newer version available for the Crusoe Terraform Provider.\n%s", updateMessage))
	}

	clientConfig, err := common.GetConfig()
	if err != nil {
		// only show a warning, since it's possible that we can't read their home dir (which is unexpected) but
		// they have everything set via env variables, so we can still proceed.
		resp.Diagnostics.AddWarning("Issue Reading Config",
			fmt.Sprintf("There was an issue reading your Crusoe Config. Terraform may not have permission to"+
				" read your home directory.\n\nWarning: %s", err.Error()))
	}

	if clientConfig.AccessKeyID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_key"),
			"Missing Crusoe API Key",
			"The provider cannot create the Crusoe API client as there is a missing or empty value for the Crusoe API key. "+
				"Set the value in ~/.crusoe/config or use the CRUSOE_ACCESS_KEY_ID environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if clientConfig.SecretKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("secret_key"),
			"Missing Crusoe API Secret",
			"The provider cannot create the Crusoe API client as there is a missing or empty value for the Crusoe API secret. "+
				"Set the value in ~/.crusoe/config or use the CRUSOE_SECRET_KEY environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if clientConfig.DefaultProject == "" {
		resp.Diagnostics.AddAttributeWarning(
			path.Root("default_project"),
			"Missing Crusoe Default Project",
			"The provider did not find a default project specified in the configuration file and will attempt to infer the project to use when not specified. "+
				"Set default_project in ~/.crusoe/config. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Create an API client and make it available during DataSource and Resource type Configure methods.
	client := common.NewAPIClient(clientConfig.ApiEndpoint, clientConfig.AccessKeyID, clientConfig.SecretKey)
	resp.DataSourceData = client
	resp.ResourceData = client
}

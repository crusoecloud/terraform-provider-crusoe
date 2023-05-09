package crusoe

import (
	"context"
	"os"

	"terraform-provider-crusoe/internal"
	"terraform-provider-crusoe/internal/disk"
	"terraform-provider-crusoe/internal/firewall_rule"
	"terraform-provider-crusoe/internal/vm"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const defaultApiEndpoint = "https://api.crusoecloud.com/v1alpha4"

type crusoeProvider struct{}

type crusoeProviderModel struct {
	Host      types.String `tfsdk:"host"`
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
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
			"host": schema.StringAttribute{
				Optional: true,
			},
			"access_key": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
			},
			"secret_key": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
			},
		},
	}
}

// DataSources defines the data sources implemented in the provider.
func (p *crusoeProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		vm.NewVMDataSource,
		disk.NewDisksDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *crusoeProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		vm.NewVMResource,
		disk.NewDiskResource,
		firewall_rule.NewFirewallRuleResource,
	}
}

// Configure prepares a Crusoe API client for data sources and resources.
func (p *crusoeProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	// Retrieve provider data from configuration
	var config crusoeProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Default to environment variables, but override with Terraform configuration values if set.
	accessKey := os.Getenv("CRUSOE_ACCESS_KEY")
	secretKey := os.Getenv("CRUSOE_SECRET_KEY")
	host := os.Getenv("CRUSOE_API_ENDPOINT")

	if config.AccessKey.ValueString() != "" {
		accessKey = config.AccessKey.ValueString()
	}
	if config.SecretKey.ValueString() != "" {
		secretKey = config.SecretKey.ValueString()
	}
	if config.Host.ValueString() != "" {
		host = config.Host.ValueString()
	}

	if host == "" {
		host = defaultApiEndpoint
	}

	if accessKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_key"),
			"Missing Crusoe API Key",
			"The provider cannot create the Crusoe API client as there is a missing or empty value for the Crusoe API key. "+
				"Set the access value in the configuration or use the CRUSOE_ACCESS_KEY environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if secretKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("secret_key"),
			"Missing Crusoe API Secret",
			"The provider cannot create the Crusoe API client as there is a missing or empty value for the Crusoe API secret. "+
				"Set the password value in the configuration or use the CRUSOE_SECRET_KEY environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Create an API client and make it available during DataSource and Resource type Configure methods.
	client := internal.NewAPIClient(host, accessKey, secretKey)
	resp.DataSourceData = client
	resp.ResourceData = client
}

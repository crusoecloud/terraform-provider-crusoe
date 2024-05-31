package load_balancer

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type loadBalancerDataSource struct {
	client *swagger.APIClient
}

type loadBalancerDataSourceModel struct {
	ProjectID     *string             `tfsdk:"project_id"`
	LoadBalancers []loadBalancerModel `tfsdk:"load_balancers"`
}

type networkInterfaceModel struct {
	NetworkID string `tfsdk:"network_id"`
	SubnetID  string `tfsdk:"subnet_id"`
}

type destinationModel struct {
	Cidr       string `tfsdk:"cidr"`
	ResourceID string `tfsdk:"resource_id"`
}

type ipAddressesModel struct {
	PrivateIPv4 lbIPv4 `tfsdk:"private_ipv4"`
	PublicIPv4  lbIPv4 `tfsdk:"public_ipv4"`
}

type lbIPv4 struct {
	Address string `tfsdk:"address"`
}

type healthCheckOptionsModel struct {
	Timeout      string `tfsdk:"timeout"`
	Port         string `tfsdk:"port"`
	Interval     string `tfsdk:"interval"`
	SuccessCount string `tfsdk:"success_count"`
	FailureCount string `tfsdk:"failure_count"`
}

type loadBalancerModel struct {
	ID                string                   `tfsdk:"id"`
	Name              string                   `tfsdk:"name"`
	NetworkInterfaces []networkInterfaceModel  `tfsdk:"network_interfaces"`
	Destinations      []destinationModel       `tfsdk:"destinations"`
	Location          string                   `tfsdk:"location"`
	Protocols         []string                 `tfsdk:"protocols"`
	Algorithm         string                   `tfsdk:"algorithm"`
	Type              string                   `tfsdk:"type"`
	IPs               []ipAddressesModel       `tfsdk:"ips"`
	HealthCheck       *healthCheckOptionsModel `tfsdk:"health_check"`
}

func NewLoadBalancerDataSource() datasource.DataSource {
	return &loadBalancerDataSource{}
}

// Configure adds the provider configured client to the data source.
func (ds *loadBalancerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*swagger.APIClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", common.ErrorMsgProviderInitFailed)

		return
	}

	ds.client = client
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *loadBalancerDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_load_balancer"
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *loadBalancerDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"load_balancers": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed: true,
					},
					"name": schema.StringAttribute{
						Computed: true,
					},
					"network_interfaces": schema.ListNestedAttribute{
						Computed: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"network_id": schema.StringAttribute{
									Computed: true,
								},
								"subnet_id": schema.StringAttribute{
									Computed: true,
								},
							},
						},
					},
					"destinations": schema.ListNestedAttribute{
						Computed: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"cidr": schema.StringAttribute{
									Computed: true,
								},
								"resource_id": schema.StringAttribute{
									Computed: true,
								},
							},
						},
					},
					"location": schema.StringAttribute{
						Computed: true,
					},
					"protocols": schema.ListAttribute{
						ElementType: types.StringType,
						Computed:    true,
					},
					"algorithm": schema.StringAttribute{
						Computed: true,
					},
					"type": schema.StringAttribute{
						Computed: true,
					},
					"ips": schema.ListNestedAttribute{
						Computed: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"public_ipv4": schema.SingleNestedAttribute{
									Computed: true,
									Optional: true,
									Attributes: map[string]schema.Attribute{
										"address": schema.StringAttribute{
											Computed: true,
										},
									},
								},
								"private_ipv4": schema.SingleNestedAttribute{
									Computed: true,
									Attributes: map[string]schema.Attribute{
										"address": schema.StringAttribute{
											Computed: true,
										},
									},
								},
							},
						},
					},
					"health_check": schema.SingleNestedAttribute{
						Computed: true,
						Attributes: map[string]schema.Attribute{
							"timeout": schema.StringAttribute{
								Computed: true,
							},
							"port": schema.StringAttribute{
								Computed: true,
							},
							"interval": schema.StringAttribute{
								Computed: true,
							},
							"success_count": schema.StringAttribute{
								Computed: true,
							},
							"failure_count": schema.StringAttribute{
								Computed: true,
							},
						},
					},
				},
			},
		},
		"project_id": schema.StringAttribute{
			Optional: true,
		},
	}}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *loadBalancerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config loadBalancerDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	projectID := ""
	if config.ProjectID != nil {
		projectID = *config.ProjectID
	} else {
		fallbackProjectID, err := common.GetFallbackProject(ctx, ds.client, &resp.Diagnostics)
		if err != nil {
			resp.Diagnostics.AddError("Failed to fetch load balancers",
				fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

			return
		}
		projectID = fallbackProjectID
	}

	dataResp, httpResp, err := ds.client.LoadBalancersApi.ListLoadBalancers(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch load balancers", "Could not fetch load balancers data at this time.")

		return
	}
	defer httpResp.Body.Close()

	var state loadBalancerDataSourceModel
	for i := range dataResp.Items {
		state.LoadBalancers = append(state.LoadBalancers, loadBalancerModel{
			ID:                dataResp.Items[i].Id,
			Name:              dataResp.Items[i].Name,
			NetworkInterfaces: loadBalancerNetworkInterfacesToTerraformDataModel(dataResp.Items[i].NetworkInterfaces),
			Destinations:      loadBalancerDestinationsToTerraformDataModel(dataResp.Items[i].Destinations),
			Location:          dataResp.Items[i].Location,
			Protocols:         dataResp.Items[i].Protocols,
			Algorithm:         dataResp.Items[i].Algorithm,
			Type:              dataResp.Items[i].Type_,
			IPs:               loadBalancerIPsToTerraformDataModel(dataResp.Items[i].Ips),
			HealthCheck: &healthCheckOptionsModel{
				Timeout:      dataResp.Items[i].HealthCheck.Timeout,
				Port:         dataResp.Items[i].HealthCheck.Port,
				Interval:     dataResp.Items[i].HealthCheck.Interval,
				SuccessCount: dataResp.Items[i].HealthCheck.SuccessCount,
				FailureCount: dataResp.Items[i].HealthCheck.FailureCount,
			},
		})
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

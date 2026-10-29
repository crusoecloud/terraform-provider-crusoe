package load_balancer

import (
	"context"

	"github.com/antihax/optional"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type loadBalancerDataSource struct {
	client *common.CrusoeClient
}

type loadBalancerDataSourceModel struct {
	ProjectID     types.String        `tfsdk:"project_id"`
	Location      types.String        `tfsdk:"location"`
	VPCNetworkID  types.String        `tfsdk:"vpc_network_id"`
	Name          types.String        `tfsdk:"name"`
	LoadBalancers []loadBalancerModel `tfsdk:"load_balancers"`
}

type loadBalancerModel struct {
	ID                     string                      `tfsdk:"id"`
	ProjectID              string                      `tfsdk:"project_id"`
	Name                   string                      `tfsdk:"name"`
	Location               string                      `tfsdk:"location"`
	VPCID                  string                      `tfsdk:"vpc_id"`
	Protocol               string                      `tfsdk:"protocol"`
	VIP                    string                      `tfsdk:"vip"`
	ListenPortsAndBackends []listenPortAndBackendModel `tfsdk:"listen_ports_and_backends"`
	HealthCheckOptions     *healthCheckOptionsModel    `tfsdk:"health_check_options"`
}

type listenPortAndBackendModel struct {
	ListenPort int64          `tfsdk:"listen_port"`
	Backends   []backendModel `tfsdk:"backends"`
}

type backendModel struct {
	IP     string `tfsdk:"ip"`
	Port   int64  `tfsdk:"port"`
	Status string `tfsdk:"status"`
}

type healthCheckOptionsModel struct {
	Timeout      int64 `tfsdk:"timeout"`
	Interval     int64 `tfsdk:"interval"`
	SuccessCount int64 `tfsdk:"success_count"`
	FailureCount int64 `tfsdk:"failure_count"`
}

func NewLoadBalancerDataSource() datasource.DataSource {
	return &loadBalancerDataSource{}
}

func (ds *loadBalancerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

//nolint:gocritic // Implements Terraform defined interface
func (ds *loadBalancerDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_load_balancer"
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *loadBalancerDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{
		MarkdownDescription: common.DevelopmentMessage,
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: providerDescProjectID,
			},
			"location": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: providerDescFilterLocation,
			},
			"vpc_network_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: providerDescFilterVPCNetworkID,
			},
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: providerDescFilterName,
			},
			"load_balancers": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: providerDescLoadBalancers,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: apiDescID,
						},
						"project_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: apiDescProjectID,
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: apiDescName,
						},
						"location": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: apiDescLocation,
						},
						"vpc_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: apiDescVPCID,
						},
						"protocol": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: apiDescProtocol + " " + providerDescProtocol,
						},
						"vip": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: apiDescVIP,
						},
						"listen_ports_and_backends": schema.ListNestedAttribute{
							Computed:            true,
							MarkdownDescription: apiDescListenPortsAndBackends,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"listen_port": schema.Int64Attribute{
										Computed:            true,
										MarkdownDescription: apiDescListenPort,
									},
									"backends": schema.ListNestedAttribute{
										Computed:            true,
										MarkdownDescription: apiDescBackends,
										NestedObject: schema.NestedAttributeObject{
											Attributes: map[string]schema.Attribute{
												"ip": schema.StringAttribute{
													Computed:            true,
													MarkdownDescription: apiDescBackendIP,
												},
												"port": schema.Int64Attribute{
													Computed:            true,
													MarkdownDescription: apiDescBackendPort,
												},
												"status": schema.StringAttribute{
													Computed:            true,
													MarkdownDescription: apiDescBackendStatus,
												},
											},
										},
									},
								},
							},
						},
						"health_check_options": schema.SingleNestedAttribute{
							Computed: true,
							Attributes: map[string]schema.Attribute{
								"timeout": schema.Int64Attribute{
									Computed:            true,
									MarkdownDescription: apiDescHealthCheckTimeout,
								},
								"interval": schema.Int64Attribute{
									Computed:            true,
									MarkdownDescription: apiDescHealthCheckInterval,
								},
								"success_count": schema.Int64Attribute{
									Computed:            true,
									MarkdownDescription: apiDescHealthCheckSuccessCount,
								},
								"failure_count": schema.Int64Attribute{
									Computed:            true,
									MarkdownDescription: apiDescHealthCheckFailureCount,
								},
							},
						},
					},
				},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *loadBalancerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config loadBalancerDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(ds.client, config.ProjectID.ValueString())

	opts := &swagger.LoadBalancersApiListExternalLoadBalancersOpts{}
	if !config.Location.IsNull() {
		opts.Location = optional.NewString(config.Location.ValueString())
	}
	if !config.VPCNetworkID.IsNull() {
		opts.VpcNetworkId = optional.NewString(config.VPCNetworkID.ValueString())
	}
	if !config.Name.IsNull() {
		opts.Name = optional.NewString(config.Name.ValueString())
	}

	dataResp, httpResp, err := ds.client.APIClient.LoadBalancersApi.ListExternalLoadBalancers(ctx, projectID, opts)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch load balancers",
			"Could not fetch load balancers data at this time: "+common.UnpackAPIError(err).Error())

		return
	}

	state := config
	state.ProjectID = types.StringValue(projectID)
	state.LoadBalancers = make([]loadBalancerModel, 0, len(dataResp.Items))
	for i := range dataResp.Items {
		state.LoadBalancers = append(state.LoadBalancers, loadBalancerToDataModel(&dataResp.Items[i]))
	}

	// Sort load balancers deterministically so repeated reads produce a stable ordering.
	common.SortByKeys(state.LoadBalancers,
		func(lb loadBalancerModel) string { return lb.Name },
		func(lb loadBalancerModel) string { return lb.ID },
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

package vm

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	swagger "gitlab.com/crusoeenergy/island/external/client-go/swagger/v1alpha4"

	"github.com/crusoecloud/terraform-provider-crusoe/internal"
)

// vmDataSource is a Terraform datasource that can be used to fetch a single VM instance.
// TODO: consider making another DataSource for getting multiple instances
type vmDataSource struct {
	client *swagger.APIClient
}

type vmDataSourceFilter struct {
	ID                *string                       `tfsdk:"id"`
	Name              *string                       `tfsdk:"name"`
	Type              *string                       `tfsdk:"type"`
	Disks             []vmDiskResourceModel         `tfsdk:"disks"`
	NetworkInterfaces []vmNetworkInterfaceDataModel `tfsdk:"network_interfaces"`
}

type vmNetworkInterfaceDataModel struct {
	Id            string `tfsdk:"id"`
	Name          string `tfsdk:"name"`
	Network       string `tfsdk:"network"`
	Subnet        string `tfsdk:"subnet"`
	InterfaceType string `tfsdk:"interface_type"`
	PrivateIpv4   vmIPv4 `tfsdk:"private_ipv4"`
	PublicIpv4    vmIPv4 `tfsdk:"public_ipv4"`
}

type vmIPv4 struct {
	Address string `tfsdk:"address"`
}

func NewVMDataSource() datasource.DataSource {
	return &vmDataSource{}
}

// Configure adds the provider configured client to the data source.
func (ds *vmDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*swagger.APIClient)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", internal.ErrorMsgProviderInitFailed)

		return
	}

	ds.client = client
}

func (ds *vmDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compute_instance"
}

func (ds *vmDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required: true,
			},
			"name": schema.StringAttribute{
				Optional: true,
			},
			"type": schema.StringAttribute{
				Computed: true,
			},
			"disks": schema.ListNestedAttribute{
				Optional: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Required: true,
						},
					},
				},
			},
			"network_interfaces": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed: true,
						},
						"name": schema.StringAttribute{
							Computed: true,
						},
						"network": schema.StringAttribute{
							Computed: true,
						},
						"subnet": schema.StringAttribute{
							Computed: true,
						},
						"interface_type": schema.StringAttribute{
							Computed: true,
						},
						"public_ipv4": schema.ObjectAttribute{
							Computed: true,
							AttributeTypes: map[string]attr.Type{
								"address": types.StringType,
							},
						},
						"private_ipv4": schema.ObjectAttribute{
							Computed: true,
							AttributeTypes: map[string]attr.Type{
								"address": types.StringType,
							},
						},
					},
				},
			},
		},
	}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *vmDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config vmDataSourceFilter
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state vmDataSourceFilter
	diags = resp.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.ID != nil {
		vm, err := getVM(ctx, ds.client, *config.ID)
		if err != nil {
			resp.Diagnostics.AddError("Failed to get Instance", err.Error())

			return
		}

		state.ID = &vm.Id
		state.Name = &vm.Name
		state.Type = &vm.ProductName
		state.NetworkInterfaces = vmNetworkInterfacesToTerraformDataModel(vm.NetworkInterfaces)

		diags = resp.State.Set(ctx, state)
		resp.Diagnostics.Append(diags...)

		return
	}

	if config.Name != nil {
		// TODO: support fetching instance by name instead of ID once the API provides a utility for this
		resp.Diagnostics.AddError("Not Supported", "Fetching a compute instance by name will be supported in a future release.")

		return
	}

	resp.Diagnostics.AddError("Missing instance identifier", "A compute instance must have an ID or a "+
		"name to be identified.")
}

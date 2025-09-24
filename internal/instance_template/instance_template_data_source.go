package instance_template

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type instanceTemplatesDataSource struct {
	client *swagger.APIClient
}

type instanceTemplatesDataSourceModel struct {
	InstanceTemplates []instanceTemplatesModel `tfsdk:"instance_templates"`
}

type diskModel struct {
	Size string `tfsdk:"size"`
	Type string `tfsdk:"type"`
}

type instanceTemplatesModel struct {
	ID                  string      `tfsdk:"id"`
	Name                string      `tfsdk:"name"`
	ProjectID           string      `tfsdk:"project_id"`
	Type                string      `tfsdk:"type"`
	SSHKey              string      `tfsdk:"ssh_key"`
	Location            string      `tfsdk:"location"`
	ImageName           string      `tfsdk:"image"`
	StartupScript       string      `tfsdk:"startup_script"`
	ShutdownScript      string      `tfsdk:"shutdown_script"`
	PublicIPAddressType string      `tfsdk:"public_ip_address_type"`
	SubnetId            string      `tfsdk:"subnet"`
	IBPartition         string      `tfsdk:"ib_partition"`
	Disks               []diskModel `tfsdk:"disks"`
	PlacementPolicy     string      `tfsdk:"placement_policy"`
	NvlinkDomainID      string      `tfsdk:"nvlink_domain_id"`
}

func NewInstanceTemplatesDataSource() datasource.DataSource {
	return &instanceTemplatesDataSource{}
}

// Configure adds the provider configured client to the data source.
func (ds *instanceTemplatesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (ds *instanceTemplatesDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_instance_templates"
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *instanceTemplatesDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"instance_templates": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed: true,
					},
					"name": schema.StringAttribute{
						Required: true,
					},
					"project_id": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"type": schema.StringAttribute{
						Required: true,
					},
					"ssh_key": schema.StringAttribute{
						Required: true,
					},
					"location": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"image": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"startup_script": schema.StringAttribute{
						Optional: true,
					},
					"shutdown_script": schema.StringAttribute{
						Optional: true,
					},
					"subnet": schema.StringAttribute{
						Required: true,
					},
					"ib_partition": schema.StringAttribute{
						Optional: true,
					},
					"public_ip_address_type": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"disks": schema.ListNestedAttribute{
						Optional: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"size": schema.StringAttribute{
									Required: true,
								},
								"type": schema.StringAttribute{
									Optional: true,
									Computed: true,
								},
							},
						},
					},
					"placement_policy": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"nvlink_domain_id": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
				},
			},
		},
	}}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *instanceTemplatesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	projectID, err := common.GetFallbackProject(ctx, ds.client, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch Instance Templates",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))

		return
	}

	dataResp, httpResp, err := ds.client.InstanceTemplatesApi.ListInstanceTemplates(ctx, projectID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to Fetch Instance Templates", "Could not fetch Instance Template data at this time.")

		return
	}
	defer httpResp.Body.Close()

	disks := make([]diskModel, 0)
	for i := range dataResp.Items {
		for j := range dataResp.Items[i].Disks {
			disks = append(disks, diskModel{
				Size: dataResp.Items[i].Disks[j].Size,
				Type: dataResp.Items[i].Disks[j].Type_,
			})
		}
	}

	var state instanceTemplatesDataSourceModel
	for i := range dataResp.Items {
		state.InstanceTemplates = append(state.InstanceTemplates, instanceTemplatesModel{
			ID:              dataResp.Items[i].Id,
			Name:            dataResp.Items[i].Name,
			Type:            dataResp.Items[i].Type_,
			SSHKey:          dataResp.Items[i].SshPublicKey,
			Location:        dataResp.Items[i].Location,
			ImageName:       dataResp.Items[i].ImageName,
			StartupScript:   dataResp.Items[i].StartupScript,
			ShutdownScript:  dataResp.Items[i].ShutdownScript,
			SubnetId:        dataResp.Items[i].SubnetId,
			IBPartition:     dataResp.Items[i].IbPartitionId,
			ProjectID:       dataResp.Items[i].ProjectId,
			Disks:           disks,
			PlacementPolicy: dataResp.Items[i].PlacementPolicy,
			NvlinkDomainID:  dataResp.Items[i].NvlinkDomainId,
		})
	}

	diags := resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

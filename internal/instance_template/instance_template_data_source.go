package instance_template

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type instanceTemplatesDataSource struct {
	client *common.CrusoeClient
}

type instanceTemplatesDataSourceModel struct {
	ProjectID         types.String             `tfsdk:"project_id"`
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

	client, ok := req.ProviderData.(*common.CrusoeClient)
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
		"project_id": schema.StringAttribute{
			Optional: true,
		},
		"instance_templates": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed: true,
					},
					"name": schema.StringAttribute{
						Computed: true,
					},
					"project_id": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"type": schema.StringAttribute{
						Computed: true,
					},
					"ssh_key": schema.StringAttribute{
						Computed: true,
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
						Computed: true,
					},
					"shutdown_script": schema.StringAttribute{
						Computed: true,
					},
					"subnet": schema.StringAttribute{
						Computed: true,
					},
					"ib_partition": schema.StringAttribute{
						Computed: true,
					},
					"public_ip_address_type": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"disks": schema.ListNestedAttribute{
						Computed: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"size": schema.StringAttribute{
									Computed: true,
								},
								"type": schema.StringAttribute{
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
	var config instanceTemplatesDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := common.GetProjectIDOrFallback(ds.client, config.ProjectID.ValueString())

	dataResp, httpResp, err := ds.client.APIClient.InstanceTemplatesApi.ListInstanceTemplates(ctx, projectID)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to Fetch Instance Templates", "Could not fetch Instance Template data at this time.")

		return
	}

	var state instanceTemplatesDataSourceModel
	state.InstanceTemplates = instanceTemplatesToModel(dataResp.Items)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// instanceTemplatesToModel maps API instance templates to the Terraform model.
// Disks are built per template so each template carries only its own disks.
func instanceTemplatesToModel(items []swagger.InstanceTemplate) []instanceTemplatesModel {
	templates := make([]instanceTemplatesModel, 0, len(items))
	for i := range items {
		disks := make([]diskModel, 0, len(items[i].Disks))
		for j := range items[i].Disks {
			disks = append(disks, diskModel{
				Size: items[i].Disks[j].Size,
				Type: items[i].Disks[j].Type_,
			})
		}

		templates = append(templates, instanceTemplatesModel{
			ID:              items[i].Id,
			Name:            items[i].Name,
			Type:            items[i].Type_,
			SSHKey:          items[i].SshPublicKey,
			Location:        items[i].Location,
			ImageName:       items[i].ImageName,
			StartupScript:   items[i].StartupScript,
			ShutdownScript:  items[i].ShutdownScript,
			SubnetId:        items[i].SubnetId,
			IBPartition:     items[i].IbPartitionId,
			ProjectID:       items[i].ProjectId,
			Disks:           disks,
			PlacementPolicy: items[i].PlacementPolicy,
			NvlinkDomainID:  items[i].NvlinkDomainId,
		})
	}

	// Sort templates deterministically so repeated reads produce a stable ordering.
	common.SortByKeys(templates,
		func(t instanceTemplatesModel) string { return t.Name },
		func(t instanceTemplatesModel) string { return t.ID },
	)

	return templates
}

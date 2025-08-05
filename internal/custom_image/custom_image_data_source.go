package custom_image

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
)

type customImageDataSource struct {
	client     *swagger.APIClient
	basePath   string
	httpClient *http.Client
}

type customImageDataSourceModel struct {
	Name         types.String       `tfsdk:"name"`
	NamePrefix   types.String       `tfsdk:"name_prefix"`
	CustomImages []customImageModel `tfsdk:"custom_images"`
	NewestImage  *customImageModel  `tfsdk:"newest_image"`
}

type customImageModel struct {
	ID          string `tfsdk:"id" json:"id"`
	Name        string `tfsdk:"name" json:"name"`
	Description string `tfsdk:"description" json:"description"`
	Location    string `tfsdk:"location" json:"location"`
	Status      string `tfsdk:"status" json:"status"`
	CreatedAt   string `tfsdk:"created_at" json:"created_at"`
	UpdatedAt   string `tfsdk:"updated_at" json:"updated_at"`
}

func NewCustomImageDataSource() datasource.DataSource {
	return &customImageDataSource{}
}

// Configure adds the provider configured client to the data source.
func (ds *customImageDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, ok := req.ProviderData.(*common.ProviderData)
	if !ok {
		resp.Diagnostics.AddError("Failed to initialize provider", common.ErrorMsgProviderInitFailed)
		return
	}
	ds.client = providerData.APIClient
	ds.basePath = providerData.BasePath
	ds.httpClient = providerData.HTTPClient
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *customImageDataSource) Metadata(ctx context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_compute_custom_image"
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *customImageDataSource) Schema(ctx context.Context, request datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{Attributes: map[string]schema.Attribute{
		"name": schema.StringAttribute{
			Optional:    true,
			Description: "Filter custom images by name. This is a case-sensitive exact match.",
		},
		"name_prefix": schema.StringAttribute{
			Optional:    true,
			Description: "Filter custom images by name prefix. This is case-sensitive and does not require trailing dashes. When multiple matches are found, the most recent one (highest numeric suffix) is selected.",
		},
		"custom_images": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"id":          schema.StringAttribute{Computed: true},
					"name":        schema.StringAttribute{Computed: true},
					"description": schema.StringAttribute{Computed: true},
					"location":    schema.StringAttribute{Computed: true},
					"status":      schema.StringAttribute{Computed: true},
					"created_at":  schema.StringAttribute{Computed: true},
					"updated_at":  schema.StringAttribute{Computed: true},
				},
			},
		},
		"newest_image": schema.SingleNestedAttribute{
			Computed: true,
			Attributes: map[string]schema.Attribute{
				"id":          schema.StringAttribute{Computed: true},
				"name":        schema.StringAttribute{Computed: true},
				"description": schema.StringAttribute{Computed: true},
				"location":    schema.StringAttribute{Computed: true},
				"status":      schema.StringAttribute{Computed: true},
				"created_at":  schema.StringAttribute{Computed: true},
				"updated_at":  schema.StringAttribute{Computed: true},
			},
		},
	}}
}

//nolint:gocritic // Implements Terraform defined interface
func (ds *customImageDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config customImageDataSourceModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID, err := common.GetFallbackProject(ctx, ds.client, &resp.Diagnostics)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch custom images",
			fmt.Sprintf("No project was specified and it was not possible to determine which project to use: %v", err))
		return
	}

	url := fmt.Sprintf("%s/projects/%s/custom-images", ds.basePath, projectID)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to build request", err.Error())
		return
	}

	httpResp, err := ds.httpClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch custom images", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != 200 {
		body, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("Failed to fetch custom images", fmt.Sprintf("Status: %d, Body: %s", httpResp.StatusCode, string(body)))
		return
	}

	var apiResp struct {
		Items []customImageModel `json:"items"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&apiResp); err != nil {
		resp.Diagnostics.AddError("Failed to parse custom images", err.Error())
		return
	}

	// Apply filters
	filteredImages := ds.filterCustomImages(apiResp.Items, config)

	var state customImageDataSourceModel
	state.Name = config.Name
	state.NamePrefix = config.NamePrefix
	state.CustomImages = filteredImages
	state.NewestImage = ds.findNewestImage(filteredImages)

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

// filterCustomImages applies name and name_prefix filters to the list of custom images
func (ds *customImageDataSource) filterCustomImages(images []customImageModel, config customImageDataSourceModel) []customImageModel {
	var filtered []customImageModel

	for _, image := range images {
		// Apply name filter (exact match)
		if !config.Name.IsNull() && !config.Name.IsUnknown() {
			if image.Name != config.Name.ValueString() {
				continue
			}
		}

		// Apply name_prefix filter
		if !config.NamePrefix.IsNull() && !config.NamePrefix.IsUnknown() {
			if !strings.HasPrefix(image.Name, config.NamePrefix.ValueString()) {
				continue
			}
		}

		filtered = append(filtered, image)
	}

	// If using name_prefix and multiple matches found, select the most recent one
	if (!config.NamePrefix.IsNull() && !config.NamePrefix.IsUnknown()) && len(filtered) > 1 {
		// Sort by name in descending order to get the most recent (highest numeric suffix)
		sort.Slice(filtered, func(i, j int) bool {
			return ds.compareImageNames(filtered[i].Name, filtered[j].Name) > 0
		})
		// Return only the most recent match
		return []customImageModel{filtered[0]}
	}

	return filtered
}

// findNewestImage finds the most recent image from a list of custom images
func (ds *customImageDataSource) findNewestImage(images []customImageModel) *customImageModel {
	if len(images) == 0 {
		return nil
	}

	if len(images) == 1 {
		return &images[0]
	}

	// Sort by name in descending order to get the most recent (highest numeric suffix)
	sortedImages := make([]customImageModel, len(images))
	copy(sortedImages, images)
	sort.Slice(sortedImages, func(i, j int) bool {
		return ds.compareImageNames(sortedImages[i].Name, sortedImages[j].Name) > 0
	})

	return &sortedImages[0]
}

// compareImageNames compares two image names and returns:
// -1 if name1 < name2
//
//	0 if name1 == name2
//
// +1 if name1 > name2
func (ds *customImageDataSource) compareImageNames(name1, name2 string) int {
	// Split names into parts
	parts1 := strings.Split(name1, "-")
	parts2 := strings.Split(name2, "-")

	// Find the common prefix
	commonPrefix := ""
	for i := 0; i < len(parts1) && i < len(parts2); i++ {
		if parts1[i] == parts2[i] {
			commonPrefix += parts1[i]
			if i < len(parts1)-1 {
				commonPrefix += "-"
			}
		} else {
			break
		}
	}

	// If names have the same prefix, compare the numeric suffixes
	if commonPrefix != "" && strings.HasPrefix(name1, commonPrefix) && strings.HasPrefix(name2, commonPrefix) {
		suffix1 := strings.TrimPrefix(name1, commonPrefix)
		suffix2 := strings.TrimPrefix(name2, commonPrefix)

		// Try to parse as integers
		num1, err1 := strconv.Atoi(suffix1)
		num2, err2 := strconv.Atoi(suffix2)

		if err1 == nil && err2 == nil {
			// Both are valid numbers, compare numerically
			if num1 < num2 {
				return -1
			} else if num1 > num2 {
				return 1
			} else {
				return 0
			}
		}
	}

	// Fall back to string comparison
	if name1 < name2 {
		return -1
	} else if name1 > name2 {
		return 1
	}
	return 0
}

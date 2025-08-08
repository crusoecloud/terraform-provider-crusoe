package custom_image

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
)

func TestCustomImageDataSource_Schema(t *testing.T) {
	ds := NewCustomImageDataSource()
	resp := &datasource.SchemaResponse{}
	ds.Schema(context.Background(), datasource.SchemaRequest{}, resp)
	if resp.Schema.Attributes == nil {
		t.Fatal("expected schema attributes to be non-nil")
	}
	attrs := resp.Schema.Attributes

	// Check for custom_images attribute
	if _, ok1 := attrs["custom_images"]; !ok1 {
		t.Error("expected custom_images attribute in schema")
	}
	customImagesAttr, ok := attrs["custom_images"].(schema.ListNestedAttribute)
	if !ok {
		t.Error("expected custom_images to be a ListNestedAttribute")
	}
	if !customImagesAttr.Computed {
		t.Error("expected custom_images to be computed")
	}
	fields := customImagesAttr.NestedObject.Attributes
	for _, field := range []string{"id", "name", "description", "locations", "tags", "created_at"} {
		if _, ok2 := fields[field]; !ok2 {
			t.Errorf("expected field %s in custom_images nested object", field)
		}
	}

	// Check for name filter attribute
	if _, ok3 := attrs["name"]; !ok3 {
		t.Error("expected name attribute in schema")
	}
	nameAttr, ok4 := attrs["name"].(schema.StringAttribute)
	if !ok4 {
		t.Error("expected name to be a StringAttribute")
	}
	if !nameAttr.Optional {
		t.Error("expected name to be optional")
	}

	// Check for name_prefix filter attribute
	if _, ok5 := attrs["name_prefix"]; !ok5 {
		t.Error("expected name_prefix attribute in schema")
	}
	namePrefixAttr, ok6 := attrs["name_prefix"].(schema.StringAttribute)
	if !ok6 {
		t.Error("expected name_prefix to be a StringAttribute")
	}
	if !namePrefixAttr.Optional {
		t.Error("expected name_prefix to be optional")
	}

	// Check for newest_image attribute
	if _, ok7 := attrs["newest_image"]; !ok7 {
		t.Error("expected newest_image attribute in schema")
	}
	newestImageAttr, ok8 := attrs["newest_image"].(schema.SingleNestedAttribute)
	if !ok8 {
		t.Error("expected newest_image to be a SingleNestedAttribute")
	}
	if !newestImageAttr.Computed {
		t.Error("expected newest_image to be computed")
	}
	newestImageFields := newestImageAttr.Attributes
	for _, field := range []string{"id", "name", "description", "locations", "tags", "created_at"} {
		if _, ok9 := newestImageFields[field]; !ok9 {
			t.Errorf("expected field %s in newest_image nested object", field)
		}
	}
}

func TestCustomImageDataSource_FilterCustomImages(t *testing.T) {
	// Test data with numeric suffixes
	imagesResp := createMockListImagesResponse(
		"ubuntu-2004", "centos-7", "ubuntu-2204")

	// Test exact name filter
	config := customImageDataSourceModel{
		Name: types.StringValue("ubuntu-2004").ValueStringPointer(),
	}
	filtered := filterCustomImagesListResponse(imagesResp, config)
	if len(filtered) != 1 {
		t.Errorf("expected 1 image, got %d", len(filtered))
	}
	if filtered[0].Name != "ubuntu-2004" {
		t.Errorf("expected ubuntu-2004, got %s", filtered[0].Name)
	}

	// Test name_prefix filter
	config = createNamePrefixFilterConfig("ubuntu")
	filtered = filterCustomImagesListResponse(imagesResp, config)
	if len(filtered) != 2 {
		t.Errorf("expected 2 images, got %d", len(filtered))
	}

	// Test no filters
	config = customImageDataSourceModel{}
	filtered = filterCustomImagesListResponse(imagesResp, config)
	if len(filtered) != 3 {
		t.Errorf("expected 3 images, got %d", len(filtered))
	}
}

func TestCustomImageDataSource_FindNewestImage(t *testing.T) {
	// Test with empty list
	var images []customImageModel
	newest := findNewestImage(images)
	if newest != nil {
		t.Error("expected nil for empty list")
	}

	// Test with single image
	images = []customImageModel{
		{ID: "1", Name: "ubuntu-2004", Description: "Ubuntu 20.04", Locations: []string{"us-east-1"}, Tags: []string{"latest"}, CreatedAt: "2023-01-01"},
	}
	newest = findNewestImage(images)
	if newest == nil {
		t.Error("expected non-nil for single image")
	} else if newest.Name != "ubuntu-2004" {
		t.Errorf("expected ubuntu-2004, got %s", newest.Name)
	}

	// Test with multiple images - should return the most recent based on name comparison
	images = []customImageModel{
		{ID: "1", Name: "test-b200-1230", Description: "Test B200", Locations: []string{"us-east-1"}, Tags: []string{"latest"}, CreatedAt: "2023-01-01"},
		{ID: "2", Name: "test-b200-1234", Description: "Test B200", Locations: []string{"us-east-1"}, Tags: []string{"latest"}, CreatedAt: "2023-01-01"},
		{ID: "3", Name: "test-b200-1220", Description: "Test B200", Locations: []string{"us-east-1"}, Tags: []string{"latest"}, CreatedAt: "2023-01-01"},
	}
	newest = findNewestImage(images)
	if newest == nil {
		t.Error("expected non-nil for multiple images")
	} else if newest.Name != "test-b200-1234" {
		t.Errorf("expected test-b200-1234 (most recent), got %s", newest.Name)
	}
}

func TestCustomImageDataSource_CompareImageNames(t *testing.T) {
	// Test cases for numeric suffix comparison
	testCases := []struct {
		name1    string
		name2    string
		expected int
		desc     string
	}{
		{"test-b200-1234", "test-b200-1230", 1, "higher number should be greater"},
		{"test-b200-1230", "test-b200-1234", -1, "lower number should be less"},
		{"test-b200-1234", "test-b200-1234", 0, "same numbers should be equal"},
		{"ubuntu-20.04", "centos-7", 1, "different prefixes should fall back to name comparison"},
		{"test-b200-1234", "test-b200-1234a", -1, "non-numeric suffix should fall back to name comparison"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := compareImageNames(tc.name1, tc.name2)
			if result != tc.expected {
				t.Errorf("compareImageNames(%q, %q) = %d, expected %d", tc.name1, tc.name2, result, tc.expected)
			}
		})
	}
}

func TestCustomImageDataSource_NamePrefixWithMultipleMatches(t *testing.T) {
	// Test data with multiple matches that should be sorted
	imagesResp := createMockListImagesResponse(
		"test-b200-1230", "test-b200-1234", "test-b200-1220", "ubuntu-20.04")

	// Test name_prefix filter that matches multiple test images (with trailing dash)
	config := createNamePrefixFilterConfig("test-b200-")
	filtered := filterCustomImagesListResponse(imagesResp, config)
	if len(filtered) != 3 {
		t.Errorf("expected 3 images, got %d", len(filtered))
	}

	// Test name_prefix filter that matches multiple test images (without trailing dash)
	config = createNamePrefixFilterConfig("test-b200")
	filtered = filterCustomImagesListResponse(imagesResp, config)
	if len(filtered) != 3 {
		t.Errorf("expected 3 images, got %d", len(filtered))
	}

	// Test name_prefix filter that matches only one image
	config = createNamePrefixFilterConfig("ubuntu")
	filtered = filterCustomImagesListResponse(imagesResp, config)
	if len(filtered) != 1 {
		t.Errorf("expected 1 image, got %d", len(filtered))
	}
	if filtered[0].Name != "ubuntu-20.04" {
		t.Errorf("expected ubuntu-20.04, got %s", filtered[0].Name)
	}

	// Test name_prefix filter that matches no images
	config = createNamePrefixFilterConfig("nonexistent")
	filtered = filterCustomImagesListResponse(imagesResp, config)
	if len(filtered) != 0 {
		t.Errorf("expected 0 images, got %d", len(filtered))
	}
}

func createMockListImagesResponse(imageNames ...string) *swagger.ListImagesResponseV1Alpha5 {
	var images []swagger.Image
	for i, imageName := range imageNames {
		images = append(images, swagger.Image{
			Id:          fmt.Sprintf("%d", i+1),
			Name:        imageName,
			Description: fmt.Sprintf("Test %s", imageName),
			Locations:   []string{"us-east-1"},
			Tags:        []string{"latest"},
			CreatedAt:   "2025-01-01",
		})
	}

	return &swagger.ListImagesResponseV1Alpha5{
		Items: images,
	}
}

func createNamePrefixFilterConfig(namePrefix string) customImageDataSourceModel {
	return customImageDataSourceModel{
		NamePrefix: types.StringValue(namePrefix).ValueStringPointer(),
	}
}

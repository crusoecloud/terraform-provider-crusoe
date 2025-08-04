package custom_image

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
	if _, ok := attrs["custom_images"]; !ok {
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
	for _, field := range []string{"id", "name", "description", "location", "status", "created_at", "updated_at"} {
		if _, ok := fields[field]; !ok {
			t.Errorf("expected field %s in custom_images nested object", field)
		}
	}

	// Check for name filter attribute
	if _, ok := attrs["name"]; !ok {
		t.Error("expected name attribute in schema")
	}
	nameAttr, ok := attrs["name"].(schema.StringAttribute)
	if !ok {
		t.Error("expected name to be a StringAttribute")
	}
	if !nameAttr.Optional {
		t.Error("expected name to be optional")
	}

	// Check for name_prefix filter attribute
	if _, ok := attrs["name_prefix"]; !ok {
		t.Error("expected name_prefix attribute in schema")
	}
	namePrefixAttr, ok := attrs["name_prefix"].(schema.StringAttribute)
	if !ok {
		t.Error("expected name_prefix to be a StringAttribute")
	}
	if !namePrefixAttr.Optional {
		t.Error("expected name_prefix to be optional")
	}
}

func TestCustomImageDataSource_FilterCustomImages(t *testing.T) {
	ds := &customImageDataSource{}

	// Test data
	images := []customImageModel{
		{ID: "1", Name: "ubuntu-20.04", Description: "Ubuntu 20.04", Location: "us-east-1", Status: "available", CreatedAt: "2023-01-01", UpdatedAt: "2023-01-01"},
		{ID: "2", Name: "centos-7", Description: "CentOS 7", Location: "us-east-1", Status: "available", CreatedAt: "2023-01-01", UpdatedAt: "2023-01-01"},
		{ID: "3", Name: "ubuntu-22.04", Description: "Ubuntu 22.04", Location: "us-east-1", Status: "available", CreatedAt: "2023-01-01", UpdatedAt: "2023-01-01"},
	}

	// Test exact name filter
	config := customImageDataSourceModel{
		Name: types.StringValue("ubuntu-20.04"),
	}
	filtered := ds.filterCustomImages(images, config)
	if len(filtered) != 1 {
		t.Errorf("expected 1 image, got %d", len(filtered))
	}
	if filtered[0].Name != "ubuntu-20.04" {
		t.Errorf("expected ubuntu-20.04, got %s", filtered[0].Name)
	}

	// Test name_prefix filter
	config = customImageDataSourceModel{
		NamePrefix: types.StringValue("ubuntu"),
	}
	filtered = ds.filterCustomImages(images, config)
	if len(filtered) != 1 {
		t.Errorf("expected 1 image (most recent), got %d", len(filtered))
	}
	if filtered[0].Name != "ubuntu-22.04" {
		t.Errorf("expected ubuntu-22.04 (most recent), got %s", filtered[0].Name)
	}

	// Test no filters
	config = customImageDataSourceModel{}
	filtered = ds.filterCustomImages(images, config)
	if len(filtered) != 3 {
		t.Errorf("expected 3 images, got %d", len(filtered))
	}
}

func TestCustomImageDataSource_CompareImageNames(t *testing.T) {
	ds := &customImageDataSource{}

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
		{"ubuntu-20.04", "ubuntu-22.04", -1, "string comparison for non-numeric suffixes"},
		{"centos-7", "ubuntu-20.04", -1, "different prefixes"},
		{"test-b200-1234", "test-b200-1234a", -1, "numeric vs non-numeric suffix"},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := ds.compareImageNames(tc.name1, tc.name2)
			if result != tc.expected {
				t.Errorf("compareImageNames(%q, %q) = %d, expected %d", tc.name1, tc.name2, result, tc.expected)
			}
		})
	}
}

func TestCustomImageDataSource_NamePrefixWithMultipleMatches(t *testing.T) {
	ds := &customImageDataSource{}

	// Test data with multiple matches that should be sorted
	images := []customImageModel{
		{ID: "1", Name: "test-b200-1230", Description: "Test B200", Location: "us-east-1", Status: "available", CreatedAt: "2023-01-01", UpdatedAt: "2023-01-01"},
		{ID: "2", Name: "test-b200-1234", Description: "Test B200", Location: "us-east-1", Status: "available", CreatedAt: "2023-01-01", UpdatedAt: "2023-01-01"},
		{ID: "3", Name: "test-b200-1220", Description: "Test B200", Location: "us-east-1", Status: "available", CreatedAt: "2023-01-01", UpdatedAt: "2023-01-01"},
		{ID: "4", Name: "ubuntu-20.04", Description: "Ubuntu 20.04", Location: "us-east-1", Status: "available", CreatedAt: "2023-01-01", UpdatedAt: "2023-01-01"},
	}

	// Test name_prefix filter that matches multiple test images (with trailing dash)
	config := customImageDataSourceModel{
		NamePrefix: types.StringValue("test-b200-"),
	}
	filtered := ds.filterCustomImages(images, config)
	if len(filtered) != 1 {
		t.Errorf("expected 1 image (most recent), got %d", len(filtered))
	}
	if filtered[0].Name != "test-b200-1234" {
		t.Errorf("expected test-b200-1234 (most recent), got %s", filtered[0].Name)
	}

	// Test name_prefix filter that matches multiple test images (without trailing dash)
	config = customImageDataSourceModel{
		NamePrefix: types.StringValue("test-b200"),
	}
	filtered = ds.filterCustomImages(images, config)
	if len(filtered) != 1 {
		t.Errorf("expected 1 image (most recent), got %d", len(filtered))
	}
	if filtered[0].Name != "test-b200-1234" {
		t.Errorf("expected test-b200-1234 (most recent), got %s", filtered[0].Name)
	}

	// Test name_prefix filter that matches only one image
	config = customImageDataSourceModel{
		NamePrefix: types.StringValue("ubuntu"),
	}
	filtered = ds.filterCustomImages(images, config)
	if len(filtered) != 1 {
		t.Errorf("expected 1 image, got %d", len(filtered))
	}
	if filtered[0].Name != "ubuntu-20.04" {
		t.Errorf("expected ubuntu-20.04, got %s", filtered[0].Name)
	}

	// Test name_prefix filter that matches no images
	config = customImageDataSourceModel{
		NamePrefix: types.StringValue("nonexistent"),
	}
	filtered = ds.filterCustomImages(images, config)
	if len(filtered) != 0 {
		t.Errorf("expected 0 images, got %d", len(filtered))
	}

	// Test name_prefix filter with partial prefix
	config = customImageDataSourceModel{
		NamePrefix: types.StringValue("test"),
	}
	filtered = ds.filterCustomImages(images, config)
	if len(filtered) != 1 {
		t.Errorf("expected 1 image (most recent), got %d", len(filtered))
	}
	if filtered[0].Name != "test-b200-1234" {
		t.Errorf("expected test-b200-1234 (most recent), got %s", filtered[0].Name)
	}
}

func TestCustomImageDataSource_Read_Integration(t *testing.T) {
	// This test would require a real Crusoe API endpoint and credentials.
	// It is left as a placeholder for future integration testing.
	t.Skip("Integration test requires real API credentials")
}

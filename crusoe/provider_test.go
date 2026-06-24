package crusoe

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestProviderSchema(t *testing.T) {
	ctx := context.Background()
	p := New()
	schemaResp := &provider.SchemaResponse{}
	p.Schema(ctx, provider.SchemaRequest{}, schemaResp)

	attrs := schemaResp.Schema.Attributes

	// Verify profile attribute exists and is optional
	profileAttr, ok := attrs["profile"].(schema.StringAttribute)
	if !ok {
		t.Fatal("profile attribute not found or wrong type")
	}
	if !profileAttr.Optional {
		t.Error("profile should be Optional")
	}

	// Verify project attribute exists and is optional
	projectAttr, ok := attrs["project"].(schema.StringAttribute)
	if !ok {
		t.Fatal("project attribute not found or wrong type")
	}
	if !projectAttr.Optional {
		t.Error("project should be Optional")
	}

	// Verify api_endpoint attribute still exists and is optional
	apiEndpointAttr, ok := attrs["api_endpoint"].(schema.StringAttribute)
	if !ok {
		t.Fatal("api_endpoint attribute not found or wrong type")
	}
	if !apiEndpointAttr.Optional {
		t.Error("api_endpoint should be Optional")
	}
}

func TestProviderMetadata(t *testing.T) {
	ctx := context.Background()
	p := New()
	metaResp := &provider.MetadataResponse{}
	p.Metadata(ctx, provider.MetadataRequest{}, metaResp)

	if metaResp.TypeName != "crusoe" {
		t.Errorf("Expected TypeName 'crusoe', got %q", metaResp.TypeName)
	}
}

func TestAllResourcesHaveProjectID(t *testing.T) {
	ctx := context.Background()
	p := New()

	// Resources that intentionally don't have project_id
	resourceExclusions := []string{
		"crusoe_project",        // crusoe_project - IS the project itself
		"crusoe_registry_token", // crusoe_registry_token - org-scoped
		"crusoe_storage_s3_key", // org-scoped
	}

	for _, resourceFunc := range p.Resources(ctx) {
		r := resourceFunc()
		metaResp := &resource.MetadataResponse{}
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "crusoe"}, metaResp)

		if slices.Contains(resourceExclusions, metaResp.TypeName) {
			continue
		}

		schemaResp := &resource.SchemaResponse{}
		r.Schema(ctx, resource.SchemaRequest{}, schemaResp)

		_, hasProjectID := schemaResp.Schema.Attributes["project_id"]
		if !hasProjectID {
			t.Errorf("Resource %q missing project_id attribute", metaResp.TypeName)
		}
	}
}

func TestAllDataSourcesHaveProjectID(t *testing.T) {
	ctx := context.Background()
	p := New()

	// Data sources that intentionally don't have project_id
	dataSourceExclusions := []string{
		"crusoe_projects",        // lists all projects
		"crusoe_registry_tokens", // org-scoped
		"crusoe_storage_s3_keys", // org-scoped
	}

	for _, dsFunc := range p.DataSources(ctx) {
		ds := dsFunc()
		metaResp := &datasource.MetadataResponse{}
		ds.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "crusoe"}, metaResp)

		if slices.Contains(dataSourceExclusions, metaResp.TypeName) {
			continue
		}

		schemaResp := &datasource.SchemaResponse{}
		ds.Schema(ctx, datasource.SchemaRequest{}, schemaResp)

		_, hasProjectID := schemaResp.Schema.Attributes["project_id"]
		if !hasProjectID {
			t.Errorf("Data source %q missing project_id attribute", metaResp.TypeName)
		}
	}
}

func TestProviderSchema_Descriptions(t *testing.T) {
	ctx := context.Background()
	p := New()
	schemaResp := &provider.SchemaResponse{}
	p.Schema(ctx, provider.SchemaRequest{}, schemaResp)

	attrs := schemaResp.Schema.Attributes

	tests := []struct {
		attr     string
		contains []string
	}{
		{
			attr:     "api_endpoint",
			contains: []string{"CRUSOE_API_ENDPOINT"},
		},
		{
			attr:     "profile",
			contains: []string{"CRUSOE_PROFILE", "~/.crusoe/config"},
		},
		{
			attr:     "project",
			contains: []string{"CRUSOE_DEFAULT_PROJECT", "project_id"},
		},
	}

	for _, tc := range tests {
		attr, ok := attrs[tc.attr].(schema.StringAttribute)
		if !ok {
			t.Fatalf("%s attribute not found or wrong type", tc.attr)
		}

		desc := attr.MarkdownDescription
		for _, substr := range tc.contains {
			if !strings.Contains(desc, substr) {
				t.Errorf("%s description should mention %q, got: %q", tc.attr, substr, desc)
			}
		}
	}
}

func TestProjectResolution_UUIDDetection(t *testing.T) {
	// Test that valid UUIDs are detected correctly
	validUUID := "550e8400-e29b-41d4-a716-446655440000"
	_, err := uuid.Parse(validUUID)
	if err != nil {
		t.Errorf("Expected valid UUID to parse: %v", err)
	}

	// Test that non-UUIDs are detected correctly (would trigger name lookup)
	nonUUID := "my-project-name"
	_, err = uuid.Parse(nonUUID)
	if err == nil {
		t.Error("Expected non-UUID to fail parsing")
	}

	// Test edge cases
	edgeCases := []struct {
		input   string
		isUUID  bool
		comment string
	}{
		{"", false, "empty string"},
		{"not-a-uuid", false, "hyphenated name"},
		{"12345678-1234-1234-1234-123456789abc", true, "valid UUID"},
		{"12345678-1234-1234-1234-123456789ABC", true, "uppercase UUID"},
		{"project-with-numbers-123", false, "name with numbers"},
	}

	for _, tc := range edgeCases {
		_, err := uuid.Parse(tc.input)
		gotIsUUID := err == nil
		if gotIsUUID != tc.isUUID {
			t.Errorf("UUID detection for %q (%s): got isUUID=%v, want %v",
				tc.input, tc.comment, gotIsUUID, tc.isUUID)
		}
	}
}

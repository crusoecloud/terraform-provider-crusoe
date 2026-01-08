package instance_group

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
)

func TestSchemaFieldConsistency(t *testing.T) {
	ctx := context.Background()

	// Get resource schema
	r := NewInstanceGroupResource()
	resourceSchemaResp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, resourceSchemaResp)

	// Get data source schema
	ds := NewInstanceGroupsDataSource()
	dsSchemaResp := &datasource.SchemaResponse{}
	ds.Schema(ctx, datasource.SchemaRequest{}, dsSchemaResp)

	// Get nested attributes from data source
	instanceGroupsAttr, ok := dsSchemaResp.Schema.Attributes["instance_groups"].(datasourceschema.ListNestedAttribute)
	if !ok {
		t.Fatal("could not get instance_groups nested attribute from data source")
	}

	dsNestedFields := instanceGroupsAttr.NestedObject.Attributes

	// Fields that should exist in both resource and data source nested object
	sharedFields := []string{
		"id",
		"name",
		"instance_template_id",
		"running_instance_count",
		"desired_count",
		"state",
		"project_id",
		"active_instance_ids",
		"inactive_instance_ids",
		"created_at",
		"updated_at",
	}

	for _, field := range sharedFields {
		_, inResource := resourceSchemaResp.Schema.Attributes[field]
		_, inDataSource := dsNestedFields[field]

		if !inResource {
			t.Errorf("field %q missing from resource schema", field)
		}
		if !inDataSource {
			t.Errorf("field %q missing from data source nested schema", field)
		}
	}
}

// Tests for instanceGroupToResourceModel

func TestInstanceGroupToResourceModel(t *testing.T) {
	apiResponse := &swagger.InstanceGroup{
		Id:                   "ig-123",
		ProjectId:            "proj-456",
		Name:                 "test-group",
		TemplateId:           "tmpl-789",
		RunningInstanceCount: 3,
		State:                "HEALTHY",
		DesiredCount:         5,
		CreatedAt:            "2024-01-01T00:00:00Z",
		UpdatedAt:            "2024-01-02T00:00:00Z",
		ActiveInstances:      []string{"vm-1", "vm-2", "vm-3"},
		InactiveInstances:    []string{"vm-4", "vm-5"},
	}

	var state instanceGroupResourceModel
	var diags diag.Diagnostics

	instanceGroupToResourceModel(apiResponse, &state, &diags)

	if diags.HasError() {
		t.Fatalf("mapping returned errors: %v", diags)
	}

	// Verify all fields are mapped correctly
	if state.ID.ValueString() != "ig-123" {
		t.Errorf("ID: expected 'ig-123', got %q", state.ID.ValueString())
	}
	if state.ProjectID.ValueString() != "proj-456" {
		t.Errorf("ProjectID: expected 'proj-456', got %q", state.ProjectID.ValueString())
	}
	if state.Name.ValueString() != "test-group" {
		t.Errorf("Name: expected 'test-group', got %q", state.Name.ValueString())
	}
	if state.InstanceTemplateID.ValueString() != "tmpl-789" {
		t.Errorf("InstanceTemplateID: expected 'tmpl-789', got %q", state.InstanceTemplateID.ValueString())
	}
	if state.RunningInstanceCount.ValueInt64() != 3 {
		t.Errorf("RunningInstanceCount: expected 3, got %d", state.RunningInstanceCount.ValueInt64())
	}
	if state.State.ValueString() != "HEALTHY" {
		t.Errorf("State: expected 'HEALTHY', got %q", state.State.ValueString())
	}
	if state.DesiredCount.ValueInt64() != 5 {
		t.Errorf("DesiredCount: expected 5, got %d", state.DesiredCount.ValueInt64())
	}
	if state.CreatedAt.ValueString() != "2024-01-01T00:00:00Z" {
		t.Errorf("CreatedAt: expected '2024-01-01T00:00:00Z', got %q", state.CreatedAt.ValueString())
	}
	if state.UpdatedAt.ValueString() != "2024-01-02T00:00:00Z" {
		t.Errorf("UpdatedAt: expected '2024-01-02T00:00:00Z', got %q", state.UpdatedAt.ValueString())
	}

	// Verify list fields
	activeIDs, _ := state.ActiveInstanceIDs.ToListValue(context.Background())
	if len(activeIDs.Elements()) != 3 {
		t.Errorf("ActiveInstanceIDs: expected 3 elements, got %d", len(activeIDs.Elements()))
	}

	inactiveIDs, _ := state.InactiveInstanceIDs.ToListValue(context.Background())
	if len(inactiveIDs.Elements()) != 2 {
		t.Errorf("InactiveInstanceIDs: expected 2 elements, got %d", len(inactiveIDs.Elements()))
	}
}

func TestInstanceGroupToResourceModel_EmptyLists(t *testing.T) {
	apiResponse := &swagger.InstanceGroup{
		Id:                   "ig-123",
		ProjectId:            "proj-456",
		Name:                 "empty-group",
		TemplateId:           "tmpl-789",
		RunningInstanceCount: 0,
		State:                "HEALTHY",
		DesiredCount:         0,
		CreatedAt:            "2024-01-01T00:00:00Z",
		UpdatedAt:            "2024-01-01T00:00:00Z",
		ActiveInstances:      []string{},
		InactiveInstances:    []string{},
	}

	var state instanceGroupResourceModel
	var diags diag.Diagnostics

	instanceGroupToResourceModel(apiResponse, &state, &diags)

	if diags.HasError() {
		t.Fatalf("mapping returned errors: %v", diags)
	}

	// Verify empty lists are handled correctly
	activeIDs, _ := state.ActiveInstanceIDs.ToListValue(context.Background())
	if len(activeIDs.Elements()) != 0 {
		t.Errorf("ActiveInstanceIDs: expected 0 elements, got %d", len(activeIDs.Elements()))
	}

	inactiveIDs, _ := state.InactiveInstanceIDs.ToListValue(context.Background())
	if len(inactiveIDs.Elements()) != 0 {
		t.Errorf("InactiveInstanceIDs: expected 0 elements, got %d", len(inactiveIDs.Elements()))
	}
}

func TestInstanceGroupToResourceModel_NilLists(t *testing.T) {
	apiResponse := &swagger.InstanceGroup{
		Id:                   "ig-123",
		ProjectId:            "proj-456",
		Name:                 "nil-lists-group",
		TemplateId:           "tmpl-789",
		RunningInstanceCount: 0,
		State:                "UPDATING",
		DesiredCount:         0,
		CreatedAt:            "2024-01-01T00:00:00Z",
		UpdatedAt:            "2024-01-01T00:00:00Z",
		ActiveInstances:      nil,
		InactiveInstances:    nil,
	}

	var state instanceGroupResourceModel
	var diags diag.Diagnostics

	instanceGroupToResourceModel(apiResponse, &state, &diags)

	if diags.HasError() {
		t.Fatalf("mapping returned errors: %v", diags)
	}

	// Verify nil lists produce empty lists, not null
	if state.ActiveInstanceIDs.IsNull() {
		t.Error("ActiveInstanceIDs should not be null for nil input")
	}
	activeIDs, _ := state.ActiveInstanceIDs.ToListValue(context.Background())
	if len(activeIDs.Elements()) != 0 {
		t.Errorf("ActiveInstanceIDs: expected 0 elements for nil input, got %d", len(activeIDs.Elements()))
	}

	if state.InactiveInstanceIDs.IsNull() {
		t.Error("InactiveInstanceIDs should not be null for nil input")
	}
	inactiveIDs, _ := state.InactiveInstanceIDs.ToListValue(context.Background())
	if len(inactiveIDs.Elements()) != 0 {
		t.Errorf("InactiveInstanceIDs: expected 0 elements for nil input, got %d", len(inactiveIDs.Elements()))
	}
}

// Tests for instanceGroupToDataSourceModel

func TestInstanceGroupToDataSourceModel(t *testing.T) {
	apiItem := &swagger.InstanceGroup{
		Id:                   "ig-123",
		ProjectId:            "proj-456",
		Name:                 "test-group",
		TemplateId:           "tmpl-789",
		RunningInstanceCount: 3,
		State:                "RUNNING",
		DesiredCount:         5,
		CreatedAt:            "2024-01-01T00:00:00Z",
		UpdatedAt:            "2024-01-02T00:00:00Z",
		ActiveInstances:      []string{"vm-1", "vm-2", "vm-3"},
		InactiveInstances:    []string{"vm-4", "vm-5"},
	}

	result := instanceGroupToDataSourceModel(apiItem)

	// Verify all fields are mapped correctly
	if result.ID != "ig-123" {
		t.Errorf("ID: expected 'ig-123', got %q", result.ID)
	}
	if result.ProjectID != "proj-456" {
		t.Errorf("ProjectID: expected 'proj-456', got %q", result.ProjectID)
	}
	if result.Name != "test-group" {
		t.Errorf("Name: expected 'test-group', got %q", result.Name)
	}
	if result.InstanceTemplateID != "tmpl-789" {
		t.Errorf("InstanceTemplateID: expected 'tmpl-789', got %q", result.InstanceTemplateID)
	}
	if result.RunningInstanceCount != 3 {
		t.Errorf("RunningInstanceCount: expected 3, got %d", result.RunningInstanceCount)
	}
	if result.State != "RUNNING" {
		t.Errorf("State: expected 'RUNNING', got %q", result.State)
	}
	if result.DesiredCount != 5 {
		t.Errorf("DesiredCount: expected 5, got %d", result.DesiredCount)
	}
	if result.CreatedAt != "2024-01-01T00:00:00Z" {
		t.Errorf("CreatedAt: expected '2024-01-01T00:00:00Z', got %q", result.CreatedAt)
	}
	if result.UpdatedAt != "2024-01-02T00:00:00Z" {
		t.Errorf("UpdatedAt: expected '2024-01-02T00:00:00Z', got %q", result.UpdatedAt)
	}
	if len(result.ActiveInstanceIDs) != 3 {
		t.Errorf("ActiveInstanceIDs: expected 3 elements, got %d", len(result.ActiveInstanceIDs))
	}
	if len(result.InactiveInstanceIDs) != 2 {
		t.Errorf("InactiveInstanceIDs: expected 2 elements, got %d", len(result.InactiveInstanceIDs))
	}
}

func TestInstanceGroupToDataSourceModel_EmptyLists(t *testing.T) {
	apiItem := &swagger.InstanceGroup{
		Id:                   "ig-123",
		ProjectId:            "proj-456",
		Name:                 "empty-group",
		TemplateId:           "tmpl-789",
		RunningInstanceCount: 0,
		DesiredCount:         0,
		State:                "STOPPED",
		CreatedAt:            "2024-01-01T00:00:00Z",
		UpdatedAt:            "2024-01-01T00:00:00Z",
		ActiveInstances:      []string{},
		InactiveInstances:    []string{},
	}

	result := instanceGroupToDataSourceModel(apiItem)

	if len(result.ActiveInstanceIDs) != 0 {
		t.Errorf("ActiveInstanceIDs: expected 0 elements, got %d", len(result.ActiveInstanceIDs))
	}
	if len(result.InactiveInstanceIDs) != 0 {
		t.Errorf("InactiveInstanceIDs: expected 0 elements, got %d", len(result.InactiveInstanceIDs))
	}
}

func TestInstanceGroupToDataSourceModel_NilLists(t *testing.T) {
	apiItem := &swagger.InstanceGroup{
		Id:                   "ig-123",
		ProjectId:            "proj-456",
		Name:                 "nil-lists-group",
		TemplateId:           "tmpl-789",
		RunningInstanceCount: 0,
		DesiredCount:         0,
		State:                "STOPPED",
		CreatedAt:            "2024-01-01T00:00:00Z",
		UpdatedAt:            "2024-01-01T00:00:00Z",
		ActiveInstances:      nil,
		InactiveInstances:    nil,
	}

	result := instanceGroupToDataSourceModel(apiItem)

	// nil slices should produce zero-length results
	if len(result.ActiveInstanceIDs) != 0 {
		t.Errorf("ActiveInstanceIDs: expected 0 elements for nil input, got %d", len(result.ActiveInstanceIDs))
	}
	if len(result.InactiveInstanceIDs) != 0 {
		t.Errorf("InactiveInstanceIDs: expected 0 elements for nil input, got %d", len(result.InactiveInstanceIDs))
	}
}

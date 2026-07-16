package project

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

// Test_projectToResourceModel verifies the shared transform sources id/name from the API
// project, overwriting whatever the model previously held (so Update reflects the
// API response rather than the plan).
func Test_projectToResourceModel(t *testing.T) {
	model := &projectResourceModel{
		ID:   types.StringValue("planned-id"),
		Name: types.StringValue("planned-name"),
	}
	project := &swagger.Project{Id: "proj-1", Name: "api-name"}

	projectToResourceModel(project, model)

	if got := model.ID.ValueString(); got != "proj-1" {
		t.Errorf("id = %q, want %q", got, "proj-1")
	}
	if got := model.Name.ValueString(); got != "api-name" {
		t.Errorf("name = %q, want %q (from API)", got, "api-name")
	}
}

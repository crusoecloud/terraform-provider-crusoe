package project

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
)

// projectToResourceModel maps an API project onto the resource model. All CRUD
// paths use it so id/name are always sourced from the API response.
func projectToResourceModel(project *swagger.Project, projectModel *projectResourceModel) {
	projectModel.ID = types.StringValue(project.Id)
	projectModel.Name = types.StringValue(project.Name)
}

// getUserOrg returns the organization id for the authenticated user.
func getUserOrg(ctx context.Context, apiClient *swagger.APIClient) (string, error) {
	dataResp, httpResp, err := apiClient.EntitiesApi.GetOrganizations(ctx)
	if httpResp != nil {
		defer httpResp.Body.Close()
	}
	if err != nil {
		return "", err
	}

	entities := dataResp.Items
	switch len(entities) {
	case 0:
		return "", errors.New("user does not belong to any organizations")
	case 1:
		return entities[0].Id, nil
	default:
		return "", errors.New("user belongs to multiple organizations")
	}
}

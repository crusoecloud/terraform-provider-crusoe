package project

import (
	"context"
	"errors"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
)

// getUserOrg returns the organization id for the authenticated user.
func getUserOrg(ctx context.Context, apiClient *swagger.APIClient) (string, error) {
	dataResp, httpResp, err := apiClient.EntitiesApi.GetOrganizations(ctx)
	if err != nil {
		return "", err
	}
	defer httpResp.Body.Close()

	entities := dataResp.Entities
	switch len(entities) {
	case 0:
		return "", errors.New("user does not belong to any organizations")
	case 1:
		return entities[0].Id, nil
	default:
		return "", errors.New("user belongs to multiple organizations")
	}
}

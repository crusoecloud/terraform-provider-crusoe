package disk

import (
	"context"
	"errors"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha4"
)

// getDisk fetches a storage disk by ID.
func getDisk(ctx context.Context, apiClient *swagger.APIClient, diskID string) (*swagger.Disk, error) {
	dataResp, httpResp, err := apiClient.DisksApi.GetDisk(ctx, diskID)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	for i := range dataResp.Disks {
		if dataResp.Disks[i].Id == diskID {
			return &dataResp.Disks[i], nil
		}
	}

	return nil, errors.New("failed to fetch disk with matching ID")
}

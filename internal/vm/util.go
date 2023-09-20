package vm

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha4"
)

const StateRunning = "STATE_RUNNING"

// getDisksDiff compares the disks attached to two VM resource models and returns
// a diff of disks defined by disk ID.
func getDisksDiff(origDisks, newDisks []vmDiskResourceModel) (disksAdded, disksRemoved []string) {
	for _, newDisk := range newDisks {
		matched := false
		for _, origDisk := range origDisks {
			if newDisk.ID == origDisk.ID {
				matched = true

				break
			}
		}
		if !matched {
			disksAdded = append(disksAdded, newDisk.ID)
		}
	}

	for _, origDisk := range origDisks {
		matched := false
		for _, newDisk := range newDisks {
			if newDisk.ID == origDisk.ID {
				matched = true

				break
			}
		}
		if !matched {
			disksRemoved = append(disksRemoved, origDisk.ID)
		}
	}

	return disksAdded, disksRemoved
}

func getVM(ctx context.Context, apiClient *swagger.APIClient, vmID string) (*swagger.InstanceV1Alpha4, error) {
	dataResp, httpResp, err := apiClient.VMsApi.GetInstance(ctx, vmID)
	if err != nil {
		return nil, fmt.Errorf("failed to find VM: %w", err)
	}
	defer httpResp.Body.Close()

	if dataResp.Instance != nil {
		return dataResp.Instance, nil
	}

	return nil, fmt.Errorf("failed to find VM with matching ID: %w", err)
}

// vmNetworkInterfacesToTerraformDataModel creates a slice of Terraform-compatible network
// interface datasource instances from Crusoe API network interfaces.
//
// In the case that a warning is returned because IP addresses are missing - which should never
// be the case - we still return a partial response that should be usable.
func vmNetworkInterfacesToTerraformDataModel(networkInterfaces []swagger.NetworkInterface) (interfaces []vmNetworkInterfaceDataModel, warning string) {
	for _, networkInterface := range networkInterfaces {
		var publicIP string
		var privateIP string
		if len(networkInterface.Ips) != 0 {
			warning = "At least one network interface is missing IP addresses. Please reach out to support@crusoeenergy.com" +
				" and let us know."
		} else {
			publicIP = networkInterface.Ips[0].PrivateIpv4.Address
			privateIP = networkInterface.Ips[0].PrivateIpv4.Address
		}

		interfaces = append(interfaces, vmNetworkInterfaceDataModel{
			Id:            networkInterface.Id,
			Name:          networkInterface.Name,
			Network:       networkInterface.Network,
			Subnet:        networkInterface.Subnet,
			InterfaceType: networkInterface.InterfaceType,
			PrivateIpv4:   vmIPv4{Address: privateIP},
			PublicIpv4:    vmIPv4{Address: publicIP},
		})
	}

	return interfaces, warning
}

// vmNetworkInterfacesToTerraformResourceModel creates a slice of Terraform-compatible network
// interface resource instances from Crusoe API network interfaces.
func vmNetworkInterfacesToTerraformResourceModel(networkInterfaces []swagger.NetworkInterface) []vmNetworkInterfaceResourceModel {
	interfaces := make([]vmNetworkInterfaceResourceModel, 0, len(networkInterfaces))
	for _, networkInterface := range networkInterfaces {
		interfaces = append(interfaces, vmNetworkInterfaceResourceModel{
			ID:            types.StringValue(networkInterface.Id),
			Name:          types.StringValue(networkInterface.Name),
			Network:       types.StringValue(networkInterface.Network),
			Subnet:        types.StringValue(networkInterface.Subnet),
			InterfaceType: types.StringValue(networkInterface.InterfaceType),
			PrivateIpv4: types.ObjectValueMust(
				map[string]attr.Type{"address": types.StringType},
				map[string]attr.Value{"address": types.StringValue(networkInterface.Ips[0].PrivateIpv4.Address)},
			),
			PublicIpv4: vmPublicIPv4ResourceModel{
				ID:      types.StringValue(networkInterface.Ips[0].PublicIpv4.Id),
				Address: types.StringValue(networkInterface.Ips[0].PublicIpv4.Address),
				Type:    types.StringValue(networkInterface.Ips[0].PublicIpv4.Type_),
			},
		})
	}

	return interfaces
}

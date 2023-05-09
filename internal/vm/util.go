package vm

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	swagger "gitlab.com/crusoeenergy/island/external/client-go/swagger/v1alpha4"
)

// reusable type attributes definition for a VM's network interface
var vmNetworkTypeAttributes = map[string]attr.Type{
	"id":             types.StringType,
	"name":           types.StringType,
	"network":        types.StringType,
	"subnet":         types.StringType,
	"interface_type": types.StringType,
	"private_ipv4": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"address": types.StringType,
		},
	},
	"public_ipv4": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"address": types.StringType,
		},
	},
}

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

const vmStateShutOff = "STATE_SHUTOFF"

// TODO: update once we support an API endpoint to fetch a single VM by ID
func getVM(ctx context.Context, apiClient *swagger.APIClient, vmID string) (*swagger.InstanceV1Alpha4, error) {
	dataResp, httpResp, err := apiClient.VMsApi.GetInstances(ctx)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	for i := range dataResp.Instances {
		if dataResp.Instances[i].Id == vmID {
			return &dataResp.Instances[i], nil
		}
	}

	return nil, errors.New("failed to find VM with matching ID")
}

// vmNetworkInterfacesToTerraformModel creates a slice of Terraform compatible network
// interfaces from Crusoe API network interfaces.
func vmNetworkInterfacesToTerraformModel(networkInterfaces []swagger.NetworkInterface) []vmNetworkInterfaceModel {
	interfaces := make([]vmNetworkInterfaceModel, 0, len(networkInterfaces))
	for _, networkInterface := range networkInterfaces {
		interfaces = append(interfaces, vmNetworkInterfaceModel{
			Id:            networkInterface.Id,
			Name:          networkInterface.Name,
			Network:       networkInterface.Network,
			Subnet:        networkInterface.Subnet,
			InterfaceType: networkInterface.InterfaceType,
			PrivateIpv4: vmIPv4{
				Address: networkInterface.Ips[0].PrivateIpv4.Address,
			},
			PublicIpv4: vmIPv4{
				Address: networkInterface.Ips[0].PublicIpv4.Address,
			},
		})
	}

	return interfaces
}

package vm

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha4"
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
			warning = "At least one network interface is missing IP addresses. Please reach out support@crusoeenergy.com" +
				" and let us know this."
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
			PrivateIpv4: vmIPv4ResourceModel{
				Address: types.StringValue(networkInterface.Ips[0].PrivateIpv4.Address),
			},
			PublicIpv4: vmIPv4ResourceModel{
				Address: types.StringValue(networkInterface.Ips[0].PublicIpv4.Address),
			},
		})
	}

	return interfaces
}

package vm

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha4"
	"github.com/crusoecloud/terraform-provider-crusoe/internal"
)

const StateRunning = "STATE_RUNNING"

var vmNetworkInterfaceSchema = types.ObjectType{
	AttrTypes: map[string]attr.Type{
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
				"id":      types.StringType,
				"address": types.StringType,
				"type":    types.StringType,
			},
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

func getVM(ctx context.Context, apiClient *swagger.APIClient, vmID string) (*swagger.InstanceV1Alpha4, error) {
	dataResp, httpResp, err := apiClient.VMsApi.GetInstance(ctx, vmID)
	if err != nil {
		return nil, fmt.Errorf("failed to find VM: %w", internal.UnpackAPIError(err))
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
		if len(networkInterface.Ips) == 0 {
			warning = "At least one network interface is missing IP addresses. Please reach out to support@crusoecloud.com" +
				" and let us know."
		} else {
			publicIP = networkInterface.Ips[0].PublicIpv4.Address
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
func vmNetworkInterfacesToTerraformResourceModel(networkInterfaces []swagger.NetworkInterface) (networkInterfacesList types.List, warning string) {
	interfaces := make([]vmNetworkInterfaceResourceModel, 0, len(networkInterfaces))
	for _, networkInterface := range networkInterfaces {
		var publicIP swagger.PublicIpv4Address
		var privateIP swagger.PrivateIpv4Address
		if len(networkInterface.Ips) == 0 {
			warning = "At least one network interface is missing IP addresses. Please reach out to support@crusoecloud.com" +
				" and let us know."
		} else {
			publicIP = *networkInterface.Ips[0].PublicIpv4
			privateIP = *networkInterface.Ips[0].PrivateIpv4
		}

		interfaces = append(interfaces, vmNetworkInterfaceResourceModel{
			ID:            types.StringValue(networkInterface.Id),
			Name:          types.StringValue(networkInterface.Name),
			Network:       types.StringValue(networkInterface.Network),
			Subnet:        types.StringValue(networkInterface.Subnet),
			InterfaceType: types.StringValue(networkInterface.InterfaceType),
			PrivateIpv4: types.ObjectValueMust(
				map[string]attr.Type{"address": types.StringType},
				map[string]attr.Value{"address": types.StringValue(privateIP.Address)},
			),
			PublicIpv4: vmPublicIPv4ResourceModel{
				ID:      types.StringValue(publicIP.Id),
				Address: types.StringValue(publicIP.Address),
				Type:    types.StringValue(publicIP.Type_),
			},
		})
	}

	values, _ := types.ListValueFrom(context.Background(), vmNetworkInterfaceSchema, interfaces)

	return values, warning
}

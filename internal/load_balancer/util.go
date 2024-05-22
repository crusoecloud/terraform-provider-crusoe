package load_balancer

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1alpha5"
)

var loadBalancerNetworkInterfaceSchema = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"network_id": types.StringType,
		"subnet_id":  types.StringType,
	},
}

var loadBalancerIPAddressSchema = types.ObjectType{
	AttrTypes: map[string]attr.Type{
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

var loadBalancerHealthCheckSchema = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"timeout": types.StringType,
		"port":  types.StringType,
		"interval":  types.StringType,
		"success_count":  types.StringType,
		"failure_count":  types.StringType,
	},
}

func loadBalancerNetworkInterfacesToTerraformDataModel(networkInterfaces []swagger.LoadBalancerNetworkInterface,
) (lbNetworkInterfaces []networkInterfaceModel) {
	interfaces := make([]networkInterfaceModel, 0, len(networkInterfaces))
	for _, networkInterface := range networkInterfaces {
		interfaces = append(interfaces, networkInterfaceModel{
			NetworkID: networkInterface.NetworkId,
			SubnetID:  networkInterface.SubnetId,
		})
	}

	return interfaces
}

func loadBalancerDestinationsToTerraformDataModel(destinations []swagger.NetworkTarget,
) (lbDestinations []destinationModel) {
	lbDestinations = make([]destinationModel, 0, len(destinations))
	for _, destination := range destinations {
		lbDestinations = append(lbDestinations, destinationModel{
			Cidr:       destination.Cidr,
			ResourceID: destination.ResourceId,
		})
	}

	return lbDestinations
}

func loadBalancerIPsToTerraformDataModel(ips []swagger.IpAddresses,
) (lbIPs []ipAddressesModel) {
	lbIPs = make([]ipAddressesModel, 0, len(ips))
	for _, ip := range ips {
		lbIPs = append(lbIPs, ipAddressesModel{
			PrivateIPv4: lbIPv4{Address: ip.PrivateIpv4.Address},
			PublicIpv4:  lbIPv4{Address: ip.PublicIpv4.Address},
		})
	}

	return lbIPs
}

func loadBalancerNetworkInterfacesToTerraformResourceModel(networkInterfaces []swagger.LoadBalancerNetworkInterface,
) (lbNetworkInterfaces types.List, diags diag.Diagnostics) {
	interfaces := make([]loadBalancerNetworkInterfaceModel, 0, len(networkInterfaces))
	for _, networkInterface := range networkInterfaces {
		interfaces = append(interfaces, loadBalancerNetworkInterfaceModel{
			NetworkID: types.StringValue(networkInterface.NetworkId),
			SubnetID:  types.StringValue(networkInterface.SubnetId),
		})
	}

	lbNetworkInterfaces, diags = types.ListValueFrom(context.Background(), loadBalancerNetworkInterfaceSchema, interfaces)

	return lbNetworkInterfaces, diags
}

func loadBalancerIPsToTerraformResourceModel(ips []swagger.IpAddresses,
) (lbIPs types.List, diags diag.Diagnostics) {
	ipList := make([]loadBalancerIPAddressModel, 0, len(ips))
	for _, ipAddress := range ips {
		ipList = append(ipList, loadBalancerIPAddressModel{
			PublicIpv4: loadBalancerPublicIPv4ResourceModel{
				ID:      types.StringValue(ipAddress.PublicIpv4.Id),
				Address: types.StringValue(ipAddress.PublicIpv4.Address),
				Type:    types.StringValue(ipAddress.PublicIpv4.Type_),
			},
			PrivateIPv4: types.ObjectValueMust(
				map[string]attr.Type{"address": types.StringType},
				map[string]attr.Value{"address": types.StringValue(ipAddress.PrivateIpv4.Address)},
			),
		})
	}

	lbIPs, diags = types.ListValueFrom(context.Background(), loadBalancerIPAddressSchema, ipList)

	return lbIPs, diags
}

func loadBalancerHealthCheckToTerraformResourceModel(healthCheck *swagger.HealthCheckOptions,
) (lbHealthCheck *healthCheckOptionsResourceModel) {
	lbHealthCheck = &healthCheckOptionsResourceModel{
		Timeout:      types.StringValue(healthCheck.Timeout),
		Port:         types.StringValue(healthCheck.Port),
		Interval:     types.StringValue(healthCheck.Interval),
		SuccessCount: types.StringValue(healthCheck.SuccessCount),
		FailureCount: types.StringValue(healthCheck.FailureCount),
	}

	return lbHealthCheck
}

func loadBalancerUpdateTerraformState(ctx context.Context, lb *swagger.LoadBalancer, state *loadBalancerResourceModel) {
	state.ID = types.StringValue(lb.Id)
	state.Name = types.StringValue(lb.Name)
	state.Location = types.StringValue(lb.Location)
	state.Algorithm = types.StringValue(lb.Algorithm)
	state.Type = types.StringValue(lb.Type_)
	destinations, _ := types.ListValueFrom(context.Background(), types.StringType, lb.Destinations)
	state.Destinations = destinations
	protocols, _ := types.ListValueFrom(context.Background(), types.StringType, lb.Protocols)
	state.Protocols = protocols
	networkInterfaces, _ := loadBalancerNetworkInterfacesToTerraformResourceModel(lb.NetworkInterfaces)
	state.NetworkInterfaces = networkInterfaces
	state.HealthCheck, _ = types.ObjectValueFrom(ctx, loadBalancerHealthCheckSchema.AttrTypes, loadBalancerHealthCheckToTerraformResourceModel(lb.HealthCheck))
	state.IPs, _ = loadBalancerIPsToTerraformResourceModel(lb.Ips)
}

package load_balancer

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec (LoadBalancer,
// plus the nested LoadBalancerNetworkInterface, NetworkTarget, HealthCheckOptions,
// PublicIpv4Address, and PrivateIpv4Address models).
const (
	apiDescID           = "ID of the load balancer."
	apiDescName         = "Name of the load balancer."
	apiDescLocation     = "Location of the load balancer."
	apiDescAlgorithm    = "Load balancing algorithm used to distribute traffic across destinations (for example, `random`)."
	apiDescType         = "Type of the load balancer (for example, `internal_ipv4`)."
	apiDescProtocols    = "Network protocols the load balancer handles. Possible values: `tcp`, `udp`."
	apiDescDestinations = "Backend targets the load balancer forwards traffic to, given as CIDR blocks or resource IDs."
	apiDescIPs          = "IP addresses assigned to the load balancer."

	// network_interfaces + nested (LoadBalancerNetworkInterface). The json tags are
	// literally `network`/`subnet`, but the spec describes each as an ID.
	apiDescNetworkInterfaces = "Network interfaces the load balancer is attached to."
	apiDescNetwork           = "ID of the VPC network for the interface."
	apiDescSubnet            = "ID of the subnet for the interface."

	// destinations nested (NetworkTarget).
	apiDescCidr       = "CIDR block, or an IP address that is converted to a CIDR. Mutually exclusive with resource_id."
	apiDescResourceID = "ID of a backend resource. Mutually exclusive with cidr."

	// ips nested public_ipv4 (PublicIpv4Address) and private_ipv4 (PrivateIpv4Address)
	// leaf attributes.
	apiDescPublicIPv4Address  = "Public IPv4 address."
	apiDescPublicIPv4ID       = "ID of the public IPv4 address."
	apiDescPublicIPv4Type     = "Allocation type of the public IPv4 address (for example, `dynamic`)."
	apiDescPrivateIPv4Address = "Private IPv4 address."

	// health_check nested (HealthCheckOptions).
	apiDescHealthCheckTimeout      = "Timeout for a health check response, in seconds."
	apiDescHealthCheckPort         = "Port on which to perform health checks."
	apiDescHealthCheckInterval     = "Interval between health checks, in seconds."
	apiDescHealthCheckSuccessCount = "Number of successful checks required to consider a backend healthy."
	apiDescHealthCheckFailureCount = "Number of allowed failures before considering a backend unhealthy."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	// The LoadBalancer read model has no project_id property, so the base text is
	// provider-authored here rather than sourced from the spec.
	providerDescProjectID     = "ID of the project the load balancer belongs to. " + project.ProviderDescProjectIDFallback
	providerDescLoadBalancers = "List of load balancers in the project."
)

var loadBalancerNetworkInterfaceSchema = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"network": types.StringType,
		"subnet":  types.StringType,
	},
}

var loadBalancerNetworkTargetSchema = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"cidr":        types.StringType,
		"resource_id": types.StringType,
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
		"timeout":       types.StringType,
		"port":          types.StringType,
		"interval":      types.StringType,
		"success_count": types.StringType,
		"failure_count": types.StringType,
	},
}

func loadBalancerNetworkInterfacesToTerraformDataModel(networkInterfaces []swagger.LoadBalancerNetworkInterface,
) (lbNetworkInterfaces []networkInterfaceModel) {
	interfaces := make([]networkInterfaceModel, 0, len(networkInterfaces))
	for _, networkInterface := range networkInterfaces {
		interfaces = append(interfaces, networkInterfaceModel{
			Network: networkInterface.Network,
			Subnet:  networkInterface.Subnet,
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
			PublicIPv4:  lbIPv4{Address: ip.PublicIpv4.Address},
		})
	}

	return lbIPs
}

func loadBalancerNetworkInterfacesToTerraformResourceModel(networkInterfaces []swagger.LoadBalancerNetworkInterface,
) (lbNetworkInterfaces types.List, diags diag.Diagnostics) {
	interfaces := make([]loadBalancerNetworkInterfaceModel, 0, len(networkInterfaces))
	for _, networkInterface := range networkInterfaces {
		interfaces = append(interfaces, loadBalancerNetworkInterfaceModel{
			Network: types.StringValue(networkInterface.Network),
			Subnet:  types.StringValue(networkInterface.Subnet),
		})
	}

	lbNetworkInterfaces, diags = types.ListValueFrom(context.Background(), loadBalancerNetworkInterfaceSchema, interfaces)

	return lbNetworkInterfaces, diags
}

func loadBalancerDestinationsToTerraformResourceModel(networkTargets []swagger.NetworkTarget,
) (destinations types.List, diags diag.Diagnostics) {
	lbNetworkTargets := make([]loadBalancerNetworkTargetModel, 0, len(networkTargets))
	for _, networkTarget := range networkTargets {
		lbNetworkTargets = append(lbNetworkTargets, loadBalancerNetworkTargetModel{
			Cidr:       types.StringValue(networkTarget.Cidr),
			ResourceID: types.StringValue(networkTarget.ResourceId),
		})
	}

	destinations, diags = types.ListValueFrom(context.Background(), loadBalancerNetworkTargetSchema, lbNetworkTargets)

	return destinations, diags
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

// healthCheckToSwagger decodes the health_check object attribute into a swagger
// HealthCheckOptions request payload, returning nil when it is null or unknown.
// The attribute must be decoded into the tfsdk-tagged resource model first — the
// swagger type only has json tags, so decoding directly into it silently yields an
// empty struct.
func healthCheckToSwagger(ctx context.Context, obj types.Object, diags *diag.Diagnostics) *swagger.HealthCheckOptions {
	if obj.IsNull() || obj.IsUnknown() {
		return nil
	}

	var model healthCheckOptionsResourceModel
	diags.Append(obj.As(ctx, &model, basetypes.ObjectAsOptions{})...)

	return &swagger.HealthCheckOptions{
		Timeout:      model.Timeout.ValueString(),
		Port:         model.Port.ValueString(),
		Interval:     model.Interval.ValueString(),
		SuccessCount: model.SuccessCount.ValueString(),
		FailureCount: model.FailureCount.ValueString(),
	}
}

func loadBalancerUpdateTerraformState(ctx context.Context, lb *swagger.LoadBalancer, state *loadBalancerResourceModel) {
	state.ID = types.StringValue(lb.Id)
	state.Name = types.StringValue(lb.Name)
	state.Location = types.StringValue(lb.Location)
	state.Algorithm = types.StringValue(lb.Algorithm)
	state.Type = types.StringValue(lb.Type_)
	state.Destinations, _ = loadBalancerDestinationsToTerraformResourceModel(lb.Destinations)
	protocols, _ := types.ListValueFrom(context.Background(), types.StringType, lb.Protocols)
	state.Protocols = protocols
	networkInterfaces, _ := loadBalancerNetworkInterfacesToTerraformResourceModel(lb.NetworkInterfaces)
	state.NetworkInterfaces = networkInterfaces
	state.HealthCheck, _ = types.ObjectValueFrom(ctx, loadBalancerHealthCheckSchema.AttrTypes, loadBalancerHealthCheckToTerraformResourceModel(lb.HealthCheck))
	state.IPs, _ = loadBalancerIPsToTerraformResourceModel(lb.Ips)
}

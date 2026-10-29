package load_balancer

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec (ExternalLoadBalancer,
// plus the nested ListenPortAndBackend, Backend, and HealthCheckOptionsExternalLB models).
const (
	apiDescID       = "ID of the External Load Balancer."
	apiDescName     = "Name of the External Load Balancer."
	apiDescLocation = "Location of the External Load Balancer."
	apiDescProtocol = "Protocol of the External Load Balancer."
	apiDescVIP      = "Virtual IP (VIP) of the External Load Balancer."
	apiDescVPCID    = "ID of the VPC network this External Load Balancer belongs to."

	apiDescListenPortsAndBackends = "Listen ports and backends configuration."
	apiDescListenPort             = "Port on which the load balancer listens."
	apiDescBackends               = "List of backend configurations."
	apiDescBackendIP              = "IP address of the backend server."
	apiDescBackendPort            = "Port on which the backend server listens."

	apiDescHealthCheckTimeout      = "Timeout for health check responses (in seconds)."
	apiDescHealthCheckInterval     = "Interval between health checks (in seconds)."
	apiDescHealthCheckSuccessCount = "Number of successful checks required to consider the backend healthy."
	apiDescHealthCheckFailureCount = "Number of allowed failures before considering the backend unhealthy."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project the External Load Balancer belongs to. " +
		project.ProviderDescProjectIDFallback
	providerDescProtocol           = "Possible values: `tcp`, `udp`."
	providerDescHealthCheckOptions = "Populated by the API when not specified."
	providerDescBackendIP          = "Must be the private IP of an existing VM in the project."
)

const protocolPrefix = "LOAD_BALANCER_PROTOCOL_"

var backendObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"ip":   types.StringType,
		"port": types.Int64Type,
	},
}

var listenPortAndBackendObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"listen_port": types.Int64Type,
		"backends":    types.SetType{ElemType: backendObjectType},
	},
}

var healthCheckOptionsObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"timeout":       types.Int64Type,
		"interval":      types.Int64Type,
		"success_count": types.Int64Type,
		"failure_count": types.Int64Type,
	},
}

// shortProtocol converts the API's fully-qualified protocol into the short form used in
// configuration, matching what the crusoe CLI accepts.
func shortProtocol(protocol string) string {
	upper := strings.ToUpper(strings.TrimSpace(protocol))

	return strings.ToLower(strings.TrimPrefix(upper, protocolPrefix))
}

func listenPortsAndBackendsToResourceModel(ctx context.Context, items []swagger.ListenPortAndBackend,
) (result types.Set, diags diag.Diagnostics) {
	entries := make([]listenPortAndBackendResourceModel, 0, len(items))
	for _, item := range items {
		backends := make([]backendResourceModel, 0, len(item.Backends))
		for _, backend := range item.Backends {
			backends = append(backends, backendResourceModel{
				IP:   types.StringValue(backend.Ip),
				Port: types.Int64Value(backend.Port),
			})
		}

		backendSet, backendDiags := types.SetValueFrom(ctx, backendObjectType, backends)
		diags.Append(backendDiags...)

		entries = append(entries, listenPortAndBackendResourceModel{
			ListenPort: types.Int64Value(item.ListenPort),
			Backends:   backendSet,
		})
	}

	result, setDiags := types.SetValueFrom(ctx, listenPortAndBackendObjectType, entries)
	diags.Append(setDiags...)

	return result, diags
}

// listenPortsAndBackendsToSwagger decodes the attribute into a request payload. The attribute
// must be decoded into the tfsdk-tagged model first — the swagger types only have json tags,
// so decoding straight into them silently yields empty structs.
func listenPortsAndBackendsToSwagger(ctx context.Context, set types.Set, diags *diag.Diagnostics,
) []swagger.ListenPortAndBackend {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}

	var entries []listenPortAndBackendResourceModel
	diags.Append(set.ElementsAs(ctx, &entries, false)...)

	items := make([]swagger.ListenPortAndBackend, 0, len(entries))
	for _, entry := range entries {
		var backendModels []backendResourceModel
		if !entry.Backends.IsNull() && !entry.Backends.IsUnknown() {
			diags.Append(entry.Backends.ElementsAs(ctx, &backendModels, false)...)
		}

		backends := make([]swagger.Backend, 0, len(backendModels))
		for _, backend := range backendModels {
			backends = append(backends, swagger.Backend{
				Ip:   backend.IP.ValueString(),
				Port: backend.Port.ValueInt64(),
			})
		}

		items = append(items, swagger.ListenPortAndBackend{
			ListenPort: entry.ListenPort.ValueInt64(),
			Backends:   backends,
		})
	}

	return items
}

func healthCheckOptionsToSwagger(ctx context.Context, obj types.Object, diags *diag.Diagnostics,
) *swagger.HealthCheckOptionsExternalLb {
	if obj.IsNull() || obj.IsUnknown() {
		return nil
	}

	var model healthCheckOptionsResourceModel
	diags.Append(obj.As(ctx, &model, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return nil
	}

	return &swagger.HealthCheckOptionsExternalLb{
		Timeout:      model.Timeout.ValueInt64(),
		Interval:     model.Interval.ValueInt64(),
		SuccessCount: model.SuccessCount.ValueInt64(),
		FailureCount: model.FailureCount.ValueInt64(),
	}
}

func healthCheckOptionsToResourceModel(ctx context.Context, options *swagger.HealthCheckOptionsExternalLb,
) (types.Object, diag.Diagnostics) {
	if options == nil {
		return types.ObjectNull(healthCheckOptionsObjectType.AttrTypes), nil
	}

	return types.ObjectValueFrom(ctx, healthCheckOptionsObjectType.AttrTypes, healthCheckOptionsResourceModel{
		Timeout:      types.Int64Value(options.Timeout),
		Interval:     types.Int64Value(options.Interval),
		SuccessCount: types.Int64Value(options.SuccessCount),
		FailureCount: types.Int64Value(options.FailureCount),
	})
}

// loadBalancerToResourceModel maps a GET response into Terraform state.
func loadBalancerToResourceModel(ctx context.Context, lb *swagger.ExternalLoadBalancer,
	state *loadBalancerResourceModel, diags *diag.Diagnostics,
) {
	state.ID = types.StringValue(lb.Id)
	state.ProjectID = types.StringValue(lb.ProjectId)
	state.Name = types.StringValue(lb.Name)
	state.Location = types.StringValue(lb.Location)
	state.VPCID = types.StringValue(lb.VpcId)
	state.Protocol = types.StringValue(shortProtocol(lb.Protocol))
	state.VIP = types.StringValue(lb.Vip)

	listenPorts, listenDiags := listenPortsAndBackendsToResourceModel(ctx, lb.ListenPortsAndBackends)
	diags.Append(listenDiags...)
	state.ListenPortsAndBackends = listenPorts

	healthCheck, healthDiags := healthCheckOptionsToResourceModel(ctx, lb.HealthCheckOptions)
	diags.Append(healthDiags...)
	state.HealthCheckOptions = healthCheck
}

// ilbDesc* and the loadBalancer*ToTerraformDataModel helpers below still serve the internal
// load balancer data source. They are removed in the follow-up MR that repoints it.
const (
	ilbDescID                      = "ID of the load balancer."
	ilbDescName                    = "Name of the load balancer."
	ilbDescLocation                = "Location of the load balancer."
	ilbDescHealthCheckTimeout      = "Timeout for a health check response, in seconds."
	ilbDescHealthCheckInterval     = "Interval between health checks, in seconds."
	ilbDescHealthCheckSuccessCount = "Number of successful checks required to consider a backend healthy."
	ilbDescHealthCheckFailureCount = "Number of allowed failures before considering a backend unhealthy."

	ilbDescAlgorithm          = "Load balancing algorithm used to distribute traffic across destinations (for example, `random`)."
	ilbDescType               = "Type of the load balancer (for example, `internal_ipv4`)."
	ilbDescProtocols          = "Network protocols the load balancer handles. Possible values: `tcp`, `udp`."
	ilbDescDestinations       = "Backend targets the load balancer forwards traffic to, given as CIDR blocks or resource IDs."
	ilbDescIPs                = "IP addresses assigned to the load balancer."
	ilbDescNetworkInterfaces  = "Network interfaces the load balancer is attached to."
	ilbDescNetwork            = "ID of the VPC network for the interface."
	ilbDescSubnet             = "ID of the subnet for the interface."
	ilbDescCidr               = "CIDR block, or an IP address that is converted to a CIDR. Mutually exclusive with resource_id."
	ilbDescResourceID         = "ID of a backend resource. Mutually exclusive with cidr."
	ilbDescPublicIPv4Address  = "Public IPv4 address."
	ilbDescPrivateIPv4Address = "Private IPv4 address."
	ilbDescHealthCheckPort    = "Port on which to perform health checks."

	ilbProviderDescProjectID     = "ID of the project the load balancer belongs to. " + project.ProviderDescProjectIDFallback
	ilbProviderDescLoadBalancers = "List of load balancers in the project."
)

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

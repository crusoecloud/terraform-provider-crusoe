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
	state.HealthCheck = loadBalancerHealthCheckToTerraformResourceModel(lb.HealthCheck)
}

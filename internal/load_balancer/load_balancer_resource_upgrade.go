package load_balancer

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

const upgradeNotSupportedDetail = "crusoe_load_balancer now manages external load balancers. " +
	"Prior state refers to an internal load balancer, which is a different API resource and cannot be migrated. " +
	"Run `terraform state rm <address>` for the affected resources, then re-create them."

// UpgradeState rejects state written by the internal load balancer implementation. Prior IDs
// belong to a different API and would 404, so there is nothing to migrate. Only v1 is handled:
// the resource was introduced at schema version 1, so no v0 state exists.
func (r *loadBalancerResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		1: {
			StateUpgrader: func(_ context.Context, _ resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				resp.Diagnostics.AddError("Cannot upgrade crusoe_load_balancer state", upgradeNotSupportedDetail)
			},
		},
	}
}

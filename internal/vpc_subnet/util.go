package vpc_subnet

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec (VpcSubnet;
// nested NatGateway; nat_gateway_enabled from VpcSubnetPostRequest).
const (
	apiDescID                = "ID of the VPC subnet."
	apiDescName              = "Name of the VPC subnet."
	apiDescCIDR              = "Address range of the VPC subnet, in CIDR notation."
	apiDescLocation          = "Location of the VPC subnet."
	apiDescNetwork           = "ID of the VPC network that the subnet belongs to."
	apiDescNATGatewayEnabled = "Whether to create a NAT gateway for the subnet."
	apiDescNATGateways       = "NAT gateways attached to the subnet. Empty unless a NAT gateway is enabled for the subnet."

	apiDescNATGatewayID                = "ID of the NAT gateway."
	apiDescNATGatewayPublicIPv4Address = "Public IPv4 address assigned to the NAT gateway."
	apiDescNATGatewayPublicIPv4ID      = "ID of the public IPv4 address assigned to the NAT gateway."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project the VPC subnet belongs to. " + project.ProviderDescProjectIDFallback
)

const (
	natGatewayPollInterval = 2 * time.Second
	natGatewayPollTimeout  = 5 * time.Minute
)

var errNATGatewayTimeout = errors.New("timed out waiting for the NAT gateway to reach the requested state")

type getVPCSubnetFunc func(ctx context.Context, projectID, subnetID string) (swagger.VpcSubnet, *http.Response, error)

var vpcSubnetNatGatewaySchema = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"id":                  types.StringType,
		"public_ipv4_address": types.StringType,
		"public_ipv4_id":      types.StringType,
	},
}

func findVpcSubnet(ctx context.Context, client *swagger.APIClient, vpcSubnetID string) (*swagger.VpcSubnet, string, error) {
	args := common.FindResourceArgs[swagger.VpcSubnet]{
		ResourceID:  vpcSubnetID,
		GetResource: client.VPCSubnetsApi.GetVPCSubnet,
		IsResource: func(subnet swagger.VpcSubnet, id string) bool {
			return subnet.Id == id
		},
	}

	return common.FindResource[swagger.VpcSubnet](ctx, client, args)
}

func natGatewayPresent(vpcSubnet *swagger.VpcSubnet) bool {
	return len(vpcSubnet.NatGateways) > 0
}

// awaitNATGatewayState polls the subnet until its NAT gateway list matches wantEnabled.
// The update operation can succeed before the NAT gateway is attached or removed, and
// state written from that result disagrees with the plan.
func awaitNATGatewayState(ctx context.Context, get getVPCSubnetFunc, projectID, subnetID string,
	wantEnabled bool, interval, timeout time.Duration,
) (*swagger.VpcSubnet, error) {
	deadline := time.Now().Add(timeout)

	for {
		vpcSubnet, httpResp, err := get(ctx, projectID, subnetID)
		if httpResp != nil {
			httpResp.Body.Close()
		}
		if err != nil {
			return nil, fmt.Errorf("error getting VPC subnet %s: %w", subnetID, err)
		}

		if natGatewayPresent(&vpcSubnet) == wantEnabled {
			return &vpcSubnet, nil
		}

		if time.Now().After(deadline) {
			return nil, errNATGatewayTimeout
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

func vpcSubnetToTerraformResourceModel(ctx context.Context, vpcSubnet *swagger.VpcSubnet, state *vpcSubnetResourceModel, diags *diag.Diagnostics) {
	state.ID = types.StringValue(vpcSubnet.Id)
	state.Name = types.StringValue(vpcSubnet.Name)
	state.CIDR = types.StringValue(vpcSubnet.Cidr)
	state.Location = types.StringValue(vpcSubnet.Location)
	state.Network = types.StringValue(vpcSubnet.VpcNetworkId)
	natGatewaysList, natDiags := natGatewaysToTerraformResourceModel(ctx, vpcSubnet.NatGateways)
	state.NATGateways = natGatewaysList
	state.NATGatewayEnabled = types.BoolValue(len(natGatewaysList.Elements()) > 0)
	diags.Append(natDiags...)
}

func natGatewaysToTerraformResourceModel(ctx context.Context, natGateways []swagger.NatGateway) (types.List, diag.Diagnostics) {
	gateways := make([]vpcSubnetNatGatewayResourceModel, 0, len(natGateways))
	for _, gateway := range natGateways {
		gateways = append(gateways, vpcSubnetNatGatewayResourceModel{
			ID:                types.StringValue(gateway.Id),
			PublicIpv4Address: types.StringValue(gateway.PublicIpv4Address),
			PublicIpv4Id:      types.StringValue(gateway.PublicIpv4Id),
		})
	}

	// Sort by ID for deterministic ordering; the API does not guarantee a stable
	// order for the (Computed) NAT gateway list.
	common.SortByKeys(gateways, func(g vpcSubnetNatGatewayResourceModel) string { return g.ID.ValueString() })

	return types.ListValueFrom(ctx, vpcSubnetNatGatewaySchema, gateways)
}

// natGatewaysUseStateUnlessToggledModifier keeps the prior nat_gateways value across
// updates unless nat_gateway_enabled changes. The stock UseStateForUnknown copies the
// prior list through a toggle, and the apply then fails because the API returns a
// different list. Keeping the prior value on other updates leaves references to the
// NAT gateway address known.
type natGatewaysUseStateUnlessToggledModifier struct{}

func natGatewaysUseStateUnlessToggled() planmodifier.List {
	return natGatewaysUseStateUnlessToggledModifier{}
}

func (m natGatewaysUseStateUnlessToggledModifier) Description(_ context.Context) string {
	return "Once the subnet exists, the value of this attribute in state is preserved unless nat_gateway_enabled changes."
}

func (m natGatewaysUseStateUnlessToggledModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

//nolint:gocritic // hugeParam: req signature required by planmodifier.List interface
func (m natGatewaysUseStateUnlessToggledModifier) PlanModifyList(ctx context.Context,
	req planmodifier.ListRequest, resp *planmodifier.ListResponse,
) {
	if req.State.Raw.IsNull() {
		return
	}

	if !req.PlanValue.IsUnknown() {
		return
	}

	var planEnabled, stateEnabled types.Bool
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("nat_gateway_enabled"), &planEnabled)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("nat_gateway_enabled"), &stateEnabled)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if planEnabled.IsUnknown() || !planEnabled.Equal(stateEnabled) {
		return
	}

	resp.PlanValue = req.StateValue
}

package kubernetes_cluster

import (
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/common"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec
// (KubernetesCluster definition; oidc_* fields from OidcAuthConfig).
const (
	apiDescID                    = "ID of the Kubernetes cluster."
	apiDescName                  = "Name of the Kubernetes cluster."
	apiDescVersion               = "Version of the Crusoe Kubernetes image the cluster runs."
	apiDescSubnetID              = "ID of the subnet the Kubernetes cluster belongs to."
	apiDescClusterCidr           = "Range of IP addresses allocated to pods scheduled on worker nodes, in CIDR notation."
	apiDescNodeCidrMaskSize      = "Mask size for the cluster CIDR."
	apiDescServiceClusterIPRange = "Range of IP addresses allocated to Kubernetes services, in CIDR notation."
	apiDescAddOns                = "Add-ons associated with the cluster."
	apiDescLocation              = "Location of the Kubernetes cluster."
	apiDescDNSName               = "DNS name of the cluster."
	apiDescNodePoolIDs           = "IDs of the node pools within the Kubernetes cluster."
	apiDescPrivate               = "Whether the cluster is private (without a public IP)."

	apiDescApiserverExtraArgs         = "Extra arguments passed to the kube-apiserver control plane component."
	apiDescSchedulerExtraArgs         = "Extra arguments passed to the kube-scheduler control plane component."
	apiDescControllerManagerExtraArgs = "Extra arguments passed to the kube-controller-manager control plane component."

	apiDescOIDCIssuerURL      = "URL of the OpenID Connect issuer."
	apiDescOIDCClientID       = "Client ID for the OpenID Connect client."
	apiDescOIDCUsernameClaim  = "Claim used to identify the user. Defaults to `sub`."
	apiDescOIDCUsernamePrefix = "Prefix added before the username to avoid name conflicts."
	apiDescOIDCGroupsClaim    = "Claim used to identify the user's groups."
	apiDescOIDCCACert         = "PEM-encoded certificate authority certificate used to validate the OIDC server's certificate."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project that owns the Kubernetes cluster. " + project.ProviderDescProjectIDFallback

	// providerDescExtraArgsNote is appended (resource-side only) to the *_extra_args
	// descriptions to document Crusoe-specific update behavior.
	providerDescExtraArgsNote = "Changes take effect after a cluster rotation. To clear args, use the Crusoe CLI."
)

// clusterToResourceModel maps an API Kubernetes cluster onto model, with the API
// object as the source of truth. Create, Read, and Update all call it so their
// mappings cannot drift.
//
// Two field groups are caller-owned rather than API-derived and come from ref (the
// plan in Create/Update, the prior state in Read):
//   - The OIDC* fields are not returned by the API (they are RequiresReplace inputs).
//   - The *_extra_args maps are resolved against ref so a field the user never set
//     stays null instead of becoming an empty map when the API echoes {}.
//
// nodepool_ids is sorted so its Computed, API-ordered value is stable across reads.
// ref and model may be the same pointer.
func clusterToResourceModel(cluster *swagger.KubernetesCluster, ref, model *kubernetesClusterResourceModel, diags *diag.Diagnostics) {
	model.ID = types.StringValue(cluster.Id)
	model.ProjectID = types.StringValue(cluster.ProjectId)
	model.Name = types.StringValue(cluster.Name)
	model.Version = types.StringValue(cluster.Version)
	model.SubnetID = types.StringValue(cluster.SubnetId)
	model.NodeCidrMaskSize = types.Int64Value(int64(cluster.NodeCidrMaskSize))
	model.ClusterCidr = types.StringValue(cluster.ClusterCidr)
	model.ServiceClusterIpRange = types.StringValue(cluster.ServiceClusterIpRange)
	model.Location = types.StringValue(cluster.Location)
	model.DNSName = types.StringValue(cluster.DnsName)
	model.Private = types.BoolValue(cluster.Private)

	addOns, d := common.StringSliceToTFList(cluster.AddOns)
	diags.Append(d...)
	model.AddOns = addOns

	nodePoolIDs, d := common.StringSliceToTFList(sortedNodePools(cluster.NodePools))
	diags.Append(d...)
	model.NodePoolIds = nodePoolIDs

	model.OIDCIssuerURL = ref.OIDCIssuerURL
	model.OIDCClientID = ref.OIDCClientID
	model.OIDCUsernameClaim = ref.OIDCUsernameClaim
	model.OIDCUsernamePrefix = ref.OIDCUsernamePrefix
	model.OIDCGroupsClaim = ref.OIDCGroupsClaim
	model.OIDCCACert = ref.OIDCCACert

	apiserver, d := resolveExtraArg(ref.ApiserverExtraArgs, cluster.ApiserverExtraArgs)
	diags.Append(d...)
	model.ApiserverExtraArgs = apiserver

	scheduler, d := resolveExtraArg(ref.SchedulerExtraArgs, cluster.SchedulerExtraArgs)
	diags.Append(d...)
	model.SchedulerExtraArgs = scheduler

	controllerManager, d := resolveExtraArg(ref.ControllerManagerExtraArgs, cluster.ControllerManagerExtraArgs)
	diags.Append(d...)
	model.ControllerManagerExtraArgs = controllerManager
}

// sortedNodePools returns the node pool IDs in deterministic (lexical) order. The
// list API does not guarantee a stable order for these opaque IDs, so sorting
// prevents spurious diffs on the Computed nodepool_ids. The input slice is not mutated.
func sortedNodePools(nodePools []string) []string {
	sorted := append([]string(nil), nodePools...)
	slices.Sort(sorted)

	return sorted
}

package kubeconfig

import (
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec
// (KubernetesAuthenticationDetails definition; auth_type from the
// getClusterCredentials operation's query parameters).
const (
	apiDescClusterID            = "ID of the Kubernetes cluster."
	apiDescClusterAddress       = "Address of the Kubernetes cluster to authenticate to."
	apiDescClusterCACertificate = "CA Certificate of the Kubernetes cluster to authenticate to."
	apiDescClusterName          = "Name of the Kubernetes cluster to authenticate to."
	apiDescClientCertificate    = "User's Client certificate for authenticating to the cluster."
	apiDescClientKey            = "The private key associated with the user's Client certificate."
	apiDescUserName             = "Name of the authenticating user."
	apiDescKubeConfigYaml       = "Kubeconfig of the Kubernetes cluster to authenticate to."
	apiDescAuthType             = "Authentication type for fetching kubeconfig. Possible values: `admin_cert`, `oidc`."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID      = "ID of the project the Kubernetes cluster belongs to. " + project.ProviderDescProjectIDFallback
	providerDescAuthTypeSuffix = "If unset, defaults to `admin_cert`."
)

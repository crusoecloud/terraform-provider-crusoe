package transport_network

import (
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec.
//
// The spec has no TransportNetwork definition: this data source reads the
// IBNetwork model (nested IBNetworkCapacity), so the text below is that model's
// text verbatim. It therefore still says "InfiniBand network", which is what the
// API returns for both IB and RoCE fabrics.
const (
	apiDescID                = "ID of the InfiniBand network."
	apiDescName              = "Name of the InfiniBand network."
	apiDescLocation          = "Location of the InfiniBand network."
	apiDescCapacities        = "Available capacity in the network, broken down by VM slice type."
	apiDescCapacityQuantity  = "Number of slices of the given slice type."
	apiDescCapacitySliceType = "VM slice type the capacity applies to."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project the transport network belongs to. " + project.ProviderDescProjectIDFallback

	providerDescTransportNetworks = "Transport networks available to the project."
)

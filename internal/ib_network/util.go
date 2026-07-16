package ib_network

import (
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec (IbNetwork; nested IbNetworkCapacity).
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
	providerDescProjectID = "ID of the project the InfiniBand network belongs to. " + project.ProviderDescProjectIDFallback
)

package transport_partition

import (
	"github.com/hashicorp/terraform-plugin-framework/types"

	swagger "github.com/crusoecloud/client-go/swagger/v1"
	"github.com/crusoecloud/terraform-provider-crusoe/internal/project"
)

// apiDesc* — schema descriptions derived from the client-go swagger spec.
//
// The spec has no TransportPartition definition: this resource reads the
// IBPartition model, so the text below is that model's text verbatim. It
// therefore still says "InfiniBand partition", which is what the API returns for
// both IB and RoCE fabrics.
const (
	apiDescID                 = "ID of the InfiniBand partition."
	apiDescName               = "Name of the InfiniBand partition."
	apiDescTransportNetworkID = "ID of the InfiniBand network the partition belongs to."
)

// providerDesc* — provider-specific schema descriptions (Terraform-side; not from the spec).
const (
	providerDescProjectID = "ID of the project the transport partition belongs to. " + project.ProviderDescProjectIDFallback
)

func transportPartitionToTerraformResourceModel(transportPartition *swagger.IbPartition, state *transportPartitionResourceModel) {
	state.ID = types.StringValue(transportPartition.Id)
	state.Name = types.StringValue(transportPartition.Name)
	state.TransportNetworkID = types.StringValue(transportPartition.IbNetworkId)
}

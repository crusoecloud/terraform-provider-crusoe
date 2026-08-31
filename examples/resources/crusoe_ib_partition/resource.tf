# This resource is deprecated, use crusoe_transport_partition instead.

data "crusoe_ib_networks" "example" {}

resource "crusoe_ib_partition" "example" {
  name          = "my-ib-partition"
  ib_network_id = data.crusoe_ib_networks.example.ib_networks[0].id
}

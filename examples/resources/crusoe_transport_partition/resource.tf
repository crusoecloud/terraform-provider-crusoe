data "crusoe_transport_networks" "example" {}

resource "crusoe_transport_partition" "example" {
  name                 = "my-transport-partition"
  transport_network_id = data.crusoe_transport_networks.example.transport_networks[0].id
}

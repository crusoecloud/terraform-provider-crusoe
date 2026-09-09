# This resource is deprecated, use crusoe_transport_partition instead.

data "crusoe_ib_networks" "example" {}

resource "crusoe_ib_partition" "example" {
  name          = "my-ib-partition"
  ib_network_id = data.crusoe_ib_networks.example.ib_networks[0].id
}

# To migrate an existing partition, change the resource type and add a moved
# block. Terraform keeps the partition in place. See the "Migrating to the
# transport resources" guide. Needs Terraform 1.8 or later and provider v1.4.0
# or later.
#
# resource "crusoe_transport_partition" "example" {
#   name                 = "my-ib-partition"
#   transport_network_id = data.crusoe_transport_networks.example.transport_networks[0].id
# }
#
# moved {
#   from = crusoe_ib_partition.example
#   to   = crusoe_transport_partition.example
# }

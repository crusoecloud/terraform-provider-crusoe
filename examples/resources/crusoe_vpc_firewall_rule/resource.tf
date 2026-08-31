resource "crusoe_vpc_network" "example" {
  name = "my-vpc-network"
  cidr = "10.0.0.0/8"
}

resource "crusoe_vpc_firewall_rule" "example" {
  network   = crusoe_vpc_network.example.id
  name      = "my-firewall-rule"
  action    = "allow"
  direction = "ingress"
  protocols = "tcp"

  # Each source and destination sets either a cidr or a resource_id, never both.
  sources           = [{ cidr = "0.0.0.0/0" }]
  source_ports      = "1-65535"
  destinations      = [{ resource_id = crusoe_vpc_network.example.id }]
  destination_ports = "443"
}

resource "crusoe_vpc_network" "example" {
  name = "my-vpc-network"
  cidr = "10.0.0.0/8"
}

resource "crusoe_vpc_firewall_rule" "example_tcp" {
  network   = crusoe_vpc_network.example.id
  name      = "my-tcp-firewall-rule"
  action    = "allow"
  direction = "ingress"
  protocols = "tcp"

  # Each source and destination sets either a cidr or a resource_id, never both.
  sources                 = [{ cidr = "0.0.0.0/0" }]
  source_port_ranges      = ["1-65535"]
  destinations            = [{ resource_id = crusoe_vpc_network.example.id }]
  destination_port_ranges = ["443"]
}

# An icmp rule has no ports. Omit the source and destination port fields.
resource "crusoe_vpc_firewall_rule" "example_icmp" {
  network   = crusoe_vpc_network.example.id
  name      = "my-icmp-firewall-rule"
  action    = "allow"
  direction = "ingress"
  protocols = "icmp"

  sources      = [{ cidr = "0.0.0.0/0" }]
  destinations = [{ resource_id = crusoe_vpc_network.example.id }]
}

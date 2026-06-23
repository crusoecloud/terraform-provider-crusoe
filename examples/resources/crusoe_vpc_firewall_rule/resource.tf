resource "crusoe_vpc_network" "example" {
  name = "my-vpc-network"
  cidr = "10.0.0.0/8"
}

resource "crusoe_vpc_firewall_rule" "example" {
  network           = crusoe_vpc_network.example.id
  name              = "my-firewall-rule"
  action            = "allow"
  direction         = "ingress"
  protocols         = "tcp"
  source            = "0.0.0.0/0"
  source_ports      = "1-65535"
  destination       = crusoe_vpc_network.example.cidr
  destination_ports = "443"
}

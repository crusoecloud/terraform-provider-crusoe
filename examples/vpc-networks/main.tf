terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

resource "crusoe_vpc_network" "my_vpc_network" {
  name = "my-new-network"
  cidr = "10.0.0.0/8"
}

resource "crusoe_vpc_subnet" "my_vpc_subnet" {
  name = "my-new-subnet"
  cidr = "10.0.0.0/16"
  location = "us-northcentral1-a"
  network = crusoe_vpc_network.my_vpc_network.id
}

// firewall rule
resource "crusoe_vpc_firewall_rule" "open_fw_rule" {
  network           = crusoe_vpc_network.my_vpc_network.id
  name              = "example-terraform-rule"
  action            = "allow"
  direction         = "ingress"
  protocols         = "tcp"
  source            = "0.0.0.0/0"
  source_ports      = "1-65535"
  destination       = crusoe_vpc_network.my_vpc_network.cidr
  destination_ports = "1-65535"

  // It is currently not possible to create subnets and firewall rules at the same time.
  // This directive should be specified when creating firewall rules and subnets
  // to avoid failures.
  depends_on = [crusoe_vpc_subnet.my_vpc_subnet]
}

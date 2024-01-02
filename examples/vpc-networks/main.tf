terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

resource "crusoe_vpc_network" "my_vpc_network" {
  name = "change-name"
  cidr = "10.0.0.0/8"
}

resource "crusoe_vpc_subnet" "my_vpc_subnet" {
  name = "change-name"
  cidr = "10.0.0.0/16"
  location = "us-northcentralstaging1-a"
  network = crusoe_vpc_network.my_vpc_network.id
}

// firewall rule
// note: this allows all ingress over TCP to our VM
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
}

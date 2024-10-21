terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

locals {
  my_ssh_key = file("~/.ssh/id_ed25519.pub")
}

resource "crusoe_vpc_network" "my_vpc_network" {
  name = "my-new-network"
  cidr = "10.0.0.0/8"
}

resource "crusoe_vpc_subnet" "my_vpc_subnet" {
  name = "my-new-subnet"
  cidr = "10.0.0.0/16"
  location = "us-eaststaging1-a"
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

  // It is currently not possible for terraform to create subnets and firewall rules concurrently.
  // This directive should be specified when creating firewall rules and subnets to avoid failures.
  depends_on = [crusoe_vpc_subnet.my_vpc_subnet]
}

// Create a VM in the new subnet
resource "crusoe_compute_instance" "my_vm_test" {
  name = "my-new-vm-test"
  type = "c1a.2x"
  location = "us-eaststaging1-a"

  # specify the base image
  image = "ubuntu20.04:latest"

  ssh_key = local.my_ssh_key

  network_interfaces = [
      {
        subnet = crusoe_vpc_subnet.my_vpc_subnet.id
      }
    ]

}
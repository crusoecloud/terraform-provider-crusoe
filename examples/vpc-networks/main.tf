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

variable "name_prefix" {
  type    = string
  default = "tf-example-vpc-networks-"
}

variable "location" {
  type    = string
  default = "us-east1-a"
}

# Create a VPC network
resource "crusoe_vpc_network" "my_vpc_network" {
  name = "${var.name_prefix}network"
  cidr = "10.0.0.0/8"
}

# Create a VPC subnet
resource "crusoe_vpc_subnet" "my_vpc_subnet" {
  name     = "${var.name_prefix}subnet"
  cidr     = "10.0.0.0/16"
  location = var.location
  network  = crusoe_vpc_network.my_vpc_network.id
}

# Create a firewall rule
resource "crusoe_vpc_firewall_rule" "open_fw_rule" {
  network           = crusoe_vpc_network.my_vpc_network.id
  name              = "${var.name_prefix}firewall-rule"
  action            = "allow"
  direction         = "ingress"
  protocols         = "tcp"
  source            = "0.0.0.0/0"
  source_ports      = "1-65535"
  destination       = crusoe_vpc_network.my_vpc_network.cidr
  destination_ports = "1-65535"

  # It is currently not possible for terraform to create subnets and firewall rules concurrently.
  # This directive should be specified when creating firewall rules and subnets to avoid failures.
  depends_on = [crusoe_vpc_subnet.my_vpc_subnet]
}

# Create a VM in the new subnet
resource "crusoe_compute_instance" "my_vm" {
  name     = "${var.name_prefix}vm"
  type     = "c1a.2x"
  image    = "ubuntu22.04:latest"
  location = var.location
  network_interfaces = [{
    subnet = crusoe_vpc_subnet.my_vpc_subnet.id
  }]
  ssh_key = local.my_ssh_key
}

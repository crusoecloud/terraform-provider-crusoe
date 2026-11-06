terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

locals {
  my_ssh_key = file("~/.ssh/id_ed25519.pub")

  # An icmp rule has no ports.
  tcp_fw_rule_ports = var.tcp_fw_rule_protocols == "icmp" ? null : ["1-65535"]
}

variable "name_prefix" {
  type    = string
  default = "tf-example-vpc-networks-"
}

variable "location" {
  type    = string
  default = "us-east1-a"
}

variable "nat_gateway_enabled" {
  type    = bool
  default = false
}

variable "tcp_fw_rule_protocols" {
  type    = string
  default = "tcp"
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

  nat_gateway_enabled = var.nat_gateway_enabled
}

# Create a tcp firewall rule
resource "crusoe_vpc_firewall_rule" "my_tcp_fw_rule" {
  network                 = crusoe_vpc_network.my_vpc_network.id
  name                    = "${var.name_prefix}tcp-firewall-rule"
  action                  = "allow"
  direction               = "ingress"
  protocols               = var.tcp_fw_rule_protocols
  sources                 = [{ cidr = "0.0.0.0/0" }]
  source_port_ranges      = local.tcp_fw_rule_ports
  destinations            = [{ resource_id = crusoe_vpc_network.my_vpc_network.id }]
  destination_port_ranges = local.tcp_fw_rule_ports

  # It is currently not possible for terraform to create subnets and firewall rules concurrently.
  # This directive should be specified when creating firewall rules and subnets to avoid failures.
  depends_on = [crusoe_vpc_subnet.my_vpc_subnet]
}

output "tcp_fw_rule_id" {
  value = crusoe_vpc_firewall_rule.my_tcp_fw_rule.id
}

# Create an icmp firewall rule. An icmp rule has no ports, so the source and
# destination port fields are omitted.
resource "crusoe_vpc_firewall_rule" "my_icmp_fw_rule" {
  network      = crusoe_vpc_network.my_vpc_network.id
  name         = "${var.name_prefix}icmp-firewall-rule"
  action       = "allow"
  direction    = "ingress"
  protocols    = "icmp"
  sources      = [{ cidr = "0.0.0.0/0" }]
  destinations = [{ resource_id = crusoe_vpc_network.my_vpc_network.id }]

  depends_on = [crusoe_vpc_firewall_rule.my_tcp_fw_rule]
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

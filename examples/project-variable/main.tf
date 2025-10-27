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

# Project ID must be specified when creating resources. This can be specified in two ways:
#  - Using the `project_id` top-level attribute of the resource being created, like in this example.
#  - Using the default project specified in the `~/.crusoe/config`
variable "project_id" {
  type    = string
  default = "00000000-0000-0000-0000-000000000000" # Replace with your project ID
}

variable "name_prefix" {
  type    = string
  default = "tf-example-project-variable-"
}

variable "vm" {
  type = object({
    type     = string
    image    = string
    location = string
  })
  default = {
    type     = "a100-80gb.1x"
    image    = "ubuntu22.04:latest"
    location = "us-east1-a"
  }
}

# Create a VPC network
resource "crusoe_vpc_network" "my_vpc_network" {
  project_id = var.project_id
  name       = "${var.name_prefix}network"
  cidr       = "10.0.0.0/8"
}

# Create a VPC subnet
resource "crusoe_vpc_subnet" "my_vpc_subnet" {
  project_id = var.project_id
  name       = "${var.name_prefix}subnet"
  cidr       = "10.0.0.0/16"
  location   = var.vm.location
  network    = crusoe_vpc_network.my_vpc_network.id
}

# Create a VM
resource "crusoe_compute_instance" "my_vm" {
  project_id = var.project_id
  name       = "${var.name_prefix}vm"
  type       = var.vm.type
  image      = var.vm.image
  location   = var.vm.location
  disks = [
    # Attach a data disk
    {
      id              = crusoe_storage_disk.data_disk.id
      mode            = "read-only"
      attachment_type = "data"
    }
  ]
  network_interfaces = [{
    subnet = crusoe_vpc_subnet.my_vpc_subnet.id
  }]
  ssh_key = local.my_ssh_key
}

resource "crusoe_storage_disk" "data_disk" {
  project_id = var.project_id
  name       = "${var.name_prefix}data-disk"
  size       = "100GiB"
  location   = var.vm.location
}

# Create a firewall rule
# This example rule allows all ingress over TCP to port 3000 on our VM
resource "crusoe_vpc_firewall_rule" "open_fw_rule" {
  project_id        = var.project_id
  network           = crusoe_compute_instance.my_vm.network_interfaces[0].network
  name              = "${var.name_prefix}firewall-rule"
  action            = "allow"
  direction         = "ingress"
  protocols         = "tcp"
  source            = "0.0.0.0/0"
  source_ports      = "1-65535"
  destination       = crusoe_compute_instance.my_vm.network_interfaces[0].private_ipv4.address
  destination_ports = "3000"
}

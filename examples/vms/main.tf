terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

variable "name_prefix" {
  type    = string
  default = "tf-example-vms-"
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

data "crusoe_vpc_subnets" "my_vpc_subnets" {
}

locals {
  my_ssh_key = file("~/.ssh/id_ed25519.pub")
  # Find a VPC subnet in the same location
  matching_vpc_subnets = [
    for subnet in data.crusoe_vpc_subnets.my_vpc_subnets.vpc_subnets :
    subnet if subnet.location == var.vm.location
  ]
  vpc_subnet_id = length(local.matching_vpc_subnets) > 0 ? local.matching_vpc_subnets[0].id : null
}

# Create a VM
resource "crusoe_compute_instance" "my_vm" {
  name     = "${var.name_prefix}vm"
  type     = var.vm.type
  image    = var.vm.image
  location = var.vm.location
  # Specify a reservation id to use, if applicable
  # If left unset, the lowest cost reservation will be selected by default
  # to create the VM without a reservation, specify an empty string
  # reservation_id = "[insert-reservation-id]"
  disks = [
    # Attach a data disk
    {
      id              = crusoe_storage_disk.data_disk.id
      mode            = "read-only"
      attachment_type = "data"
    }
  ]
  network_interfaces = [{
    # Subnet IDs can be obtained via the `crusoe networking vpc-subnets list` CLI command
    subnet = local.vpc_subnet_id
  }]
  ssh_key = local.my_ssh_key
}

resource "crusoe_storage_disk" "data_disk" {
  name     = "${var.name_prefix}data-disk"
  size     = "100GiB"
  location = var.vm.location
}

# Create a firewall rule
# This example rule allows all ingress over TCP to port 3000 on our VM
resource "crusoe_vpc_firewall_rule" "open_fw_rule" {
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

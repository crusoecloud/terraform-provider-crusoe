terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

variable "name_prefix" {
  type    = string
  default = "tf-example-transport-networks-"
}

variable "ib_vm" {
  type = object({
    slices   = number
    type     = string
    image    = string
    location = string
  })
  default = {
    slices   = 8
    type     = "a100-80gb-sxm-ib.8x"
    image    = "ubuntu22.04-nvidia-sxm-docker:latest"
    location = "us-east1-a"
  }
}

variable "vm_count" {
  type    = number
  default = 2
}

# List available IB networks
data "crusoe_transport_networks" "my_transport_networks" {}
output "transport_networks" {
  value = data.crusoe_transport_networks.my_transport_networks.transport_networks
}

locals {
  my_ssh_key = file("~/.ssh/id_ed25519.pub")
  # Find a transport network in the same location and with available capacity
  available_transport_networks = [
    for network in data.crusoe_transport_networks.my_transport_networks.transport_networks :
    network if network.location == var.ib_vm.location && anytrue([
      for capacity in network.capacities :
      capacity.quantity >= var.ib_vm.slices * var.vm_count && capacity.slice_type == var.ib_vm.type
    ])
  ]
  available_transport_network_id = length(local.available_transport_networks) > 0 ? local.available_transport_networks[0].id : null
}

# Output available_transport_network_id for testing
output "selected_transport_network_id" {
  value = local.available_transport_network_id
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
  location = var.ib_vm.location
  network  = crusoe_vpc_network.my_vpc_network.id
}

# Create a transport partition to deploy VMs in
resource "crusoe_transport_partition" "my_partition" {
  name                 = "${var.name_prefix}partition"
  transport_network_id = local.available_transport_network_id
}

# Create multiple VMs, all in the same transport partition
resource "crusoe_compute_instance" "my_vms" {
  count    = var.vm_count
  name     = "${var.name_prefix}vm-${count.index}"
  type     = var.ib_vm.type
  image    = var.ib_vm.image
  location = var.ib_vm.location
  # Attach one disk per VM
  disks = [
    {
      id              = crusoe_storage_disk.my_data_disks[count.index].id
      attachment_type = "data"
      mode            = "read-only"
    }
  ]
  network_interfaces = [{
    # Subnet IDs can be obtained via the `crusoe networking vpc-subnets list` CLI command
    subnet = crusoe_vpc_subnet.my_vpc_subnet.id
  }]
  host_channel_adapters = [
    {
      transport_partition_id = crusoe_transport_partition.my_partition.id
    }
  ]
  ssh_key = local.my_ssh_key
}

# Create multiple storage disks
resource "crusoe_storage_disk" "my_data_disks" {
  count    = var.vm_count
  name     = "${var.name_prefix}data-disk-${count.index}"
  size     = "100GiB"
  location = var.ib_vm.location
}

# This feature is currently in development. Reach out to support@crusoecloud.com with any questions.

terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

variable "name_prefix" {
  type    = string
  default = "tf-example-load-balancers-"
}

variable "vm" {
  type = object({
    type     = string
    image    = string
    location = string
    count    = number
  })
  default = {
    type     = "c1a.2x"
    image    = "ubuntu22.04:latest"
    location = "us-east1-a"
    count    = 2
  }
}

locals {
  my_ssh_key = file("~/.ssh/id_ed25519.pub")
}

# List load balancers
data "crusoe_load_balancer" "my_load_balancers" {}
output "crusoe_lbs" {
  value = data.crusoe_load_balancer.my_load_balancers
}

# Create a VPC network for the VMs
resource "crusoe_vpc_network" "my_vpc_network" {
  name = "${var.name_prefix}network"
  cidr = "10.0.0.0/8"
}

# Create a subnet for the VMs
resource "crusoe_vpc_subnet" "my_vm_subnet" {
  name     = "${var.name_prefix}vm-subnet"
  cidr     = "10.0.0.0/16"
  location = var.vm.location
  network  = crusoe_vpc_network.my_vpc_network.id
}

# Create two VMs in the new subnet
resource "crusoe_compute_instance" "my_vms" {
  count    = var.vm.count
  name     = "${var.name_prefix}vm-${count.index}"
  type     = var.vm.type
  image    = var.vm.image
  location = var.vm.location
  network_interfaces = [
    {
      subnet = crusoe_vpc_subnet.my_vm_subnet.id
    }
  ]
  ssh_key = local.my_ssh_key
}

# Create a subnet for the load balancer
resource "crusoe_vpc_subnet" "my_lb_subnet" {
  name       = "${var.name_prefix}lb-subnet"
  cidr       = "10.1.0.0/18"
  location   = var.vm.location
  network    = crusoe_vpc_network.my_vpc_network.id
  depends_on = [crusoe_vpc_subnet.my_vm_subnet]
}

# Create a load balancer in the new subnet
resource "crusoe_load_balancer" "my_load_balancer" {
  name      = "${var.name_prefix}lb"
  algorithm = "random"
  destinations = [
    {
      resource_id = crusoe_compute_instance.my_vms[0].id
    },
    {
      resource_id = crusoe_compute_instance.my_vms[1].id
    }
  ]
  location  = var.vm.location
  protocols = ["tcp"]
  network_interfaces = [
    {
      subnet = crusoe_vpc_subnet.my_lb_subnet.id
    }
  ]
}

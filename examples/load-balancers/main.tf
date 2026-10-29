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

# Create a load balancer fronting the VMs. An external load balancer attaches to the
# VPC network, and its backends are the private IPs of existing VMs.
resource "crusoe_load_balancer" "my_load_balancer" {
  name     = "${var.name_prefix}lb"
  location = var.vm.location
  protocol = "tcp"
  vpc_id   = crusoe_vpc_network.my_vpc_network.id

  listen_ports_and_backends = [
    {
      listen_port = 80
      backends = [
        for vm in crusoe_compute_instance.my_vms : {
          ip   = vm.network_interfaces[0].private_ipv4.address
          port = 80
        }
      ]
    }
  ]
}

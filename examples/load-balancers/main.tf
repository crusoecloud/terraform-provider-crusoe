// This feature is currently in development. Reach out to support@crusoecloud.com with any questions.

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

// list load balancers
data "crusoe_load_balancer" "my_load_balancers" {}
output "crusoe_lbs" {
  value = data.crusoe_load_balancer.my_load_balancers
}

resource "crusoe_vpc_network" "my_vpc_network" {
  name = "my-new-network"
  cidr = "10.0.0.0/8"
}

// Create subnet for the VMs
resource "crusoe_vpc_subnet" "my_vm_subnet" {
  name = "my-vm-subnet"
  cidr = "10.0.0.0/16"
  location = "us-eastdevelopment1-a"
  network = crusoe_vpc_network.my_vpc_network.id
}

// Create two VMs in the new subnet
resource "crusoe_compute_instance" "my_vm" {
  count = 2
  name = "my-lb-vm-${count.index}"
  type = "c1a.2x"
  location = "us-eastdevelopment1-a"

  # specify the base image
  image = "ubuntu22.04:latest"

  ssh_key = local.my_ssh_key

  network_interfaces = [
      {
        subnet = crusoe_vpc_subnet.my_vm_subnet.id
      }
    ]

}

// Create subnet for the load balancer
resource "crusoe_vpc_subnet" "my_lb_subnet" {
  name = "my-lb-subnet"
  cidr = "10.1.0.0/18"
  location = "us-eastdevelopment1-a"
  network = crusoe_vpc_network.my_vpc_network.id
  depends_on = [crusoe_vpc_subnet.my_vm_subnet]
}

// Create load balancer in the new subnet
resource "crusoe_load_balancer" "my_load_balancer" {
  name         = "my-new-lb"
  algorithm    = "random"
  destinations = [
    {
        resource_id = crusoe_compute_instance.my_vm[0].id
    },
    {
        resource_id = crusoe_compute_instance.my_vm[1].id
    }
  ]
  location     = "us-eastdevelopment1-a"
  protocols    = ["tcp"]
  network_interfaces = [
    {
        subnet = crusoe_vpc_subnet.my_lb_subnet.id
    }
  ]
}
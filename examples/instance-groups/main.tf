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

variable "name_prefix" {
  type    = string
  default = "tf-example-instance-groups-"
}

variable "vm" {
  type = object({
    type     = string
    image    = string
    location = string
  })
  default = {
    type     = "c1a.2x"
    image    = "ubuntu22.04:latest"
    location = "us-east1-a"
  }
}

variable "vm_count" {
  type    = number
  default = 3
}

# List instance groups
data "crusoe_compute_instance_groups" "my_groups" {}
output "crusoe_groups" {
  value = data.crusoe_compute_instance_groups.my_groups
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
  location = var.vm.location
  network  = crusoe_vpc_network.my_vpc_network.id
}

# Create an instance template
resource "crusoe_instance_template" "my_template" {
  name     = "${var.name_prefix}template"
  type     = var.vm.type
  image    = var.vm.image
  location = var.vm.location
  # Attach one disk per VM
  disks = [
    {
      size = "10GiB"
      type = "persistent-ssd"
    }
  ]
  # This can be obtained via the `crusoe networking vpc-subnets list` CLI command
  subnet  = crusoe_vpc_subnet.my_vpc_subnet.id
  ssh_key = local.my_ssh_key
  # Optional parameter to configure placement policy, only "spread" is currently supported
  # Defaults to "unspecified" if not provided
  placement_policy = "spread"
}

# Create an instance group
resource "crusoe_compute_instance_group" "my_group" {
  name                   = "${var.name_prefix}group"
  instance_name_prefix   = "${var.name_prefix}vm"
  instance_template      = crusoe_instance_template.my_template.id
  running_instance_count = var.vm_count
}

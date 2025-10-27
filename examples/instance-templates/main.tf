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
  default = "tf-example-instance-templates-"
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

variable "vm_count" {
  type    = number
  default = 1
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

# List instance templates
data "crusoe_instance_templates" "my_templates" {}
output "crusoe_templates" {
  value = data.crusoe_instance_templates.my_templates.instance_templates
}

# Create an instance template
resource "crusoe_instance_template" "my_template" {
  name     = "${var.name_prefix}template"
  type     = var.vm.type
  image    = var.vm.image
  location = var.vm.location
  # Attach two data disks per VM
  disks = [
    {
      size = "10GiB"
      type = "persistent-ssd"
    },
    {
      size = "11GiB"
      type = "persistent-ssd"
    }
  ]
  # Subnet IDs can be obtained via the `crusoe networking vpc-subnets list` CLI command
  subnet  = crusoe_vpc_subnet.my_vpc_subnet.id
  ssh_key = local.my_ssh_key
  # Optional parameter to configure placement policy, only "spread" is currently supported
  # Defaults to "unspecified" if not provided
  placement_policy = "spread"
}

# Create VMs from template
resource "crusoe_compute_instance_by_template" "my_vms" {
  name_prefix       = "${var.name_prefix}vm"
  instance_template = crusoe_instance_template.my_template.id
  count             = var.vm_count
}

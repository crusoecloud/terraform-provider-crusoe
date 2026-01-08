// This example creates an instance group with a configurable number of VMs.
// Before running, update the subnet_id variable with your subnet ID.

terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

// SSH key for VM access - update the path if your key is in a different location
locals {
  my_ssh_key = file("~/.ssh/id_ed25519.pub")
}

// -----------------
// Input Variables
// -----------------

variable "name_prefix" {
  type        = string
  default     = "tf-example-instance-groups-"
  description = "Prefix for resource names"
}

variable "subnet_id" {
  type        = string
  description = "Subnet ID for the instance template (crusoe networking vpc-subnets list)"
  default     = "00000000-0000-0000-0000-000000000000"
}

variable "vm" {
  type = object({
    type     = string
    image    = string
    location = string
  })
  default = {
    type     = "c1a.2x"             # VM instance type
    image    = "ubuntu22.04:latest" # OS image
    location = "us-east1-a"         # Must match subnet location
  }
  description = "VM configuration for the instance template"
}

variable "vm_count" {
  type        = number
  default     = 3
  description = "Number of instances in the group"
}

// -----------------
// Resources
// -----------------

// Instance template defines the VM configuration (type, image, disks, networking)
// All VMs in the instance group will be created using this template
resource "crusoe_instance_template" "my_template" {
  name     = "${var.name_prefix}template"
  type     = var.vm.type
  image    = var.vm.image
  location = var.vm.location

  // Each VM gets one persistent SSD disk
  disks = [
    {
      size = "10GiB"
      type = "persistent-ssd"
    }
  ]

  subnet  = var.subnet_id
  ssh_key = local.my_ssh_key

  // Optional parameter to configure placement policy, only "spread" is currently supported
  // Defaults to "unspecified" if not provided
  placement_policy = "spread"
}

// Instance group manages a set of identical VMs based on the template
// Automatically maintains the desired count of running instances
resource "crusoe_compute_instance_group" "my_group" {
  name                 = "${var.name_prefix}group"
  instance_template_id = crusoe_instance_template.my_template.id
  desired_count        = var.vm_count
  depends_on           = [crusoe_instance_template.my_template]
}

// -----------------
// Data Sources
// -----------------

// Data source to list all instance groups in the project
data "crusoe_compute_instance_groups" "all" {
  depends_on = [crusoe_compute_instance_group.my_group]
}

// -----------------
// Outputs
// -----------------

output "instance_group" {
  value       = crusoe_compute_instance_group.my_group
  description = "The created instance group with all attributes"
}


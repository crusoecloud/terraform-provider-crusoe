// This feature is currently in development. Reach out to support@crusoecloud.com with any questions.

terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

locals {
  my_ssh_key = file("~/.ssh/id_rsa.pub")
}

# list instance groups
data "crusoe_compute_instance_groups" "my_groups" {}
output "crusoe_groups" {
  value = data.crusoe_compute_instance_groups.my_groups
}

// new template
resource "crusoe_instance_template" "my_template" {
  name = "my-new-template"
  type = "c1a.2x"
  location = "us-east1-a"
  // this can be obtained via the `crusoe networking vpc-subnets list` CLI command
  subnet = "subnet-id"

  # specify the base image
  image = "ubuntu22.04:latest"

  disks = [
      // disk to create for each VM
      {
        size = "10GiB"
        type = "persistent-ssd"
      }
    ]

  ssh_key = local.my_ssh_key

}

// create an instance group
resource "crusoe_compute_instance_group" "my_group" {
  name = "my-instance-group"
  instance_name_prefix = "my-new-vm"
  instance_template = crusoe_instance_template.my_template.id
  running_instance_count = 3
}
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

# list instance templates
data "crusoe_instance_templates" "my_templates" {}
output "crusoe_templates" {
  value = data.crusoe_instance_templates.my_templates
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
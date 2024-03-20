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

// new template
resource "crusoe_instance_template" "my_template" {
  name = "my-new-template"
  type = "a40.1x"
  location = "us-northcentral1-a"
  subnet = "bd247b17-fd13-44ba-8aa8-703852b6f326"

  # specify the base image
  image = "ubuntu20.04:latest"

  disks = [
      // disk to create for each VM
      {
        size = "10GiB"
        type = "persistent-ssd"
      }
    ]

  ssh_key = local.ssh_key

}

// create vm from template, with a name of my-new-vm-N
resource "crusoe_compute_instance_by_template" "my_vm" {
  name_prefix = "my-new-vm"
  instance_template = crusoe_instance_template.my_template.id
}
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

resource "crusoe_project" "my_project" {
  name = "my-new-project"
}

// new VM
resource "crusoe_compute_instance" "my_vm" {
  count = 2
  name = "my-new-vm-${count.index}"
  type = "a100-80gb.1x"
  location = "us-northcentraldevelopment1-a"

  # optionally specify a different base image
  #image = "nvidia-docker"

  ssh_key = local.my_ssh_key
  startup_script = file("startup.sh")
  project_id = crusoe_project.my_project.id

  disks = [
    // disk attached at startup
    {
      id = crusoe_storage_disk.data_disk.id
      attachment_type = "disk-readonly"
    }
  ]

}

resource "crusoe_storage_disk" "data_disk" {
  name = "data-disk"
  size = "200GiB"
  project_id = crusoe_project.my_project.id
  location = "us-northcentraldevelopment1-a"
}


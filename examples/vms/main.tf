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

variable "project_id" {
  type    = string
  default = "9da86c46-900f-49f3-b56b-67123c562c3c"
}

// new VM
resource "crusoe_compute_instance" "my_vm" {
  count = 2
  name = "my-new-vm-${count.index}"
  type = "a40.1x"
  location = "us-northcentralstaging1-a"

  # optionally specify a different base image
  #image = "nvidia-docker"

  disks = [
      // disk attached at startup
      {
        id = crusoe_storage_disk.data_disk.id
        mode = "read-only"
        attachment_type = "data"
      }
    ]

  ssh_key = local.my_ssh_key
  startup_script = file("startup.sh")
  project_id = var.project_id



}

resource "crusoe_storage_disk" "data_disk" {
  name = "data-disk"
  size = "200GiB"
  project_id = var.project_id
  location = "us-northcentralstaging1-a"
}


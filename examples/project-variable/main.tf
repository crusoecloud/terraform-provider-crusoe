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

variable "project_id" {
  type    = string
  default = "dc799c96-a9f8-4cd7-89e8-aca3a1b00f48"
}

// new VM
resource "crusoe_compute_instance" "my_vm" {
  name     = "my-new-vm"
  type     = "c1a.2x"
  location = "us-southcentraldevelopment1-a"

  disks = [
    // disk attached at startup
    {
      id              = crusoe_storage_disk.data_disk.id
      mode            = "read-write"
      attachment_type = "data"
    }
  ]

  ssh_key    = local.my_ssh_key
  project_id = var.project_id

}

resource "crusoe_storage_disk" "data_disk" {
  name       = "data-disk"
  size       = "2GiB"
  project_id = var.project_id
  location   = "us-southcentraldevelopment1-a"
}

// firewall rule
// note: this allows all ingress over TCP to our VM
resource "crusoe_vpc_firewall_rule" "open_fw_rule" {
  network           = crusoe_compute_instance.my_vm.network_interfaces[0].network
  name              = "example-terraform-rule"
  action            = "allow"
  direction         = "ingress"
  protocols         = "tcp"
  source            = "0.0.0.0/0"
  source_ports      = "1-65535"
  destination       = crusoe_compute_instance.my_vm.network_interfaces[0].public_ipv4.address
  destination_ports = "1-65535"
  project_id        = var.project_id
}

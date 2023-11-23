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

// new VM
resource "crusoe_compute_instance" "my_vm" {
  name = "my-new-vm4"
  type = "a100-80gb.1x"
  location = "us-northcentraldevelopment1-a"

  # optionally specify a different base image
  #image = "nvidia-docker"

  ssh_key = local.my_ssh_key

  disks = [
    // disk attached at startup
    {
      id = crusoe_storage_disk.data_disk.id
      attachment_type = "disk-readonly"
    }
  ]

}

resource "crusoe_storage_disk" "data_disk" {
  name = "data-disk5"
  size = "200GiB"
  location = "us-northcentraldevelopment1-a"
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
}

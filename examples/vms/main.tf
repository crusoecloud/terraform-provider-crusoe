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

// new VM
resource "crusoe_compute_instance" "my_vm" {
  name = "my-new-vm"
  type = "c1a.2x"
  project_id = "7228d902-5ec6-4a8e-9025-a0b82b994763"
  location = "us-eaststaging1-a"

  # specify the base image
  image = "ubuntu20.04:latest"

  # specify a reservation id to use, if applicable
  # if left unset, the lowest cost reservation will be selected by default
  # to create the VM without a reservation, specify an empty string
  # reservation_id = "[insert-reservation-id]"

  disks = [
      
      {
        id = crusoe_storage_disk.data_disk.id
        mode = "read-only"
        attachment_type = "data"
      },
      {
        id = crusoe_storage_disk.data_disk_2.id
        mode = "read-only"
        attachment_type = "data"
      },
      // disk attached at startup
      
      
    ]

  ssh_key = local.my_ssh_key

}

resource "crusoe_storage_disk" "data_disk" {
  name = "data-disk"
  project_id = "7228d902-5ec6-4a8e-9025-a0b82b994763"
  size = "210GiB"
  location = "us-eaststaging1-a"
}

resource "crusoe_storage_disk" "data_disk_2" {
  name = "data-disk-2"
  project_id = "7228d902-5ec6-4a8e-9025-a0b82b994763"
  size = "200GiB"
  location = "us-eaststaging1-a"
}

# // firewall rule
# // this example rule allows all ingress over TCP to port 3000 on our VM
# resource "crusoe_vpc_firewall_rule" "open_fw_rule" {
#   network           = crusoe_compute_instance.my_vm.network_interfaces[0].network
#   name              = "example-terraform-rule"
#   action            = "allow"
#   direction         = "ingress"
#   protocols         = "tcp"
#   source            = "0.0.0.0/0"
#   source_ports      = "1-65535"
#   destination       = crusoe_compute_instance.my_vm.network_interfaces[0].private_ipv4.address
#   destination_ports = "3000"
# }

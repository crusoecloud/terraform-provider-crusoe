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

# list IB networks
data "crusoe_ib_networks" "ib_networks" {}
output "crusoe_ib" {
  value = data.crusoe_ib_networks.ib_networks
}

# create an IB partition to deploy VMs in
resource "crusoe_ib_partition" "my_partition" {
  name = "my-ib-partition"

  # available IB network IDs can be listed by using the output
  # above. alternatively, they can be obtain with the CLI by
  #   crusoe networking ib-network list
  ib_network_id = "<ib_network_id>"
  project_id = crusoe_project.my_project.id
}

# create multiple VMs, all in the same Infiniband partition
resource "crusoe_compute_instance" "my_vm1" {
  count = 8

  name = "ib-vm-${count.index}"
  type = "a100-80gb-sxm-ib.8x" # IB enabled VM type
  location = "us-east1-a"
  image = "ubuntu22.04-nvidia-sxm-docker:latest" # IB image

  ssh_key = local.my_ssh_key

  host_channel_adapters = [
    {
      ib_partition_id = crusoe_ib_partition.my_partition.id
    }
  ]
  disks = [
    // at startup, attach one disk per VM
    {
      id = crusoe_storage_disk.data_disk[count.index].id
      attachment_type = "data"
      mode = "read-only"
    }
  ]
}

# create multiple storage disks
resource "crusoe_storage_disk" "data_disk" {
  count = 8
  name = "data-disk-${count.index}"
  size = "1TiB"
  location = "us-east1-a"
}

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
  name = "andres-tf-test"
  type = "a40.1x"
  location = "us-northcentral1-a"

  # optionally specify a different base image
  #image = "nvidia-docker"

  ssh_key        = local.my_ssh_key
//  startup_script = file("startup.sh")

//  disks = [
//    // attached at startup
//     crusoe_storage_disk.data_disk
//  ]

  network_interfaces = [{
    public_ipv4 = {
      type: "dynamic"
    }
  }]
}

// attached disk
//resource "crusoe_storage_disk" "data_disk" {
//  name = "data-disk"
//  size = "200GiB"
//  location = "us-northcentral1-a"
//}

// firewall rule
// note: this allows all ingress over TCP to our VM
//resource "crusoe_vpc_firewall_rule" "open_fw_rule" {
//  network           = crusoe_compute_instance.my_vm.network_interfaces[0].network
//  name              = "testrule-terra"
//  action            = "allow"
//  direction         = "ingress"
//  protocols         = "tcp"
//  source            = "0.0.0.0/0"
//  source_ports      = "1-65535"
//  destination       = crusoe_compute_instance.my_vm.network_interfaces[0].public_ipv4.address
//  destination_ports = "1-65535"
//}

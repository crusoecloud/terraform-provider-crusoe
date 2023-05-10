terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

provider "crusoe" {
  // create a new access keypair at https://console.crusoecloud.com/security/tokens
  host = "https://api.crusoecloud.site/v1alpha4"
  access_key = "C6isrrkKTPi_Ze1WapCI4g"
  secret_key = "834oEWIPROoFRwXNAkOFnA"
}

locals {
  my_ssh_key = file("~/.ssh/id_ed25519.pub")
}

// new VM
//resource "crusoe_compute_instance" "my_vm" {
//  name = "my-new-vm"
//  type = "a40.1x"
//
//  ssh_key        = local.my_ssh_key
//  startup_script = file("startup.sh")
//
//  disks = [
//    // uncomment to attach at startup
//    // crusoe_storage_disk.data_disk
//  ]
//}

// attached disk
resource "crusoe_storage_disk" "data_disk" {
  location = "ndcr-wwh-staging"
  name = "data-disk"
  size = "2GiB"
}

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

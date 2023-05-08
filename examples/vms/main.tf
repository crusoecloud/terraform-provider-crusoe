terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

provider "crusoe" {
  # staging env
  host = "https://api.crusoecloud.site/v1alpha4"
  access_key = "MY_KEY"
  secret_key = "MY_SECRET"
}

locals {
  my_ssh_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFxu4fSr7AILGLlkr5xyJ+x7G5an0mO4WDTuuL2MDXym agutierrez@crusoeenergy.com"
}

// new VM
resource "crusoe_compute_instance" "test_vm" {
  name = "my-cool-vm"
  type = "a40.1x"

  ssh_key = local.my_ssh_key
  startup_script = file("startup.sh")

  disks = [
    // uncomment to attach at startup
    // crusoe_storage_disk.data_disk
  ]
}

// attached disk
resource "crusoe_storage_disk" "data_disk" {
  name = "data-disk1"
  size = "20GiB"
}

// firewall rule
resource "crusoe_vpc_firewall_rule" "fw_rule2" {
  network = crusoe_compute_instance.test_vm.network_interfaces[0].network //"2132b387-1692-4c52-8a51-c380b6889b87"
  name = "testrule-terra"
  action = "allow"
  direction = "ingress"
  protocols = "tcp"
  source = "0.0.0.0/0"
  source_ports = "1-65535"
  destination = crusoe_compute_instance.test_vm.network_interfaces[0].public_ipv4.address
  destination_ports = "1-65535"
}

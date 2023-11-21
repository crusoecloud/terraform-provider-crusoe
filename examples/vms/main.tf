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

resource "crusoe_project" "my_project"{
  name = "my-new-cool-project"
}

// new VM
resource "crusoe_compute_instance" "my_vm" {
  name = "my-new-vm"
  type = "a40.1x"
  location = "us-northcentral1-a"

  # optionally specify a different base image
  #image = "nvidia-docker"

  ssh_key        = local.my_ssh_key
  startup_script = file("startup.sh")
  project_id = crusoe_project.my_project.id

  disks = [
    // attached at startup
     crusoe_storage_disk.data_disk
  ]
}

// attached disk
resource "crusoe_storage_disk" "data_disk" {
  name = "data-disk"
  size = "200GiB"
  location = "us-northcentral1-a"
  project_id = crusoe_project.my_project.id
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

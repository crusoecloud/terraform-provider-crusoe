# unit.tftest.hcl
variables {
  name_prefix = "tf-test-vms-"
  vm = {
    type     = "a100-80gb.1x"
    image    = "ubuntu22.04:latest"
    location = "us-east1-a"
  }
}

run "validate_vm" {
  command = plan

  assert {
    condition     = crusoe_compute_instance.my_vm.name == "${var.name_prefix}vm"
    error_message = "Expected VM name to be '${var.name_prefix}vm', but got '${crusoe_compute_instance.my_vm.name}'."
  }

  assert {
    condition     = crusoe_compute_instance.my_vm.type == var.vm.type
    error_message = "Expected VM type to be '${var.vm.type}', but got '${crusoe_compute_instance.my_vm.type}'."
  }

  assert {
    condition     = crusoe_compute_instance.my_vm.location == var.vm.location
    error_message = "Expected VM location to be '${var.vm.location}', but got '${crusoe_compute_instance.my_vm.location}'."
  }

  assert {
    condition     = crusoe_compute_instance.my_vm.image == var.vm.image
    error_message = "Expected VM image to be '${var.vm.image}', but got '${crusoe_compute_instance.my_vm.image}'."
  }
}


run "validate_disk" {
  command = plan

  assert {
    condition     = crusoe_storage_disk.data_disk.name == "${var.name_prefix}data-disk"
    error_message = "Expected disk name to be '${var.name_prefix}data-disk', but got '${crusoe_storage_disk.data_disk.name}'."
  }

  assert {
    condition     = crusoe_storage_disk.data_disk.size == "100GiB"
    error_message = "Expected disk size to be '100GiB', but got '${crusoe_storage_disk.data_disk.size}'."
  }

  assert {
        condition     = crusoe_storage_disk.data_disk.location == var.vm.location
    error_message = "Expected disk location to be '${var.vm.location}', but got '${crusoe_storage_disk.data_disk.location}'."
  }
}

run "validate_firewall_rule" {
  command = plan

  assert {
    condition     = crusoe_vpc_firewall_rule.open_fw_rule.name == "${var.name_prefix}firewall-rule"
    error_message = "Expected firewall rule name to be '${var.name_prefix}firewall-rule', but got '${crusoe_vpc_firewall_rule.open_fw_rule.name}'."
  }

  assert {
    condition     = crusoe_vpc_firewall_rule.open_fw_rule.action == "allow"
    error_message = "Expected firewall rule action to be 'allow', but got '${crusoe_vpc_firewall_rule.open_fw_rule.action}'."
  }

  assert {
    condition     = crusoe_vpc_firewall_rule.open_fw_rule.direction == "ingress"
    error_message = "Expected firewall rule direction to be 'ingress', but got '${crusoe_vpc_firewall_rule.open_fw_rule.direction}'."
  }

  assert {
    condition     = crusoe_vpc_firewall_rule.open_fw_rule.protocols == "tcp"
    error_message = "Expected firewall rule protocol to be 'tcp', but got '${crusoe_vpc_firewall_rule.open_fw_rule.protocols}'."
  }

  assert {
    condition     = crusoe_vpc_firewall_rule.open_fw_rule.source == "0.0.0.0/0"
    error_message = "Expected firewall source name to be '0.0.0.0/0', but got '${crusoe_vpc_firewall_rule.open_fw_rule.source}'."
  }

  assert {
    condition     = crusoe_vpc_firewall_rule.open_fw_rule.source_ports == "1-65535"
    error_message = "Expected firewall rule source ports to be '1-65535', but got '${crusoe_vpc_firewall_rule.open_fw_rule.source_ports}'."
  }

  assert {
    condition     = crusoe_vpc_firewall_rule.open_fw_rule.destination_ports == "3000"
    error_message = "Expected firewall rule destination ports to be '3000', but got '${crusoe_vpc_firewall_rule.open_fw_rule.destination_ports}'."
  }
}

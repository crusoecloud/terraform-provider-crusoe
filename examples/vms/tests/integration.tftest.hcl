# integration.tftest.hcl
variables {
  name_prefix = "tf-test-vms-"
  vm = {
    type     = "a100-80gb.1x"
    image    = "ubuntu22.04:latest"
    location = "us-east1-a"
  }
}

run "check_vm_and_disk_exists" {
  command = apply

  assert {
    condition     = crusoe_compute_instance.my_vm.id != null
    error_message = "The VM '${var.name_prefix}vm' was not created successfully, as its ID is null."
  }

  assert {
    condition     = crusoe_storage_disk.data_disk.id != null
    error_message = "The data disk '${var.name_prefix}data-disk' was not created successfully, as its ID is null."
  }

  assert {
    condition     = crusoe_vpc_firewall_rule.open_fw_rule.id != null
    error_message = "The firewall rule '${var.name_prefix}firewall-rule' was not created successfully, as its ID is null."
  }

  assert {
    condition     = crusoe_vpc_firewall_rule.open_fw_rule.destination == crusoe_compute_instance.my_vm.network_interfaces[0].private_ipv4.address
    error_message = "Expected firewall rule destination to match the VMs's destination IP address."
  }

  assert {
    condition     = crusoe_vpc_firewall_rule.open_fw_rule.network == crusoe_compute_instance.my_vm.network_interfaces[0].network
    error_message = "Expected firewall rule network to match the VM's network, but got '${crusoe_vpc_firewall_rule.open_fw_rule.network}'."
  }

  assert {
    condition     = tolist(crusoe_compute_instance.my_vm.disks)[0].id == crusoe_storage_disk.data_disk.id
    error_message = "The VM's data disk's ID does not match the data disk's ID."
  }
}

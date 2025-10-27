variables {
  name_prefix = "tf-test-ib-"
  ib_vm = {
    slices   = 8
    type     = "a100-80gb-sxm-ib.8x"
    image    = "ubuntu22.04-nvidia-sxm-docker:latest"
    location = "us-east1-a"
  }
  vm_count = 2
}

run "validate_create_ib_vm" {
  command = apply

  assert {
    condition     = crusoe_vpc_network.my_vpc_network.id != null
    error_message = "The network was not created successfully, as its ID is null."
  }

  assert {
    condition     = crusoe_vpc_subnet.my_vpc_subnet.id != null
    error_message = "The subnet was not created successfully, as its ID is null."
  }

  assert {
    condition     = crusoe_ib_partition.my_partition.id != null
    error_message = "The IB partition was not created successfully, as its ID is null."
  }

  assert {
    condition     = length(crusoe_storage_disk.my_data_disks) == var.vm_count
    error_message = <<-EOT
    Expected ${var.vm_count} storage disks, but ${length(crusoe_storage_disk.my_data_disks)} were created.
    Successfully created: ${join(", ", [for disk in crusoe_storage_disk.my_data_disks : disk.name])}
    EOT
  }

  assert {
    condition     = length(crusoe_compute_instance.my_vms) == var.vm_count
    error_message = <<-EOT
    Expected ${var.vm_count} VMs, but ${length(crusoe_compute_instance.my_vms)} were created.
    Successfully created: ${join(", ", [for vm in crusoe_compute_instance.my_vms : vm.name])}
    EOT
  }

  assert {
    condition = alltrue([
      for vm in crusoe_compute_instance.my_vms :
      vm.host_channel_adapters[0].ib_partition_id == crusoe_ib_partition.my_partition.id
    ])
    error_message = "Failed VM partition verification - expected: ${crusoe_ib_partition.my_partition.id}, actual: ${join("; ", [
      for vm in crusoe_compute_instance.my_vms :
      "${vm.name}: ${vm.host_channel_adapters[0].ib_partition_id}"
      if vm.host_channel_adapters[0].ib_partition_id != crusoe_ib_partition.my_partition.id
    ])}"
  }

  assert {
    condition = alltrue([
      for i in range(var.vm_count) :
      tolist(crusoe_compute_instance.my_vms[i].disks)[0].id == crusoe_storage_disk.my_data_disks[i].id
    ])
    error_message = "Failed disk verification - ${join("; ", [
      for i in range(var.vm_count) :
      "${crusoe_compute_instance.my_vms[i].name}: expected: ${crusoe_storage_disk.my_data_disks[i].id}, actual: ${tolist(crusoe_compute_instance.my_vms[i].disks)[0].id}"
      if tolist(crusoe_compute_instance.my_vms[i].disks)[0].id != crusoe_storage_disk.my_data_disks[i].id
    ])}"
  }
}

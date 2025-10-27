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

run "validate_network" {
  command = plan

  assert {
    condition     = crusoe_vpc_network.my_vpc_network.name == "${var.name_prefix}network"
    error_message = "Expected network name to be '${var.name_prefix}network', but got '${crusoe_vpc_network.my_vpc_network.name}'."
  }

  assert {
    condition     = crusoe_vpc_network.my_vpc_network.cidr == "10.0.0.0/8"
    error_message = "Expected network CIDR to be '10.0.0.0/8', but got '${crusoe_vpc_network.my_vpc_network.cidr}'."
  }
}

run "validate_subnet" {
  command = plan

  assert {
    condition     = crusoe_vpc_subnet.my_vpc_subnet.name == "${var.name_prefix}subnet"
    error_message = "Expected subnet name to be '${var.name_prefix}subnet', but got '${crusoe_vpc_subnet.my_vpc_subnet.name}'."
  }

  assert {
    condition     = crusoe_vpc_subnet.my_vpc_subnet.cidr == "10.0.0.0/16"
    error_message = "Expected subnet CIDR to be '10.0.0.0/16', but got '${crusoe_vpc_subnet.my_vpc_subnet.cidr}'."
  }

  assert {
    condition     = crusoe_vpc_subnet.my_vpc_subnet.location == var.ib_vm.location
    error_message = "Expected subnet location to be '${var.ib_vm.location}', but got '${crusoe_vpc_subnet.my_vpc_subnet.location}'."
  }
}

run "validate_partition" {
  command = plan

  assert {
    condition     = crusoe_ib_partition.my_partition.name == "${var.name_prefix}partition"
    error_message = "Expected IB partition name to be '${var.name_prefix}partition', but got '${crusoe_ib_partition.my_partition.name}'."
  }

  assert {
    condition     = crusoe_ib_partition.my_partition.ib_network_id == output.selected_ib_network_id
    error_message = "Expected IB network ID to be '${output.selected_ib_network_id}', but got '${crusoe_ib_partition.my_partition.ib_network_id}'."
  }
}

run "validate_disks" {
  command = plan

  assert {
    condition = alltrue([
      for i in range(var.vm_count) :
      crusoe_storage_disk.my_data_disks[i].name == "${var.name_prefix}data-disk-${i}"
    ])
    error_message = "Failed name verification - ${join("; ", [
      for i in range(var.vm_count) :
      "expected: ${var.name_prefix}data-disk-${i}, actual: ${crusoe_storage_disk.my_data_disks[i].name}"
      if crusoe_storage_disk.my_data_disks[i].name != "${var.name_prefix}data-disk-${i}"
    ])}"
  }

  assert {
    condition = alltrue([
      for i in range(var.vm_count) :
      crusoe_storage_disk.my_data_disks[i].size == "100GiB"
    ])
    error_message = "Failed size verification - expected: 100GiB, actual: ${join("; ", [
      for i in range(var.vm_count) :
      "${var.name_prefix}data-disk-${i}: ${crusoe_storage_disk.my_data_disks[i].size}"
      if crusoe_storage_disk.my_data_disks[i].size != "100GiB"
    ])}"
  }

  assert {
    condition = alltrue([
      for i in range(var.vm_count) :
      crusoe_storage_disk.my_data_disks[i].location == var.ib_vm.location
    ])
    error_message = "Failed location verification - expected: ${var.ib_vm.location}, actual: ${join("; ", [
      for i in range(var.vm_count) :
      "${var.name_prefix}data-disk-${i}: ${crusoe_storage_disk.my_data_disks[i].location}"
      if crusoe_storage_disk.my_data_disks[i].location != var.ib_vm.location
    ])}"
  }
}

run "validate_instances" {
  command = plan

  assert {
    condition = alltrue([
      for i in range(var.vm_count) :
      crusoe_compute_instance.my_vms[i].name == "${var.name_prefix}vm-${i}"
    ])
    error_message = "Failed name verification - ${join("; ", [
      for i in range(var.vm_count) :
      "expected: ${var.name_prefix}vm-${i}, actual: ${crusoe_compute_instance.my_vms[i].name}"
      if crusoe_compute_instance.my_vms[i].name != "${var.name_prefix}vm-${i}"
    ])}"
  }

  assert {
    condition = alltrue([
      for i in range(var.vm_count) :
      crusoe_compute_instance.my_vms[i].type == var.ib_vm.type
    ])
    error_message = "Failed type verification - expected: ${var.ib_vm.type}, actual: ${join("; ", [
      for i in range(var.vm_count) :
      "${var.name_prefix}vm-${i}: ${crusoe_compute_instance.my_vms[i].type}"
      if crusoe_compute_instance.my_vms[i].type != var.ib_vm.type
    ])}"
  }

  assert {
    condition = alltrue([
      for i in range(var.vm_count) :
      crusoe_compute_instance.my_vms[i].image == var.ib_vm.image
    ])
    error_message = "Failed image verification - expected: ${var.ib_vm.image}, actual: ${join("; ", [
      for i in range(var.vm_count) :
      "${var.name_prefix}vm-${i}: ${crusoe_compute_instance.my_vms[i].image}"
      if crusoe_compute_instance.my_vms[i].image != var.ib_vm.image
    ])}"
  }

  assert {
    condition = alltrue([
      for i in range(var.vm_count) :
      crusoe_compute_instance.my_vms[i].location == var.ib_vm.location
    ])
    error_message = "Failed location verification - expected: ${var.ib_vm.location}, actual: ${join("; ", [
      for i in range(var.vm_count) :
      "${var.name_prefix}vm-${i}: ${crusoe_compute_instance.my_vms[i].location}"
      if crusoe_compute_instance.my_vms[i].location != var.ib_vm.location
    ])}"
  }

  assert {
    condition = alltrue([
      for i in range(var.vm_count) :
      length(crusoe_compute_instance.my_vms[i].disks) == 1
    ])
    error_message = "Failed disks verification - expected: 1, actual: ${join("; ", [
      for i in range(var.vm_count) :
      "${var.name_prefix}vm-${i}: ${length(crusoe_compute_instance.my_vms[i].disks)}"
      if length(crusoe_compute_instance.my_vms[i].disks) != 1
    ])}"
  }
}

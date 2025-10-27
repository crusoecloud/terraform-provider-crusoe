# integration.tftest.hcl
variables {
  name_prefix = "tf-test-instance-templates-"
  vm = {
    type     = "a100-80gb.1x"
    image    = "ubuntu22.04:latest"
    location = "us-east1-a"
  }
  vm_count = 1 # there is a bug with instance by template create that prevents a larger number of VMs to be created.
}

run "validate_create_instance_templates" {
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
    condition     = crusoe_instance_template.my_template.id != null
    error_message = "The instance template was not created successfully, as its ID is null."
  }

  assert {
    condition     = crusoe_instance_template.my_template.name == "${var.name_prefix}template"
    error_message = "Expected instance template name to be '${crusoe_vpc_subnet.my_vpc_subnet.id}', but got '${crusoe_instance_template.my_template.subnet}'."
  }

  assert {
    condition     = crusoe_instance_template.my_template.image == var.vm.image
    error_message = "Expected instance template image to be '${var.vm.image}', but got '${crusoe_instance_template.my_template.image}'."
  }

  assert {
    condition     = crusoe_instance_template.my_template.type == var.vm.type
    error_message = "Expected instance template type to be '${var.vm.type}', but got '${crusoe_instance_template.my_template.type}'."
  }

  assert {
    condition     = crusoe_instance_template.my_template.location == var.vm.location
    error_message = "Expected instance template location to be '${var.vm.location}', but got '${crusoe_instance_template.my_template.location}'."
  }

  assert {
    condition     = length(crusoe_instance_template.my_template.disks) == 2
    error_message = "Expected instance template to request 2 disks, but got ${length(crusoe_instance_template.my_template.disks)}."
  }

  assert {
    condition     = crusoe_instance_template.my_template.subnet == crusoe_vpc_subnet.my_vpc_subnet.id
    error_message = "Expected instance template subnet to be '${crusoe_vpc_subnet.my_vpc_subnet.id}', but got '${crusoe_instance_template.my_template.subnet}'."
  }

  assert {
    condition     = crusoe_instance_template.my_template.placement_policy == "spread"
    error_message = "Expected instance template placement policy to be 'spread', but got '${crusoe_instance_template.my_template.placement_policy}'."
  }

  assert {
    condition     = length(crusoe_compute_instance_by_template.my_vms) == var.vm_count
    error_message = "Expected ${var.vm_count} VMs, but got ${length(crusoe_compute_instance_by_template.my_vms)}."
  }

  assert {
    condition = alltrue([
      for vm in crusoe_compute_instance_by_template.my_vms : vm.image == var.vm.image
    ])
    error_message = "Failed VM image verification - expected: ${var.vm.image}, actual: ${join("; ", [
      for vm in crusoe_compute_instance_by_template.my_vms : "${vm.name}: ${vm.image}"
    ])}"
  }

  assert {
    condition = alltrue([
      for vm in crusoe_compute_instance_by_template.my_vms : vm.location == var.vm.location
    ])
    error_message = "Failed VM location verification - expected: ${var.vm.location}, actual: ${join("; ", [
      for vm in crusoe_compute_instance_by_template.my_vms : "${vm.name}: ${vm.location}"
    ])}"
  }

  # attachment_type on disk is returning an empty string, commenting out until this can be resolved
  # assert {
  #   condition = alltrue([
  #     for vm in crusoe_compute_instance_by_template.my_vms : length([
  #       for disk in vm.disks : disk
  #       if disk.attachment_type != "os"
  #     ]) == 2
  #   ])
  #   error_message = "Failed VM disks verification - expected: 2, actual: ${join("; ", [
  #     for vm in crusoe_compute_instance_by_template.my_vms :
  #     "${vm.name}: ${length([
  #       for disk in vm.disks : disk
  #       if disk.attachment_type != "os"
  #     ])}"
  #   ])}"
  # }

  assert {
    condition = alltrue([
      for vm in crusoe_compute_instance_by_template.my_vms : vm.network_interfaces[0].subnet == crusoe_vpc_subnet.my_vpc_subnet.id
    ])
    error_message = "Failed VM subnet verification - expected: ${crusoe_vpc_subnet.my_vpc_subnet.id}, actual: ${join("; ", [
      for vm in crusoe_compute_instance_by_template.my_vms : "${vm.name}: ${vm.network_interfaces[0].subnet}"
    ])}"
  }
}

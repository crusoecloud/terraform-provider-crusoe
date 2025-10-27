# unit.tftest.hcl
variables {
  name_prefix = "tf-test-instance-templates-"
  vm = {
    type     = "a100-80gb.1x"
    image    = "ubuntu22.04:latest"
    location = "us-east1-a"
  }
  vm_count = 1 # there is a bug with instance by template create that prevents a larger number of VMs to be created.
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
    condition     = crusoe_vpc_subnet.my_vpc_subnet.location == var.vm.location
    error_message = "Expected subnet location to be '${var.vm.location}', but got '${crusoe_vpc_subnet.my_vpc_subnet.location}'."
  }
}

run "validate_instance_template" {
  command = plan

  assert {
    condition     = crusoe_instance_template.my_template.name == "${var.name_prefix}template"
    error_message = "Expected instance template name to be '${var.name_prefix}template', but got '${crusoe_instance_template.my_template.name}'."
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
    condition     = crusoe_instance_template.my_template.placement_policy == "spread"
    error_message = "Expected instance template placement policy to be 'spread', but got '${crusoe_instance_template.my_template.placement_policy}'."
  }

  assert {
    condition     = crusoe_instance_template.my_template.image == var.vm.image
    error_message = "Expected instance template image to be '${var.vm.image}', but got '${crusoe_instance_template.my_template.image}'."
  }
}

run "validate_instance_by_template" {
  command = plan

  assert {
    condition     = length(crusoe_compute_instance_by_template.my_vms) == var.vm_count
    error_message = "Expected VM count to be ${var.vm_count}, but got '${length(crusoe_compute_instance_by_template.my_vms)}'."
  }

  assert {
    condition = alltrue([
      for vm in crusoe_compute_instance_by_template.my_vms : vm.name_prefix == "${var.name_prefix}vm"
    ])
    error_message = "Failed name prefix verification - expected: ${var.name_prefix}vm, actual: ${join("; ", [
      for vm in crusoe_compute_instance_by_template.my_vms : vm.name_prefix
    ])}"
  }
}

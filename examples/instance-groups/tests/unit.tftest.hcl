# unit.tftest.hcl
variables {
  name_prefix = "tf-test-instance-groups-"
  vm = {
    type     = "c1a.2x"
    image    = "ubuntu22.04:latest"
    location = "us-east1-a"
  }
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
    condition     = crusoe_instance_template.my_template.image == var.vm.image
    error_message = "Expected instance template image to be '${var.vm.image}', but got '${crusoe_instance_template.my_template.image}'."
  }

  assert {
    condition     = length(crusoe_instance_template.my_template.disks) == 1
    error_message = "Expected VM to have exactly one disk, but got '${length(crusoe_instance_template.my_template.disks)}'."
  }
}

run "validate_instance_group" {
  command = plan

  assert {
    condition     = crusoe_compute_instance_group.my_group.name == "${var.name_prefix}group"
    error_message = "Expected instance group name to be '${var.name_prefix}group', but got '${crusoe_compute_instance_group.my_group.name}'."
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.instance_name_prefix == "${var.name_prefix}vm"
    error_message = "Expected instance template name prefix to be '${var.name_prefix}vm', but got '${crusoe_compute_instance_group.my_group.instance_name_prefix}'."
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.running_instance_count == 3
    error_message = "Expected running instance count to be 3, but got '${crusoe_compute_instance_group.my_group.running_instance_count}'."
  }
}

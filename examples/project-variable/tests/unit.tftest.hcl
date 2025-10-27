# unit.tftest.hcl

# NOTE: Create a terraform.tfvars file adjacent to this file with project_id defined using 
#   a project that differs from the default project specified in the `~/.crusoe/config`
variables {
  name_prefix = "tf-test-project-variable-"
  vm = {
    type     = "a100-80gb.1x"
    image    = "ubuntu22.04:latest"
    location = "us-east1-a"
  }
}

run "validate_vpc_network_project_id" {
  command = plan
  assert {
    condition     = crusoe_vpc_network.my_vpc_network.project_id == var.project_id
    error_message = "Expected project id to be '${var.project_id}', but got '${crusoe_vpc_network.my_vpc_network.project_id}'."
  }
}

run "validate_vpc_subnet_project_id" {
  command = plan
  assert {
    condition     = crusoe_vpc_subnet.my_vpc_subnet.project_id == var.project_id
    error_message = "Expected project id to be '${var.project_id}', but got '${crusoe_vpc_subnet.my_vpc_subnet.project_id}'."
  }
}

run "validate_vm_project_id" {
  command = plan
  assert {
    condition     = crusoe_compute_instance.my_vm.project_id == var.project_id
    error_message = "Expected project id to be '${var.project_id}', but got '${crusoe_compute_instance.my_vm.project_id}'."
  }
}


run "validate_disk_project_id" {
  command = plan
  assert {
    condition     = crusoe_storage_disk.data_disk.project_id == var.project_id
    error_message = "Expected project id to be '${var.project_id}', but got '${crusoe_storage_disk.data_disk.project_id}'."
  }
}

run "validate_firewall_rule_project_id" {
  command = plan
  assert {
    condition     = crusoe_vpc_firewall_rule.open_fw_rule.project_id == var.project_id
    error_message = "Expected project id to be '${var.project_id}', but got '${crusoe_vpc_firewall_rule.open_fw_rule.project_id}'."
  }
}


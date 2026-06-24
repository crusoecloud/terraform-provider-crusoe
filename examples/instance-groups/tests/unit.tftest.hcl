variables {
  name_prefix = "tf-test-instance-groups-"
  subnet_id   = "00000000-0000-0000-0000-000000000000"
  vm = {
    type     = "c1a.2x"
    image    = "ubuntu22.04:latest"
    location = "us-east1-a"
  }
  vm_count = 3
}

run "validate_large_instance_count" {
  command = plan

  variables {
    vm_count = 100
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.desired_count == 100
    error_message = "Expected desired_count to be 100, but got '${crusoe_compute_instance_group.my_group.desired_count}'."
  }
}

run "validate_custom_name_prefix" {
  command = plan

  variables {
    name_prefix = "my-custom-app-"
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.name == "my-custom-app-group"
    error_message = "Expected instance group name to be 'my-custom-app-group', but got '${crusoe_compute_instance_group.my_group.name}'."
  }

  assert {
    condition     = crusoe_instance_template.my_template.name == "my-custom-app-template"
    error_message = "Expected template name to be 'my-custom-app-template', but got '${crusoe_instance_template.my_template.name}'."
  }
}

run "validate_empty_name_prefix" {
  command = plan

  variables {
    name_prefix = ""
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.name == "group"
    error_message = "Expected instance group name to be 'group', but got '${crusoe_compute_instance_group.my_group.name}'."
  }

  assert {
    condition     = crusoe_instance_template.my_template.name == "template"
    error_message = "Expected template name to be 'template', but got '${crusoe_instance_template.my_template.name}'."
  }
}


run "validate_template_reference" {
  command = plan

  assert {
    condition     = crusoe_compute_instance_group.my_group.instance_template_id == crusoe_instance_template.my_template.id
    error_message = "Instance group should reference the template ID."
  }
}


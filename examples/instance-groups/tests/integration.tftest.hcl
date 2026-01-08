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

run "create_instance_group" {
  command = apply

  assert {
    condition     = crusoe_instance_template.my_template.id != null
    error_message = "Instance template was not created successfully, ID is null."
  }

  assert {
    condition     = crusoe_instance_template.my_template.name == "${var.name_prefix}template"
    error_message = "Expected template name '${var.name_prefix}template', got '${crusoe_instance_template.my_template.name}'."
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.id != null
    error_message = "Instance group was not created successfully, ID is null."
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.name == "${var.name_prefix}group"
    error_message = "Expected instance group name '${var.name_prefix}group', got '${crusoe_compute_instance_group.my_group.name}'."
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.instance_template_id == crusoe_instance_template.my_template.id
    error_message = "Instance group template ID does not match. Expected '${crusoe_instance_template.my_template.id}', got '${crusoe_compute_instance_group.my_group.instance_template_id}'."
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.desired_count == var.vm_count
    error_message = "Expected desired_count ${var.vm_count}, got ${crusoe_compute_instance_group.my_group.desired_count}."
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.project_id != null && crusoe_compute_instance_group.my_group.project_id != ""
    error_message = "Instance group project_id should not be null or empty."
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.created_at != null && crusoe_compute_instance_group.my_group.created_at != ""
    error_message = "Instance group created_at timestamp should not be null or empty."
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.updated_at != null && crusoe_compute_instance_group.my_group.updated_at != ""
    error_message = "Instance group updated_at timestamp should not be null or empty."
  }

  assert {
    condition     = contains(["HEALTHY", "UPDATING"], crusoe_compute_instance_group.my_group.state)
    error_message = "Expected state HEALTHY or UPDATING after create, got '${crusoe_compute_instance_group.my_group.state}'."
  }

  assert {
    condition     = length(crusoe_compute_instance_group.my_group.active_instance_ids) == crusoe_compute_instance_group.my_group.running_instance_count
    error_message = "active_instance_ids length should match running_instance_count."
  }

  assert {
    condition = alltrue([
      for id in crusoe_compute_instance_group.my_group.active_instance_ids : length(id) > 0
    ])
    error_message = "All active instance IDs should be non-empty strings."
  }

  assert {
    condition     = length(crusoe_compute_instance_group.my_group.inactive_instance_ids) == 0
    error_message = "Expected 0 inactive instances, got ${length(crusoe_compute_instance_group.my_group.inactive_instance_ids)}."
  }

  assert {
    condition     = length(data.crusoe_compute_instance_groups.all.instance_groups) > 0
    error_message = "Data source should return at least one instance group."
  }

  assert {
    condition = contains(
      [for ig in data.crusoe_compute_instance_groups.all.instance_groups : ig.id],
      crusoe_compute_instance_group.my_group.id
    )
    error_message = "Data source should include the created instance group by ID."
  }

  assert {
    condition = anytrue([
      for ig in data.crusoe_compute_instance_groups.all.instance_groups :
      ig.id == crusoe_compute_instance_group.my_group.id && ig.name == crusoe_compute_instance_group.my_group.name
    ])
    error_message = "Data source 'name' should match resource. Expected '${crusoe_compute_instance_group.my_group.name}'."
  }

  assert {
    condition = anytrue([
      for ig in data.crusoe_compute_instance_groups.all.instance_groups :
      ig.id == crusoe_compute_instance_group.my_group.id && ig.instance_template_id == crusoe_compute_instance_group.my_group.instance_template_id
    ])
    error_message = "Data source 'instance_template_id' should match resource. Expected '${crusoe_compute_instance_group.my_group.instance_template_id}'."
  }

  assert {
    condition = anytrue([
      for ig in data.crusoe_compute_instance_groups.all.instance_groups :
      ig.id == crusoe_compute_instance_group.my_group.id && ig.desired_count == crusoe_compute_instance_group.my_group.desired_count
    ])
    error_message = "Data source 'desired_count' should match resource. Expected ${crusoe_compute_instance_group.my_group.desired_count}."
  }

  assert {
    condition = anytrue([
      for ig in data.crusoe_compute_instance_groups.all.instance_groups :
      ig.id == crusoe_compute_instance_group.my_group.id && ig.project_id == crusoe_compute_instance_group.my_group.project_id
    ])
    error_message = "Data source 'project_id' should match resource. Expected '${crusoe_compute_instance_group.my_group.project_id}'."
  }
}

run "scale_up_and_rename" {
  command = apply

  variables {
    vm_count    = 4
    name_prefix = "tf-test-ig-renamed-"
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.name == "tf-test-ig-renamed-group"
    error_message = "Expected instance group name 'tf-test-ig-renamed-group' after rename, got '${crusoe_compute_instance_group.my_group.name}'."
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.desired_count == var.vm_count
    error_message = "Expected desired_count ${var.vm_count} after scale up, got ${crusoe_compute_instance_group.my_group.desired_count}."
  }

  assert {
    condition     = contains(["HEALTHY", "UPDATING"], crusoe_compute_instance_group.my_group.state)
    error_message = "Expected state HEALTHY or UPDATING after scale up, got '${crusoe_compute_instance_group.my_group.state}'."
  }


  assert {
    condition     = crusoe_compute_instance_group.my_group.updated_at >= crusoe_compute_instance_group.my_group.created_at
    error_message = "updated_at should be >= created_at after scale operation."
  }
}

run "scale_down_instance_group" {
  command = apply

  variables {
    vm_count    = 1
    name_prefix = "tf-test-ig-renamed-"
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.desired_count == var.vm_count
    error_message = "Expected desired_count ${var.vm_count} after scale down, got ${crusoe_compute_instance_group.my_group.desired_count}."
  }

  assert {
    condition     = contains(["HEALTHY", "UPDATING"], crusoe_compute_instance_group.my_group.state)
    error_message = "Expected state HEALTHY or UPDATING after scale down, got '${crusoe_compute_instance_group.my_group.state}'."
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.updated_at >= crusoe_compute_instance_group.my_group.created_at
    error_message = "updated_at should be >= created_at after scale operation."
  }
}

run "scale_to_zero" {
  command = apply

  variables {
    vm_count    = 0
    name_prefix = "tf-test-ig-renamed-"
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.desired_count == 0
    error_message = "Expected desired_count 0, got ${crusoe_compute_instance_group.my_group.desired_count}."
  }

  assert {
    condition     = contains(["HEALTHY", "UPDATING"], crusoe_compute_instance_group.my_group.state)
    error_message = "Expected state HEALTHY or UPDATING after scale to zero, got '${crusoe_compute_instance_group.my_group.state}'."
  }

  assert {
    condition     = crusoe_compute_instance_group.my_group.updated_at >= crusoe_compute_instance_group.my_group.created_at
    error_message = "updated_at should be >= created_at after scale operation."
  }
}

# There is a bug with create instance group - they only added running_instance_count and not
#  desired_count, so the number goes to 0 since the instances haven't started. It throws a
#  state error. Commenting this out until that is resolved

# # integration.tftest.hcl
# variables {
#   name_prefix = "tf-test-instance-groups-"
#   vm = {
#     type     = "c1a.2x"
#     image    = "ubuntu22.04:latest"
#     location = "us-east1-a"
#   }
#   vm_count = 3
# }

# run "validate_create_instance_group" {
#   command = apply

#   assert {
#     condition     = crusoe_vpc_network.my_vpc_network.id != null
#     error_message = "The network was not created successfully, as its ID is null."
#   }

#   assert {
#     condition     = crusoe_vpc_subnet.my_vpc_subnet.id != null
#     error_message = "The subnet was not created successfully, as its ID is null."
#   }

#   assert {
#     condition     = crusoe_instance_template.my_template.id != null
#     error_message = "The instance template was not created successfully, as its ID is null."
#   }

#   assert {
#     condition     = crusoe_instance_template.my_template.subnet == crusoe_vpc_subnet.my_vpc_subnet.id
#     error_message = "Expected instance template subnet to be '${crusoe_vpc_subnet.my_vpc_subnet.id}', but got '${crusoe_instance_template.my_template.subnet}'."
#   }

#   assert {
#     condition     = crusoe_compute_instance_group.my_group.id != null
#     error_message = "The instance group was not created successfully, as its ID is null."
#   }

#   assert {
#     condition     = crusoe_compute_instance_group.my_group.name == "${var.name_prefix}group"
#     error_message = "Expected instance group name to be '${var.name_prefix}group', but got '${crusoe_compute_instance_group.my_group.name}'."
#   }

#   assert {
#     condition     = crusoe_compute_instance_group.my_group.instance_template == crusoe_instance_template.my_template.id
#     error_message = "Expected instance group template to be '${crusoe_instance_template.my_template.id}', but got '${crusoe_compute_instance_group.my_group.instance_template}'."
#   }

#   assert {
#     condition     = length(crusoe_compute_instance_group.my_group.instances) == var.vm_count
#     error_message = "Expected ${var.vm_count} running instances, but ${length(crusoe_compute_instance_group.my_group.instances)} were created."
#   }
# }

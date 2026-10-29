# unit.tftest.hcl
variables {
    name_prefix = "tf-test-load-balancers-"
}

run "validate_load_balancer" {
    command = plan

    assert {
        condition     = crusoe_load_balancer.my_load_balancer.name == "tf-test-load-balancers-lb"
        error_message = "Expected load balancer name to be 'tf-test-load-balancers-lb', but got '${crusoe_load_balancer.my_load_balancer.name}'."
    }

    assert {
        condition     = crusoe_load_balancer.my_load_balancer.location == "us-east1-a"
        error_message = "Expected load balancer location to be 'us-east1-a', but got '${crusoe_load_balancer.my_load_balancer.location}'."
    }

    assert {
        condition     = crusoe_load_balancer.my_load_balancer.protocol == "tcp"
        error_message = "Expected load balancer protocol to be 'tcp', but got '${crusoe_load_balancer.my_load_balancer.protocol}'."
    }
}

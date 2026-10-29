# integration.tftest.hcl
variables {
    name_prefix = "tf-test-load-balancers-"
}

run "validate_create_load_balancer" {
    command = apply

    assert {
        condition     = crusoe_load_balancer.my_load_balancer.id != null
        error_message = "The load balancer 'tf-test-load-balancers-lb' was not created successfully, as its ID is null."
    }

    assert {
        condition     = crusoe_load_balancer.my_load_balancer.vip != ""
        error_message = "Expected the load balancer to be assigned a VIP."
    }

    assert {
        condition     = crusoe_load_balancer.my_load_balancer.vpc_id == crusoe_vpc_network.my_vpc_network.id
        error_message = "Expected load balancer vpc_id to be '${crusoe_vpc_network.my_vpc_network.id}', but got '${crusoe_load_balancer.my_load_balancer.vpc_id}'."
    }

    assert {
        condition     = length(one(crusoe_load_balancer.my_load_balancer.listen_ports_and_backends).backends) == var.vm.count
        error_message = "Expected one backend per VM."
    }

    assert {
        condition     = crusoe_load_balancer.my_load_balancer.health_check_options != null
        error_message = "Expected the API to populate default health check options."
    }
}

# The data source is read before the load balancer exists on the first apply, so re-apply to see it.
run "validate_data_source" {
    command = apply

    assert {
        condition = anytrue([
            for lb in data.crusoe_load_balancer.my_load_balancers.load_balancers :
            lb.id == crusoe_load_balancer.my_load_balancer.id && lb.protocol == "tcp"
        ])
        error_message = "Expected the data source to return the load balancer with protocol 'tcp'."
    }
}

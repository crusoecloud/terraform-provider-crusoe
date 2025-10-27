# integration.tftest.hcl
variables {
    name_prefix = "tf-test-vpc-networks-"
}

run "validate_create_vpc_network" {
    command = apply

    assert {
        condition     = crusoe_compute_instance.my_vm.id != null
        error_message = "The VM 'tf-test-vpc-networks-vm' was not created successfully, as its ID is null."
    }

    assert {
        condition     = crusoe_vpc_firewall_rule.open_fw_rule.id != null
        error_message = "The firewall rule 'tf-test-vpc-networks-firewall-rule' was not created successfully, as its ID is null."
    }

    assert {
        condition     = crusoe_vpc_network.my_vpc_network.id != null
        error_message = "The vpc network 'tf-test-vpc-networks-network' was not created successfully, as its ID is null."
    }

    assert {
        condition     = crusoe_vpc_subnet.my_vpc_subnet.id != null
        error_message = "The vpc subnet 'tf-test-vpc-networks-subnet' was not created successfully, as its ID is null."
    }
    
    assert {
        condition     = crusoe_compute_instance.my_vm.network_interfaces[0].subnet == crusoe_vpc_subnet.my_vpc_subnet.id
        error_message = "Expected VM network interface subnet to be '${crusoe_vpc_subnet.my_vpc_subnet.id}', but got '${crusoe_compute_instance.my_vm.network_interfaces[0].subnet}'."
    }

    assert {
        condition     = crusoe_vpc_firewall_rule.open_fw_rule.network == crusoe_vpc_network.my_vpc_network.id
        error_message = "Firewall rule must be associated with the correct VPC network."
    }
}
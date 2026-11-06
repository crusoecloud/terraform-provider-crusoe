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
        condition     = crusoe_vpc_firewall_rule.my_tcp_fw_rule.id != null
        error_message = "The firewall rule 'tf-test-vpc-networks-tcp-firewall-rule' was not created successfully, as its ID is null."
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
        condition     = crusoe_vpc_firewall_rule.my_tcp_fw_rule.network == crusoe_vpc_network.my_vpc_network.id
        error_message = "Firewall rule must be associated with the correct VPC network."
    }

    assert {
        condition     = crusoe_vpc_firewall_rule.my_icmp_fw_rule.id != null
        error_message = "The icmp firewall rule 'tf-test-vpc-networks-icmp-firewall-rule' was not created successfully, as its ID is null."
    }

    assert {
        condition     = crusoe_vpc_firewall_rule.my_icmp_fw_rule.source_port_ranges == null && crusoe_vpc_firewall_rule.my_icmp_fw_rule.destination_port_ranges == null
        error_message = "Expected the icmp firewall rule to have no ports after apply."
    }
}

# Switch the tcp rule to icmp. The rule must update in place and its ports must be cleared.
run "switch_tcp_fw_rule_to_icmp" {
    command = apply

    variables {
        tcp_fw_rule_protocols = "icmp"
    }

    assert {
        condition     = crusoe_vpc_firewall_rule.my_tcp_fw_rule.id == run.validate_create_vpc_network.tcp_fw_rule_id
        error_message = "The firewall rule ID changed after the switch to icmp, indicating the rule was replaced instead of updated in place."
    }

    assert {
        condition     = crusoe_vpc_firewall_rule.my_tcp_fw_rule.protocols == "icmp"
        error_message = "Expected the firewall rule protocol to be 'icmp', but got '${crusoe_vpc_firewall_rule.my_tcp_fw_rule.protocols}'."
    }

    assert {
        condition     = crusoe_vpc_firewall_rule.my_tcp_fw_rule.source_port_ranges == null && crusoe_vpc_firewall_rule.my_tcp_fw_rule.destination_port_ranges == null
        error_message = "Expected the firewall rule to have no ports after the switch to icmp."
    }
}

# Switch the rule back to tcp. The rule must update in place and its ports must be restored.
run "switch_tcp_fw_rule_back_to_tcp" {
    command = apply

    assert {
        condition     = crusoe_vpc_firewall_rule.my_tcp_fw_rule.id == run.validate_create_vpc_network.tcp_fw_rule_id
        error_message = "The firewall rule ID changed after the switch back to tcp, indicating the rule was replaced instead of updated in place."
    }

    assert {
        condition     = crusoe_vpc_firewall_rule.my_tcp_fw_rule.source_port_ranges[0] == "1-65535" && crusoe_vpc_firewall_rule.my_tcp_fw_rule.destination_port_ranges[0] == "1-65535"
        error_message = "Expected the firewall rule ports to be restored to '1-65535' after the switch back to tcp."
    }
}

run "enable_nat_gateway" {
    command = apply

    variables {
        nat_gateway_enabled = true
    }

    assert {
        condition     = length(crusoe_vpc_subnet.my_vpc_subnet.nat_gateways) == 1
        error_message = "Expected one NAT gateway after enabling nat_gateway_enabled, got ${length(crusoe_vpc_subnet.my_vpc_subnet.nat_gateways)}."
    }

    assert {
        condition     = crusoe_vpc_subnet.my_vpc_subnet.nat_gateways[0].public_ipv4_address != null
        error_message = "Expected the NAT gateway to have a public IPv4 address."
    }
}

run "disable_nat_gateway" {
    command = apply

    variables {
        nat_gateway_enabled = false
    }

    assert {
        condition     = length(crusoe_vpc_subnet.my_vpc_subnet.nat_gateways) == 0
        error_message = "Expected no NAT gateways after disabling nat_gateway_enabled, got ${length(crusoe_vpc_subnet.my_vpc_subnet.nat_gateways)}."
    }
}

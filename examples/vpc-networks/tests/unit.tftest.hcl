# unit.tftest.hcl
variables {
    name_prefix = "tf-test-vpc-networks-"
}

run "validate_network" {
    command = plan

    assert {
        condition     = crusoe_vpc_network.my_vpc_network.name == "tf-test-vpc-networks-network"
        error_message = "Expected network name to be 'tf-test-vpc-networks-network', but got '${crusoe_vpc_network.my_vpc_network.name}'."
    }

    assert {
        condition     = crusoe_vpc_network.my_vpc_network.cidr == "10.0.0.0/8"
        error_message = "Expected network CIDR to be '10.0.0.0/8', but got '${crusoe_vpc_network.my_vpc_network.cidr}'."
    }
}

run "validate_subnet" {
    command = plan

    assert {
        condition     = crusoe_vpc_subnet.my_vpc_subnet.name == "tf-test-vpc-networks-subnet"
        error_message = "Expected subnet name to be 'tf-test-vpc-networks-subnet', but got '${crusoe_vpc_subnet.my_vpc_subnet.name}'."
    }

    assert {
        condition     = crusoe_vpc_subnet.my_vpc_subnet.cidr == "10.0.0.0/16"
        error_message = "Expected subnet CIDR to be '10.0.0.0/16', but got '${crusoe_vpc_subnet.my_vpc_subnet.cidr}'."
    }

    assert {
        condition     = crusoe_vpc_subnet.my_vpc_subnet.location == "us-east1-a"
        error_message = "Expected subnet location to be 'us-east1-a', but got '${crusoe_vpc_subnet.my_vpc_subnet.location}'."
    }
}

run "validate_firewall_rule" {
    command = plan

    assert {
        condition     = crusoe_vpc_firewall_rule.open_fw_rule.name == "tf-test-vpc-networks-firewall-rule"
        error_message = "Expected firewall rule name to be 'tf-test-vpc-networks-firewall-rule', but got '${crusoe_vpc_firewall_rule.open_fw_rule.name}'."
    }

    assert {
        condition     = crusoe_vpc_firewall_rule.open_fw_rule.action == "allow"
        error_message = "Expected firewall rule action to be 'allow', but got '${crusoe_vpc_firewall_rule.open_fw_rule.action}'."
    }

    assert {
        condition     = crusoe_vpc_firewall_rule.open_fw_rule.direction == "ingress"
        error_message = "Expected firewall rule direction to be 'ingress', but got '${crusoe_vpc_firewall_rule.open_fw_rule.direction}'."
    }

    assert {
        condition     = crusoe_vpc_firewall_rule.open_fw_rule.protocols == "tcp"
        error_message = "Expected firewall rule protocol to be 'tcp', but got '${crusoe_vpc_firewall_rule.open_fw_rule.protocols}'."
    }

    assert {
        condition     = crusoe_vpc_firewall_rule.open_fw_rule.source == "0.0.0.0/0"
        error_message = "Expected firewall source name to be '0.0.0.0/0', but got '${crusoe_vpc_firewall_rule.open_fw_rule.source}'."
    }

    assert {
        condition     = crusoe_vpc_firewall_rule.open_fw_rule.source_ports == "1-65535"
        error_message = "Expected firewall rule source ports to be '1-65535', but got '${crusoe_vpc_firewall_rule.open_fw_rule.source_ports}'."
    }

    assert {
        condition     = crusoe_vpc_firewall_rule.open_fw_rule.destination == crusoe_vpc_network.my_vpc_network.cidr
        error_message = "Expected firewall rule destination to match the VPC network's CIDR."
    }

    assert {
        condition     = crusoe_vpc_firewall_rule.open_fw_rule.destination_ports == "1-65535"
        error_message = "Expected firewall rule destination ports to be '1-65535', but got '${crusoe_vpc_firewall_rule.open_fw_rule.destination_ports}'."
    }
}

run "validate_vm" {
    command = plan

    assert {
        condition     = crusoe_compute_instance.my_vm.name == "tf-test-vpc-networks-vm"
        error_message = "Expected VM name to be 'tf-test-vpc-networks-vm', but got '${crusoe_compute_instance.my_vm.name}'."
    }

    assert {
        condition     = crusoe_compute_instance.my_vm.image == "ubuntu22.04:latest"
        error_message = "Expected VM image to be 'ubuntu22.04:latest', but got '${crusoe_compute_instance.my_vm.image}'."
    }

    assert {
        condition     = crusoe_compute_instance.my_vm.location == "us-east1-a"
        error_message = "Expected VM location to be 'us-east1-a', but got '${crusoe_compute_instance.my_vm.location}'."
    }

    assert {
        condition     = crusoe_compute_instance.my_vm.type == "c1a.2x"
        error_message = "Expected VM type to be 'c1a.2x', but got '${crusoe_compute_instance.my_vm.type}'."
    }
}


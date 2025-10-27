# unit.tftest.hcl

variables {
  name_prefix           = "tf-test-kubernetes-"
  control_plane_version = "1.32.7-cmk.3"
  location              = "us-east1-a"
  add_ons = [
    "cluster_autoscaler",
    "nvidia_gpu_operator",
    "nvidia_network_operator",
    "crusoe_csi",
  ]
  worker = {
    type    = "a100-80gb.1x"
    version = "1.32.7-cmk.3"
    count   = 2
  }
  kubeconfig_path = "./kubeconfig.yaml"
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
    condition     = crusoe_vpc_subnet.my_vpc_subnet.location == var.location
    error_message = "Expected subnet location to be '${var.location}', but got '${crusoe_vpc_subnet.my_vpc_subnet.location}'."
  }
}

run "validate_kubernetes_cluster" {
  command = plan

  assert {
    condition     = crusoe_kubernetes_cluster.my_cluster.name == "${var.name_prefix}cluster"
    error_message = "Expected kubernetes cluster name to be '${var.name_prefix}cluster', but got '${crusoe_kubernetes_cluster.my_cluster.name}'."
  }

  assert {
    condition     = crusoe_kubernetes_cluster.my_cluster.version == var.control_plane_version
    error_message = "Expected kubernetes cluster version to be '${var.control_plane_version}', but got '${crusoe_kubernetes_cluster.my_cluster.version}'."
  }

  assert {
    condition     = crusoe_kubernetes_cluster.my_cluster.location == var.location
    error_message = "Expected kubernetes cluster location to be '${var.location}', but got '${crusoe_kubernetes_cluster.my_cluster.location}'."
  }
}

run "validate_kubernetes_node_pool" {
  command = plan

  assert {
    condition     = crusoe_kubernetes_node_pool.c1a_node_pool.name == "${var.name_prefix}c1a-node-pool"
    error_message = "Expected kubernetes node pool name to be '${var.name_prefix}c1a-node-pool', but got '${crusoe_kubernetes_node_pool.c1a_node_pool.name}'."
  }

  assert {
    condition     = crusoe_kubernetes_node_pool.c1a_node_pool.instance_count == var.worker.count
    error_message = "Expected kubernetes node pool instance count to be to be '${var.worker.count}', but got '${crusoe_kubernetes_node_pool.c1a_node_pool.instance_count}'."
  }

  assert {
    condition     = crusoe_kubernetes_node_pool.c1a_node_pool.type == var.worker.type
    error_message = "Expected kubernetes node pool type to be to be '${var.worker.type}', but got '${crusoe_kubernetes_node_pool.c1a_node_pool.type}'."
  }


  assert {
    condition     = crusoe_vpc_subnet.my_vpc_subnet.location == var.location
    error_message = "Expected subnet location to be '${var.location}', but got '${crusoe_vpc_subnet.my_vpc_subnet.location}'."
  }
}

run "validate_kubeconfig_file" {
  command = plan

  assert {
    condition     = local_file.kubeconfig_file.filename == var.kubeconfig_path
    error_message = "Expected kubernetes node pool type to be to be '${var.kubeconfig_path}', but got '${crusoe_kubernetes_node_pool.c1a_node_pool.type}'."
  }
}

# integration.tftest.hcl
variables {
  name_prefix = "tf-test-kubernetes-"
}

run "validate_create_kubernetes_cluster" {
  command = apply

  assert {
    condition     = crusoe_vpc_network.my_vpc_network.id != null
    error_message = "The vpc network was not created successfully, as its ID is null."
  }

  assert {
    condition     = crusoe_vpc_subnet.my_vpc_subnet.id != null
    error_message = "The vpc subnet was not created successfully, as its ID is null."
  }

  assert {
    condition     = crusoe_kubernetes_cluster.my_cluster.id != null
    error_message = "The kubernetes cluster was not created successfully, as its ID is null."
  }

  assert {
    condition     = crusoe_kubernetes_node_pool.my_node_pool.id != null
    error_message = "The node pool was not created successfully, as its ID is null."
  }

  assert {
    condition     = crusoe_kubernetes_node_pool.my_node_pool.cluster_id == crusoe_kubernetes_cluster.my_cluster.id
    error_message = "The node pool cluster ID does not match the kubernetes cluster ID."
  }

  assert {
    condition     = crusoe_kubeconfig.my_cluster_kubeconfig.cluster_id == crusoe_kubernetes_cluster.my_cluster.id
    error_message = "The kubeconfig cluster ID does not match the kubernetes cluster ID."
  }

  assert {
    condition     = local_file.kubeconfig_file.id != null
    error_message = "The local file was not created successfully, as its ID is null."
  }

  assert {
    condition     = local_file.kubeconfig_file.content == crusoe_kubeconfig.my_cluster_kubeconfig.kubeconfig_yaml
    error_message = "The content of the kubeconfig YAML file does not match the content on the local file."
  }
}

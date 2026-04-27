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

  assert {
    condition     = crusoe_kubernetes_cluster.my_cluster.apiserver_extra_args == null
    error_message = "Expected apiserver_extra_args to be null when not set."
  }
}

# Set extra args on the existing cluster (in-place PATCH, no recreation).
run "set_extra_args" {
  command = apply

  variables {
    apiserver_extra_args = {
      "audit-log-maxage" = "30"
    }
    scheduler_extra_args = {
      "v" = "2"
    }
    controller_manager_extra_args = {
      "node-monitor-grace-period" = "20s"
    }
  }

  # Cluster must not have been recreated — same ID proves in-place update.
  assert {
    condition     = crusoe_kubernetes_cluster.my_cluster.id == run.validate_create_kubernetes_cluster.cluster.id
    error_message = "Cluster was recreated instead of updated in-place when setting extra args."
  }

  assert {
    condition     = crusoe_kubernetes_cluster.my_cluster.apiserver_extra_args["audit-log-maxage"] == "30"
    error_message = "Expected apiserver_extra_args[audit-log-maxage] to be '30'."
  }

  assert {
    condition     = crusoe_kubernetes_cluster.my_cluster.scheduler_extra_args["v"] == "2"
    error_message = "Expected scheduler_extra_args[v] to be '2'."
  }

  assert {
    condition     = crusoe_kubernetes_cluster.my_cluster.controller_manager_extra_args["node-monitor-grace-period"] == "20s"
    error_message = "Expected controller_manager_extra_args[node-monitor-grace-period] to be '20s'."
  }
}

# Update existing extra args (in-place PATCH, no recreation).
run "update_extra_args" {
  command = apply

  variables {
    apiserver_extra_args = {
      "audit-log-maxage"    = "60"
      "audit-log-maxbackup" = "5"
    }
    scheduler_extra_args = {
      "v" = "2"
    }
    controller_manager_extra_args = {
      "node-monitor-grace-period" = "20s"
    }
  }

  assert {
    condition     = crusoe_kubernetes_cluster.my_cluster.id == run.validate_create_kubernetes_cluster.cluster.id
    error_message = "Cluster was recreated instead of updated in-place when updating extra args."
  }

  assert {
    condition     = crusoe_kubernetes_cluster.my_cluster.apiserver_extra_args["audit-log-maxage"] == "60"
    error_message = "Expected apiserver_extra_args[audit-log-maxage] to be updated to '60'."
  }

  assert {
    condition     = crusoe_kubernetes_cluster.my_cluster.apiserver_extra_args["audit-log-maxbackup"] == "5"
    error_message = "Expected apiserver_extra_args[audit-log-maxbackup] to be '5'."
  }
}

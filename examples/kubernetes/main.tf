terraform {
  required_providers {
    crusoe = {
      source  = "crusoecloud/crusoe"
    }
  }
}

locals {
  my_ssh_key = file("~/.ssh/id_ed25519.pub")
  control_plane_version = "1.30.8-cmk.1"
  worker_version = "1.30.8-cmk.1"
  location = "us-east1-a"
  add_ons = [
    "nvidia_gpu_operator",
    "nvidia_network_operator",
    "crusoe_csi",
  ]
  worker_count = 2
  worker_type = "c1a.4x"
}

resource "crusoe_kubernetes_cluster" "my_cluster" {
  name = "my-tf-cluster"
  # Set the desired CMK control plane version
  # See `crusoe kubernetes versions list` for available versions
  version = local.control_plane_version
  location = local.location

  # Optional: Set cluster/service CIDRs and node CIDR mask size
  # cluster_cidr = "192.168.1.0/24"
  # node_cidr_mask_size = "27"
  # service_cluster_ip_range = "192.168.2.0/24"

  # Optional: Add additional add-ons
  # See `crusoe kubernetes clusters create --help` for available add-ons
  # add_ons = local.add_ons
}

resource "crusoe_kubernetes_node_pool" "c1a_nodepool" {
  name = "my-tf-c1a-nodepool"
  cluster_id = crusoe_kubernetes_cluster.my_cluster.id
  instance_count = local.worker_count
  # Optional: Set the desired CMK worker node version
  # If not specified, the default is the latest stable version compatible with the cluster
  # List available node pool versions with "crusoe kubernetes versions list"
  # version = local.worker_version
  type = local.worker_type
  # Optional: Add your SSH public key to the created nodes to allow SSH access
  ssh_key = local.my_ssh_key

  # Optional: Kubernetes Node objects will be labeled with the following key:value pairs
  # requested_node_labels = {
  #   "labelkey" = "labelvalue"
  # }
}

output "cluster" {
  value = crusoe_kubernetes_cluster.my_cluster
}

output "nodepool" {
  value = crusoe_kubernetes_node_pool.c1a_nodepool
}

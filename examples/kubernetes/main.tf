terraform {
  required_providers {
    crusoe = {
      source  = "crusoecloud/crusoe"
    }
  }
}

locals {
  # Optional: Add your SSH public key to the created nodes to allow SSH access
  my_ssh_key = file("~/.ssh/id_ed25519.pub")
}

resource "crusoe_kubernetes_cluster" "my_cluster" {
  name = "tf-cluster"
  version = "1.30"
  configuration = "ha"
  location = "us-east1-a"

  # Optional: Set cluster/service CIDRs and node CIDR mask size
  # cluster_cidr = "192.168.1.0/24"
  # node_cidr_mask_size = "27"
  # service_cluster_ip_range = "192.168.2.0/24"
}

resource "crusoe_kubernetes_node_pool" "c1a_nodepool" {
  name = "tf-c1a-nodepool"
  cluster_id = crusoe_kubernetes_cluster.my_cluster.id
  instance_count = "1"
  type = "c1a.2x"
  ssh_key = local.my_ssh_key
  requested_node_labels = {
    # Optional: Kubernetes Node objects will be labeled with the following key:value pairs
    # "labelkey" = "labelvalue"
  }
}

output "cluster" {
  value = crusoe_kubernetes_cluster.my_cluster
}

output "nodepool" {
  value = crusoe_kubernetes_node_pool.c1a_nodepool
}

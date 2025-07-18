terraform {
  required_providers {
    crusoe = {
      source  = "crusoecloud/crusoe"
    }
  }
}

locals {
  my_ssh_public_key = file("~/.ssh/id_ed25519.pub")
  control_plane_version = "1.30.8-cmk.28"
  worker_version = "1.30.8-cmk.7"
  location = "us-east1-a"
  add_ons = [
    "cluster_autoscaler",
    "nvidia_gpu_operator",
    "nvidia_network_operator",
    "crusoe_csi",
  ]
  # Changing the worker count will modify the nodepool in-place
  # Requesting more workers will scale the nodepool until the new desired count is reached
  # Note that requesting fewer workers will not delete existing VMs - they must be deleted manually
  worker_count = 1
  worker_type = "c1a.4x"
  kubeconfig_path = "./kubeconfig.yaml"
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

  # Optional: Configure OIDC authentication for Kubernetes
  # Replace with your own identity provider values
  # oidc_issuer_url      = "https://auth.example.com/oauth2/aussah0123456bd97"
  # oidc_client_id       = "0123456789abcdef"
  # oidc_username_claim  = "sub"      # typically "sub" or "email"
  # oidc_groups_claim    = "groups"   # claim used to identify user groups
  # oidc_username_prefix = ""         # prefix prepended to username claim
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
  ssh_key = local.my_ssh_public_key

  # Optional: Kubernetes Node objects will be labeled with the following key:value pairs
  # requested_node_labels = {
  #   "labelkey" = "labelvalue"
  # }

  lifecycle {
    ignore_changes = [
    ssh_key,
    ]
  }
}

resource "crusoe_kubeconfig" "my_cluster_kubeconfig" {
  cluster_id = crusoe_kubernetes_cluster.my_cluster.id
  # Optional: Specify authentication type for kubeconfig.
  # Supported values: "admin_cert" (default), "oidc"
  # auth_type  = "oidc"
}

# # Optional: Use the kubeconfig with the Kubernetes provider
# provider "kubernetes" {
#     host = crusoe_kubeconfig.my_cluster_kubeconfig.cluster_address
#     cluster_ca_certificate = crusoe_kubeconfig.my_cluster_kubeconfig.cluster_ca_certificate
#     client_certificate = crusoe_kubeconfig.my_cluster_kubeconfig.client_certificate
#     client_key = crusoe_kubeconfig.my_cluster_kubeconfig.client_key
#     username = crusoe_kubeconfig.my_cluster_kubeconfig.username
# }
#
# resource "time_sleep" "wait_5m" {
#   depends_on = [crusoe_kubernetes_cluster.my_cluster]
#
#   # Sleep for 5 minutes to allow the cluster to become ready
#   create_duration = "5m"
# }
#
# # Optional: Use the Kubernetes provider to create a ConfigMap
# resource "kubernetes_config_map" "my-configmap" {
#   metadata {
#     name = "my-configmap"
#     namespace = "default"
#   }
#
#   data = {
#     configkey = "configvalue"
#   }
#
#   depends_on = [time_sleep.wait_5m]
# }

output "cluster" {
  value = crusoe_kubernetes_cluster.my_cluster
}

output "nodepool" {
  value = crusoe_kubernetes_node_pool.c1a_nodepool
}

# Optional: Output the kubeconfig YAML to a local file on disk
resource "local_file" "kubeconfig_file" {
  content = crusoe_kubeconfig.my_cluster_kubeconfig.kubeconfig_yaml
  filename = local.kubeconfig_path
}

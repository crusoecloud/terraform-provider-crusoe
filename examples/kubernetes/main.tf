terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

locals {
  my_ssh_public_key = file("~/.ssh/id_ed25519.pub")
}

variable "name_prefix" {
  type    = string
  default = "tf-example-kubernetes-"
}

variable "control_plane_version" {
  type    = string
  default = "1.31.7-cmk.7"
}

variable "location" {
  type    = string
  default = "us-east1-a"
}

variable "add_ons" {
  type = list(string)
  default = [
    "cluster_autoscaler",
    "nvidia_gpu_operator",
    "nvidia_network_operator",
    "crusoe_csi",
  ]
}

# Changing the worker count will modify the node pool in-place
# Requesting more workers will scale the node pool until the new desired count is reached
# Note that requesting fewer workers will not delete existing VMs - they must be deleted manually
variable "worker" {
  type = object({
    type    = string
    version = string
    count   = number
  })
  default = {
    type    = "a100-80gb.1x"
    version = "1.32.7-cmk.3"
    count   = 2
  }
}

variable "kubeconfig_path" {
  type    = string
  default = "./kubeconfig.yaml"
}

resource "crusoe_vpc_network" "my_vpc_network" {
  name = "${var.name_prefix}network"
  cidr = "10.0.0.0/8"
}

resource "crusoe_vpc_subnet" "my_vpc_subnet" {
  name                = "${var.name_prefix}subnet"
  cidr                = "10.0.0.0/16"
  location            = var.location
  network             = crusoe_vpc_network.my_vpc_network.id
  nat_gateway_enabled = false
}

resource "crusoe_vpc_firewall_rule" "my_egress_rule" {
  name              = "${var.name_prefix}egress-rule"
  action            = "allow"
  direction         = "egress"
  protocols         = "tcp,udp"
  source            = crusoe_vpc_subnet.my_vpc_subnet.cidr
  source_ports      = "1-65535"
  destination       = "0.0.0.0/0"
  destination_ports = "1-65535"
  network           = crusoe_vpc_network.my_vpc_network.id
}

resource "crusoe_kubernetes_cluster" "my_cluster" {
  name = "${var.name_prefix}cluster"
  # Set the desired CMK control plane version
  # See `crusoe kubernetes versions list` for available versions
  version   = var.control_plane_version
  location  = var.location
  subnet_id = crusoe_vpc_subnet.my_vpc_subnet.id

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

  # Optional: Enable private cluster creation
  # private = true

  depends_on = [crusoe_vpc_firewall_rule.my_egress_rule]
}

resource "crusoe_kubernetes_node_pool" "my_node_pool" {
  name           = "${var.name_prefix}node-pool"
  cluster_id     = crusoe_kubernetes_cluster.my_cluster.id
  instance_count = var.worker.count
  # Optional: Set the desired CMK worker node version
  # If not specified, the default is the latest stable version compatible with the cluster
  # List available node pool versions with "crusoe kubernetes versions list"
  version = var.worker.version
  type    = var.worker.type
  # Optional: Add your SSH public key to the created nodes to allow SSH access
  ssh_key = local.my_ssh_public_key

  # Optional: Kubernetes Node objects will be labeled with the following key:value pairs
  # requested_node_labels = {
  #   "labelkey" = "labelvalue"
  # }

  # Optional: Use local ephemeral NVMe disks for containerd storage
  # ephemeral_storage_for_containerd = true

  # Optional: Control the number of nodes to delete and recreate in batches when updating the node pool.
  # If omitted, any existing nodes will not be updated, but future ones will use the new config.
  # batch_size       = 10  # The number of nodes to replace at a time
  # batch_percentage = 100 # The percentage of nodes to replace at a time

  # Optional: Select the public IP type for the node_pool
  # public_ip_type = "dynamic"

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
# resource "kubernetes_config_map" "my_configmap" {
#   metadata {
#     name = "${var.name_prefix}configmap"
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

output "node_pool" {
  value = crusoe_kubernetes_node_pool.my_node_pool
}

# Optional: Output the kubeconfig YAML to a local file on disk
resource "local_file" "kubeconfig_file" {
  content  = crusoe_kubeconfig.my_cluster_kubeconfig.kubeconfig_yaml
  filename = var.kubeconfig_path
}

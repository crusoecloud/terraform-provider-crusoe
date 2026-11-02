resource "crusoe_kubernetes_addon" "example" {
  cluster_id = "my-cluster-id"
  addon_type = "cert-manager"
}

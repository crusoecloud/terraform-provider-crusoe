data "crusoe_registry_manifests" "example" {
  repo_name  = "my-repo"
  image_name = "my-image"
  location   = "us-southcentral1-a"
}

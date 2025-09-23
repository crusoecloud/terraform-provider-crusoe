terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

variable "project_id" {
  type    = string
  default = "dc799c96-a9f8-4cd7-89e8-aca3a1b00f48"
}

resource "crusoe_registry_repository" "my_registry_repo" {
  location   = "us-southcentral1-a"
  name       = "my_registry_repo"
  mode       = "pull-through-cache"
  project_id = var.project_id

  upstream_registry = {
    provider = "docker-hub"
    url      = "https://hub.docker.com"
  }
}

# Example of deleting a specific image from the registry
resource "crusoe_registry_image" "delete_alpine" {
  project_id = var.project_id
  repo_name  = "aws-ecr-public"
  image_name = "docker/library/alpine"
  location   = "us-southcentral1-a"
}

# Example of deleting a specific manifest from the registry
resource "crusoe_registry_manifest" "delete_alpine_manifest" {
  project_id = "dc799c96-a9f8-4cd7-89e8-aca3a1b00f48"
  repo_name  = "aws-ecr-public"
  image_name = "docker/library/alpine"
  digest = "sha256:56fd63902607fa52ae1735379eb9fb43d037a50f99846717afb5e96c33a80081"
  location   = "us-southcentral1-a"
}

data "crusoe_registry_repositories" "all" {
  project_id = var.project_id
}

output "registry_repositories" {
  value = [for repo in data.crusoe_registry_repositories.all.repositories : repo]
}

data "crusoe_registry_images" "all" {
  project_id = var.project_id
  repo_name  = "aws-ecr-public"
  location   = "us-southcentral1-a"
}

output "registry_images" {
  value = [for image in data.crusoe_registry_images.all.images : image]
}


# Example of listing manifests for a specific image
data "crusoe_registry_manifests" "busybox_manifests" {
  project_id = var.project_id
  repo_name  = "standard-bug-bash"
  image_name = "busybox"
  location   = "us-southcentral1-a"
}

output "busybox_manifests" {
  value = [for manifest in data.crusoe_registry_manifests.busybox_manifests.manifests : manifest]
}

# Example of listing manifests with tag filtering
data "crusoe_registry_manifests" "latest_manifests" {
  project_id   = var.project_id
  repo_name    = "standard-bug-bash"
  image_name   = "busybox"
  location     = "us-southcentral1-a"
  tag_contains = "1.35"
}

output "latest_manifests" {
  value = [for manifest in data.crusoe_registry_manifests.latest_manifests.manifests : manifest]
}

# Example of creating a registry token
resource "crusoe_registry_token" "my_token" {
  alias      = "my-registry-token"
  expires_at = "2025-10-21T23:59:59Z"
}

output "registry_token_id" {
  value = crusoe_registry_token.my_token.id
}

# Note: The actual token value is sensitive and won't be displayed in outputs
# but can be referenced in other resources if needed

# Example of listing all registry tokens
data "crusoe_registry_tokens" "all_tokens" {
  project_id = var.project_id
}

output "registry_tokens" {
  value = [for token in data.crusoe_registry_tokens.all_tokens.tokens : token]
}
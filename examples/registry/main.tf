terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

locals {
  location   = "us-southcentral1-a"
  repo_name  = "standard-bug-bash"
  mode       = "pull-through-cache"
  image_name = "busybox"
  tag        = "1.34"
  provider   = "docker-hub"
  url        = "https://hub.docker.com"
  alias      = "my_token_alias"
}

resource "crusoe_registry_repository" "my_registry_repo" {
  location = local.location
  name     = local.repo_name
  mode     = local.mode

  upstream_registry = {
    provider = local.provider
    url      = local.url
  }
}

data "crusoe_registry_repositories" "all" {
}

output "registry_repositories" {
  value = [for repo in data.crusoe_registry_repositories.all.repositories : repo]
}

data "crusoe_registry_images" "all" {
  repo_name = local.repo_name
  location  = local.location
}

output "registry_images" {
  value = [for image in data.crusoe_registry_images.all.images : image]
}

# Example of listing manifests for a specific image
data "crusoe_registry_manifests" "busybox_manifests" {
  repo_name  = local.repo_name
  image_name = local.image_name
  location   = local.location
}

output "busybox_manifests" {
  value = [for manifest in data.crusoe_registry_manifests.busybox_manifests.manifests : manifest]
}


# Example of listing manifests with tag filtering
data "crusoe_registry_manifests" "latest_manifests" {
  repo_name    = local.repo_name
  image_name   = local.image_name
  location     = local.location
  tag_contains = local.tag
}

output "latest_manifests" {
  value = [for manifest in data.crusoe_registry_manifests.latest_manifests.manifests : manifest]
}

# Example of creating a registry token
resource "crusoe_registry_token" "my_token" {
  alias = local.alias
  # expires_at = "2025-10-21T23:59:59Z" // Set value in RFC 3339 format
}

output "registry_token_id" {
  value = crusoe_registry_token.my_token.id
}

# Note: The actual token value is sensitive and won't be displayed in outputs
# but can be referenced in other resources if needed

# Example of listing all registry tokens
data "crusoe_registry_tokens" "all_tokens" {
}

output "registry_tokens" {
  value = [for token in data.crusoe_registry_tokens.all_tokens.tokens : token]
}
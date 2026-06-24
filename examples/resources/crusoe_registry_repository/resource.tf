resource "crusoe_registry_repository" "example" {
  location = "us-southcentral1-a"
  name     = "my-registry-repo"
  mode     = "pull-through-cache"

  upstream_registry = {
    provider = "docker-hub"
    url      = "https://hub.docker.com"
  }
}

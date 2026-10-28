resource "crusoe_registry_repository" "example" {
  location = "us-southcentral1-a"
  name     = "my-registry-repo"
  mode     = "pull-through-cache"

  upstream_registry = {
    provider = "docker-hub"
    url      = "https://hub.docker.com"
  }
}

# A pull-through cache of a private AWS ECR registry. Credentials are required when the
# repository is created; a repository created without them must be recreated to add them.
variable "ecr_access_key_id" {
  type      = string
  ephemeral = true
}

variable "ecr_secret_access_key" {
  type      = string
  sensitive = true
  ephemeral = true
}

resource "crusoe_registry_repository" "private_ecr" {
  location = "us-southcentral1-a"
  name     = "my-private-ecr-cache"
  mode     = "pull-through-cache"

  upstream_registry = {
    provider = "aws-ecr"
    url      = "https://123456789012.dkr.ecr.us-east-1.amazonaws.com"

    # username and password are write-only: never stored in plan or state.
    upstream_registry_credentials = {
      username = var.ecr_access_key_id # IAM access key ID
      password = var.ecr_secret_access_key
      # Increment to send new credentials in place.
      credentials_version = 1
    }
  }
}

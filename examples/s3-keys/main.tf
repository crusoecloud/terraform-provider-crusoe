terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

# Create an S3 access key
resource "crusoe_storage_s3_key" "example" {
  alias = "my-service-key"
  # Optional: set expiration date
  # expire_at = "2025-12-31T23:59:59Z"
}

# Output the access key ID (safe to display)
output "access_key_id" {
  value       = crusoe_storage_s3_key.example.access_key_id
  description = "The S3 access key ID"
}

# Output the secret access key (sensitive - only shown once)
output "secret_access_key" {
  value       = crusoe_storage_s3_key.example.secret_access_key
  description = "The S3 secret access key (only available after initial creation)"
  sensitive   = true
}

# List all S3 keys for the organization
data "crusoe_storage_s3_keys" "all" {}

output "all_keys" {
  value = data.crusoe_storage_s3_keys.all.keys
}
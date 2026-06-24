resource "crusoe_storage_s3_bucket" "example" {
  name     = "my-s3-bucket"
  location = "us-east1-a"

  versioning_enabled = true

  tags = {
    environment = "production"
  }
}

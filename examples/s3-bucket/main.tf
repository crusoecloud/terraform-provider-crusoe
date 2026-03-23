terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

locals {
  location    = "us-east1-a"
  bucket_name = "my-terraform-bucket"
}

# Basic S3 bucket - updated with tags
resource "crusoe_storage_s3_bucket" "basic" {
  name     = "${local.bucket_name}-basic"
  location = local.location

  tags = {
    environment = "testing"
    updated     = "true"
  }
}

output "basic_bucket_name" {
  value = crusoe_storage_s3_bucket.basic.name
}

# S3 bucket with versioning enabled
resource "crusoe_storage_s3_bucket" "versioned" {
  name               = "${local.bucket_name}-versioned"
  location           = local.location
  versioning_enabled = true

  tags = {
    environment = "development"
    managed_by  = "terraform"
  }
}

output "versioned_bucket_name" {
  value = crusoe_storage_s3_bucket.versioned.name
}

# S3 bucket with object lock and retention
resource "crusoe_storage_s3_bucket" "locked" {
  name                  = "${local.bucket_name}-locked"
  location              = local.location
  versioning_enabled    = true
  object_lock_enabled   = true
  retention_period      = 7
  retention_period_unit = "days"

  tags = {
    environment = "production"
    compliance  = "required"
    managed_by  = "terraform"
  }
}

output "locked_bucket_name" {
  value = crusoe_storage_s3_bucket.locked.name
}

output "locked_bucket_object_lock" {
  value = crusoe_storage_s3_bucket.locked.object_lock_enabled
}

# Data source to list all buckets
data "crusoe_storage_s3_buckets" "all" {
}

output "all_buckets" {
  value = [for bucket in data.crusoe_storage_s3_buckets.all.buckets : bucket.name]
}

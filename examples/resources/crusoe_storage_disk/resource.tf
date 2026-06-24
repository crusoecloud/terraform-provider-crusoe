resource "crusoe_storage_disk" "example" {
  name     = "my-data-disk"
  size     = "100GiB"
  location = "us-east1-a"
}

terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

locals {
  my_ssh_key = file("~/.ssh/id_ed25519.pub")
}

# Get a specific custom image by name
data "crusoe_compute_custom_image" "my_custom_image" {
  name = "my-custom-image"
}

# Get the latest custom image with a specific prefix
data "crusoe_compute_custom_image" "my_custom_image" {
  name_prefix = "my-custom-image"
}

# VM using a specific custom image
resource "crusoe_compute_instance" "vm_with_custom_image" {
  name         = "vm-with-custom-image"
  type         = "a40.1x"
  location     = "us-northcentral1-a"
  custom_image = data.crusoe_compute_custom_image.my_custom_image.newest_image.id
  ssh_key      = local.my_ssh_key
}

# VM using the latest custom image with a prefix
resource "crusoe_compute_instance" "vm_with_latest_custom_image" {
  name         = "vm-with-latest-custom-image"
  type         = "a40.1x"
  location     = "us-northcentral1-a"
  custom_image = data.crusoe_compute_custom_image.my_custom_image.newest_image.id
  ssh_key      = local.my_ssh_key
}

# Output the custom image details
output "custom_image_details" {
  value = data.crusoe_compute_custom_image.my_custom_image.newest_image
}

output "latest_custom_image_details" {
  value = data.crusoe_compute_custom_image.my_custom_image.newest_image
} 
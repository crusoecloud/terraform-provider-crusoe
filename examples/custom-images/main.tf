terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

# Get all custom images with a specific name
data "crusoe_compute_custom_image" "custom_images_by_name" {
  name = "my-custom-image"
}
# Output all custom images (list of 1 or 0 images)
output "all_custom_images_with_name_match" {
  value = data.crusoe_compute_custom_image.custom_images_by_name.custom_images
}

# Get the latest custom image with a specific prefix
data "crusoe_compute_custom_image" "custom_images_by_name_prefix" {
  name_prefix = "ubuntu-"
}
# Output the newest custom image (single image or nil)
output "newest_custom_image_with_name_prefix_match" {
  value = data.crusoe_compute_custom_image.custom_images_by_name_prefix.newest_image
}
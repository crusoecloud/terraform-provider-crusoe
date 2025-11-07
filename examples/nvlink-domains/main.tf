terraform {
  required_providers {
    crusoe = {
      source = "crusoecloud/crusoe"
    }
  }
}

data "crusoe_nvlink_domains" "my_nvlink_domains" {
}

output "nvlink_domains" {
  value = data.crusoe_nvlink_domains.my_nvlink_domains.nvlink_domains
}

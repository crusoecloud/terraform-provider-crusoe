terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

locals {
  # replace with path to your SSH key if different
  ssh_key = file("~/.ssh/id_rsa.pub")
}


resource "crusoe_load_balancer" "my_load_balancer" {
  name         = "my-new-lb"
  algorithm    = "random"
  destinations = [
    {
        resource_id = "2aaeea55-d12e-4653-82ce-5c8cf86798e4"
    }
  ]
  location     = "us-eaststaging1-a"
  protocols    = ["tcp"]
  network_interfaces = [
    {
        subnet = "28819919-acb3-484e-ac1f-3262fc44b1d8"
    }
  ]
}

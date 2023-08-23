
# Terraform Provider Crusoe

Test

This repo defines the official Terraform Provider for use with [Crusoe Cloud](https://crusoecloud.com/), the world's first carbon-reducing, low-cost GPU cloud platform.

## Getting Started

To get started, first [install Terraform](https://developer.hashicorp.com/terraform/downloads). Then, get an access keypair from https://console.crusoecloud.ai/security/tokens and add the following to `~/.crusoe/config`:

```toml
[default]
access_key_id="MY_ACCESS_KEY"
secret_key="MY_SECRET_KEY"
```

Then, add the following to the start of your terraform file, for example `main.tf`:

```
terraform {
  required_providers {
    crusoe = {
      source = "registry.terraform.io/crusoecloud/crusoe"
    }
  }
}

locals {
  # replace with path to your SSH key if different
  ssh_key = file("~/.ssh/id_ed25519.pub")
}
```

You can then use Terraform to create instances, disks, and networking rules. To create 10 VMs with 8 80GB A100s, we would add the following block: 

```terraform
# Create 10, 8xA100-80GB VMs
resource "crusoe_compute_instance" "nodes" {
  count = 10
  name = "node-${count.index}"
  type = "a100-80gb.8x"
  ssh_key = local.ssh_key
}
```

For more usage examples, including storage disks, startup scripts, and firewall rules, see the [examples folder](./examples/).

## Development

To develop the Terraform provider, you'll need a recent version of [golang](https://go.dev/doc/install) installed.
At the moment, the project is pinned to go1.18, but newer versions will likely work for development.  

Add the following to your `~/.terraformrc`

```
provider_installation {

  dev_overrides {
    "registry.terraform.io/crusoecloud/crusoe" = "/Users/{MY_USERNAME_HERE}/go/bin/"
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}
```

Run `make install` to build a provider and install it into your go-path. Then, you should be able to run `terraform apply` with the provided examples.


# Terraform Provider Crusoe

This repo defines the official Terraform Provider for use with [Crusoe Cloud](https://crusoecloud.com/), the world's first carbon-reducing, low-cost GPU cloud platform.

## Getting Started

To get started, first [install Terraform](https://developer.hashicorp.com/terraform/downloads). Then, get an access keypair from https://console.crusoecloud.com/security/tokens and add the following to `~/.crusoe/config`:

Note that Terraform does not read the `profile="env"` line at the top of the config file; it takes its environment directly from the `[default]` section. 

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

Add the following to your `~/.terraformrc`

```
provider_installation {

  dev_overrides {
    "registry.terraform.io/crusoecloud/crusoe" = "$GOPATH/bin/"
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}
```

Run `make install` to build a provider and install it into your go-path. Then, you should be able to run `terraform apply` with the provided examples.

Other common commands are: `terraform init` to initialize your working directory, and `terraform plan` to preview changes without applying them. 

## Versioning

A new version of the Crusoe Cloud Terraform provider is generated when there is a new merge request into the `release` branch in GitHub. 
This generates a new tag and triggers our `goreleaser` pipeline which will handle distributing the new Terraform version.

Our `main` branch is primarily used for development. Once features are ready to be deployed, a Crusoe Cloud maintainer will merge the changes from `main` into `release` to deploy a new version. 

## Contributing

We welcome (and have already had several!) open-source contributions to the Crusoe Cloud Terraform Provider.
Here is the workflow for contributing to the Crusoe Cloud Terraform provider:
1. Make a branch off `main` and open a pull request from your branch into `main`.
2. A Crusoe Cloud maintainer will review the pull request and, once approved, merge it into the `main` branch.
3. Once your pull request has been approved, make a separate pull request to add your changes to the changelog into the `main` branch. There will be an (Unreleased) version that you can add your changes to.
4. To release your changes, you can make a separate pull request from the `main` branch into the `release` branch. Merges into the release branch trigger our `goreleaser` job which handles distributing a new version.
5. Once the pull request has been approved and merged by a Crusoe Cloud maintainer, a new Terraform version will be released.
6. A separate pull request will be made by a Crusoe Cloud maintainer to update the changelog with the date the newest version has been released.

## Maintaining Changelog

The Crusoe Cloud changelog follows [Hashicorp's best practices](https://developer.hashicorp.com/terraform/plugin/best-practices/versioning) for versioning and changelog specifications.
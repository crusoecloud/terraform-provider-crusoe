# Terraform Provider Crusoe

This repo defines the official Terraform Provider for use with [Crusoe Cloud](https://crusoecloud.com/), the world's first carbon-reducing, low-cost GPU cloud platform.

## Examples

TODO

## Development

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

Create a new access token pair if you haven't yet at https://console.crusoecloud.com/security/tokens. 
These will go in the crusoe provider definition of your terraform files. Alternatively, you can define them as
environment variables:

```bash
export CRUSOE_ACCESS_KEY='MY_API_KEY'
export CRUSOE_SECRET_KEY='MY_SECRET_KEY'
```

Run `make install` to build a provider and install it into your go-path. From there, you should be able to run `terraform init && terraform apply` with the example. You'll be prompted for an API key and secret if you haven't specified them in your `.tf` file, but if you've set your env variables,

### Build provider

Run the following command to build the provider and move it to your go bin folder.

```shell
make install
```

Test the provider by running `terraform plan && terraform apply` in a directory with a `.tf` file, such as `examples/vms`.

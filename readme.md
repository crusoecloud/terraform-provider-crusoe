
# Terraform Provider Crusoe

This repo defines the official Terraform Provider for use with [Crusoe Cloud](https://crusoecloud.com/), the world's first carbon-reducing, low-cost GPU cloud platform.

## Getting Started

To get started, first [install Terraform](https://developer.hashicorp.com/terraform/downloads). Then, get an access keypair from https://console.crusoecloud.com/security/tokens and add the following to `~/.crusoe/config`:

Sample Config File:
```toml
[default]
access_key_id="MY_ACCESS_KEY"
secret_key="MY_SECRET_KEY"
```

# Profiles
You may specify different profiles to use in your `~/.crusoe/config` file

Example:
```
profile="profile1"

[profile1]
access_key_id="7useLCstekgdYQ8Te2vQxi"
secret_key="coKGIrMJudgAtk3YKMpsB2"
ssh_public_key_file="~/.ssh/id_rsa.pub"
api_endpoint="http://test:80/v1alpha5"

[profile2]
access_key_id="CstekgdYQ8Te2vQxi7useL"
secret_key="IrMJudgAtk3YKMpsB2coKG"
```

In the above, we have specified two profiles, where profile1 will be used as the default as we specify on the first line. If a value is not specified in the profile, the default value will be used. For example, profile2 did not specify an api_endpoint, so the default API endpoint to production will be used.

You may set which profile you use with the environment variable `CRUSOE_PROFILE` like `export CRUSOE_PROFILE=profile2`

## Provider Configuration

The provider block supports optional `profile` and `project` attributes:

```hcl
provider "crusoe" {
  profile = "production"  # Optional: profile from ~/.crusoe/config
  project = "my-project"  # Optional: project name or UUID
}
```

### Project Precedence

The project used for resources is determined by the following precedence (highest to lowest):

1. `project_id` attribute on individual resource/data source (UUID)
2. `project` argument in provider block (name or UUID)
3. `CRUSOE_DEFAULT_PROJECT` environment variable (name or UUID)
4. `default_project` from selected profile in `~/.crusoe/config`

### Profile Precedence

The profile used for credentials is determined by:

1. `profile` argument in provider block
2. `CRUSOE_PROFILE` environment variable
3. `profile` key in config file (top-level)
4. `"default"`

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

### Semantic Versioning

This provider follows semantic versioning (MAJOR.MINOR.PATCH):

- **MAJOR** (1.0.0 → 2.0.0): Breaking changes that require user action
- **MINOR** (0.5.0 → 0.6.0): New features, new resources/data sources, new attributes
- **PATCH** (0.5.42 → 0.5.43): Bug fixes, documentation updates, internal refactoring

Examples:
- New provider attribute → minor bump (0.5.42 → 0.6.0)
- New resource or data source → minor bump
- Bug fix → patch bump
- Breaking schema change → major bump (with UPGRADE NOTES in changelog)

### `versions.env`

The `versions.env` file at the repository root defines the current major and minor version numbers:

```bash
export MAJOR_VERSION=0
export MINOR_VERSION=6
```

Update this file **only for major or minor version bumps** when merging to `release`. Patch versions are auto-incremented by the release pipeline.

## Contributing

We welcome (and have already had several!) open-source contributions to the Crusoe Cloud Terraform Provider.
Here is the workflow for contributing to the Crusoe Cloud Terraform provider:
1. Make a branch off `main` and open a pull request from your branch into `main`.
2. A Crusoe Cloud maintainer will review the pull request and, once approved, merge it into the `main` branch.
3. To release: open a pull request from `main` into `release`. This PR must include a changelog entry for the new version (see Maintaining Changelog below).
4. Once the pull request has been approved and merged by a Crusoe Cloud maintainer, a new Terraform version will be released. Do not squash the commits, as it will cause the `main` and `release` branches to diverge.

## Maintaining Changelog

The Crusoe Cloud changelog follows [Hashicorp's best practices](https://developer.hashicorp.com/terraform/plugin/best-practices/versioning) for versioning and changelog specifications.

**Every merge to the `release` branch must include a changelog entry.** To add an entry:

1. Open `CHANGELOG.md` and add a new version section at the top
2. Increment the version number from the previous release (e.g., `0.5.45` → `0.5.46`)
3. Follow the category and format guidelines below

### Changelog Categories

Only include categories that have entries. Use dash-prefixed bullets. Keep descriptions concise but informative.

- `ENHANCEMENTS:` - Smaller features added to an existing resource or data source, such as a new attribute.
- `BUG FIXES:` - Any bugs that were fixed.
- `NEW FEATURES:` - Major new improvements such as a new resource or data source.
- `UPGRADE NOTES:` - Breaking or incompatible changes and how to handle them.

### Example

```markdown
## 0.6.0

ENHANCEMENTS:

- Added `profile` attribute to provider block for config file profile selection
```

### When a Changelog Entry is Needed

- New provider/resource/data source attributes
- Behavioral changes
- Bug fixes
- Breaking changes (always include UPGRADE NOTES)
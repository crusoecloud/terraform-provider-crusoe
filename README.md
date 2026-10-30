
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
api_endpoint="http://test:80/v1"

[profile2]
access_key_id="CstekgdYQ8Te2vQxi7useL"
secret_key="IrMJudgAtk3YKMpsB2coKG"
```

In the above, we have specified two profiles, where profile1 will be used as the default as we specify on the first line. If a value is not specified in the profile, the default value will be used. For example, profile2 did not specify an api_endpoint, so the default API endpoint to production will be used.

You may set which profile you use with the environment variable `CRUSOE_PROFILE` like `export CRUSOE_PROFILE=profile2`

## Service Account Authentication

As an alternative to an access keypair, you can authenticate as a [service account](https://console.crusoecloud.com) using OAuth2 client credentials. This is often preferable for automation (e.g. CI/CD) since service account credentials can be scoped and rotated independently of a human user's keypair.

Sample Config File:
```toml
[default]
client_id="MY_CLIENT_ID"
client_secret="MY_CLIENT_SECRET"
```

Like `access_key_id`/`secret_key`, these can also be set per-profile, or via the `CRUSOE_CLIENT_ID` and `CRUSOE_CLIENT_SECRET` environment variables (which take precedence over the config file, following the same precedence rules as the API key credentials above).

The provider automatically fetches and refreshes the OAuth2 access token as needed against Crusoe's token endpoint (`https://auth.crusoe.ai/oauth2/token` by default) — no manual token management is required. Every token request also carries an `audience` (`https://api.crusoe.ai` by default), since the API rejects a service-account token whose `aud` claim doesn't match. To point at a different token endpoint or audience (e.g. for a non-production environment), set `token_url`/`audience` in the config file or the `CRUSOE_TOKEN_URL`/`CRUSOE_AUDIENCE` environment variables; the vast majority of users should never need this, and if you set one you almost always need to set the other too.

**Configure exactly one authentication method.** You must set either the API key pair (`access_key_id` + `secret_key`) or the service account pair (`client_id` + `client_secret`) — not both, and not just one field of a pair:

- If neither pair is fully configured, the provider fails with a "Missing Crusoe Credentials" error.
- If only one field of a pair is set (e.g. `client_id` without `client_secret`), the provider fails with an "Incomplete ... Credentials" error naming the missing field.
- If both pairs are fully configured at the same time, the provider fails with a "Conflicting Crusoe Credentials" error rather than silently preferring one — this avoids accidentally authenticating as the wrong identity.

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

This repository includes a `mise.toml` that pins the Go toolchain along with `golangci-lint`, `terraform`, and `tfplugindocs`. If you use [mise](https://mise.jdx.dev), run `mise install` to provision them; otherwise install a recent version of [golang](https://go.dev/doc/install) yourself.

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

Run `make install` to build a provider and install it into your go-path. Then, you should be able to run `terraform apply` with the provided examples. (Under mise this lands in mise's Go bin, which is where the `$GOPATH/bin/` override above resolves, so the snippet works unchanged.)

Other common commands are: `terraform init` to initialize your working directory, and `terraform plan` to preview changes without applying them. 

Schema attribute descriptions are derived from the `github.com/crusoecloud/client-go` swagger spec (the source of truth). When adding schema fields, generate the text with the `/derive-schema-descriptions` Claude Code skill instead of hand-writing it, then regenerate docs with `make docs`.

## Versioning

A new version of the Crusoe Cloud Terraform provider is generated when there is a new merge request into the `release` branch in GitHub.
This generates a new tag and triggers our `goreleaser` pipeline which will handle distributing the new Terraform version.
On a successful release, the pipeline posts an announcement (version, commit, and a link to the GitHub release) to the `#ccx-ci-releases` Slack channel.
These notifications require the `SLACK_CCX_CI_RELEASES_WEBHOOK_URL` webhook URL to be configured as both a GitHub Actions secret (release announcement) and a GitLab CI/CD variable (pipeline status notifications); if unset, the notification steps log a message and skip without failing the release.

`main` is releasable at all times: anything merged into it can go out in the next release. A change whose backend is not in production yet, or that customers cannot reach yet, waits in its merge request or goes in behind a feature flag with the `skip-changelog` label (see [`.changelog/README.md`](.changelog/README.md), "Changes behind a feature flag"). A Crusoe Cloud maintainer merges `main` into `release` to publish a version.

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

Update this file **only for major or minor version bumps**, in the same release-prep commit on `main` that adds the changelog section (`make changelog VERSION=vX.Y.0`), before the release MR is opened and never on the `release` branch. Patch versions are auto-incremented by the release pipeline.

## Contributing

We welcome (and have already had several!) open-source contributions to the Crusoe Cloud Terraform Provider.
Here is the workflow for contributing to the Crusoe Cloud Terraform provider:
1. Make a branch off `main` and open a merge request from your branch into `main`.
2. A Crusoe Cloud maintainer reviews the merge request and merges it into `main` once it can ship: `main` is releasable at all times, so a change that depends on a backend not yet in production waits in the merge request, or goes in behind a feature flag. Every merge request with a changelog fragment, which is every customer-visible change, needs an approval from the CCX squad: `.changelog/` is theirs in `.gitlab/CODEOWNERS`, together with `CHANGELOG.md`, `versions.env` and `scripts/`.
3. Add a changelog fragment to the merge request, `.changelog/<MR>.txt` (see Maintaining Changelog below). CI fails a merge request that needs one and has none.
4. Before a release, a maintainer runs `make changelog` on a branch off `main`, which renders the fragments merged since the last release into a new section of `CHANGELOG.md`, and merges that release-prep commit into `main` (with the `versions.env` bump for a minor or major release).
5. To release: open a merge request from `main` into `release`. Its review checks the rendered changelog entry by entry; it does not hold features back, since everything on `main` can ship.
6. Once the merge request has been approved and merged by a Crusoe Cloud maintainer, a new Terraform version will be released. Do not squash the commits, as it will cause the `main` and `release` branches to diverge.

## Maintaining Changelog

The Crusoe Cloud changelog follows [Hashicorp's best practices](https://developer.hashicorp.com/terraform/plugin/best-practices/versioning) for versioning and changelog specifications.

`CHANGELOG.md` is generated from fragments: one file per merge request in [`.changelog/`](.changelog/README.md), named `.changelog/<MR>.txt`, with one fenced `release-note:<type>` block per customer-visible change. That README has the types, the format and the product tag that turns an entry into a post on [Crusoe Updates](https://docs.crusoecloud.com/updates/). In Claude Code, `/changelog` drafts the fragment from the branch diff; `make changelog-check FILE=.changelog/<MR>.txt` validates it. CI (`changelog_check`) fails a merge request without a fragment unless it carries the `skip-changelog` label or only touches files no customer can notice.

### Preparing a release

`make changelog` renders the fragments merged since the last release tag into a new `## vX.Y.Z (date)` section at the top of `CHANGELOG.md`; `make changelog-preview` prints it without editing the file. The version is the next patch of `versions.env`, the one the release pipeline will tag; for a minor or major release run `make changelog VERSION=vX.Y.0` and bump `versions.env` in the same commit. Commit the section to `main` in a release-prep commit **before** the release MR is opened. Never add it on the `release` branch or inside the release MR, which would make `release` diverge from `main` and conflict on the next merge. Until the version is tagged, running `make changelog` again replaces the section, so a wrong entry is fixed in its fragment and rendered again rather than edited in `CHANGELOG.md`.

The release tags live on GitHub, where the release workflow creates them; `make changelog` fetches them itself. Sections up to 1.8.0 were written by hand and keep their format.

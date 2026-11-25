terraform {
  required_providers {
    crusoe = {
      source = "crusoecloud/crusoe"
    }
  }
}

# Credentials come from ~/.crusoe/config or the CRUSOE_* environment variables.
# All attributes are optional.
provider "crusoe" {
  profile = "default"    # Profile in ~/.crusoe/config.
  project = "my-project" # Default project, as a name or UUID.
}

# A second configuration for resources in another project.
provider "crusoe" {
  alias   = "staging"
  project = "staging-project"
}

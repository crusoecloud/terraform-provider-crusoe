//go:build tools

package tools

import (
	// Import used for Terraform public doc autogeneration
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)

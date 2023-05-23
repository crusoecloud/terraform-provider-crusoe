// Meant for development - any changes targeting the released provider should be made in main.go.
package main

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/crusoecloud/terraform-provider-crusoe/crusoe"
)

func main() {
	err := providerserver.Serve(context.Background(), crusoe.New, providerserver.ServeOpts{
		// overridden during local development by an override that should be set in ~/.terraformrc
		Address: "registry.terraform.io/crusoecloud/crusoe",
	})
	if err != nil {
		// this should never occur since Terraform is responsible for serving the plugin.
		panic(err)
	}
}

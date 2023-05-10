package main

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"terraform-provider-crusoe/crusoe"
)

func main() {
	err := providerserver.Serve(context.Background(), crusoe.New, providerserver.ServeOpts{
		// TODO: verify this is correct after publishing
		Address: "registry.terraform.io/crusoecloud/crusoe",
	})
	if err != nil {
		// this should never occur since Terraform is responsible for serving the plugin.
		panic(err)
	}
}

package main

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/crusoecloud/terraform-provider-crusoe/crusoe"
)

//go:generate go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs

func main() {
	err := providerserver.Serve(context.Background(), crusoe.New, providerserver.ServeOpts{
		Address: "registry.terraform.io/crusoecloud/crusoe",
	})
	if err != nil {
		// this should never occur since Terraform is responsible for serving the plugin.
		panic(err)
	}
}

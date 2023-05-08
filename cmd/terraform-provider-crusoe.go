package main

import (
	"context"
	"terraform-provider-crusoe/crusoe"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	// "github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
	err := providerserver.Serve(context.Background(), crusoe.New, providerserver.ServeOpts{
		// NOTE: This is not a typical Terraform Registry provider address,
		// such as registry.terraform.io/hashicorp/hashicups. This specific
		// provider address is used in these tutorials in conjunction with a
		// specific Terraform CLI configuration for manual development testing
		// of this provider.
		// TODO: verify this is correct after publishing
		Address: "registry.terraform.io/crusoecloud/crusoe",
	})
	if err != nil {
		// this shouldn't occur since Terraform is responsible for executing the plugin.
		panic(err)
	}
}

// terraform-provider-statusharbor manages Status Harbor resources via
// Terraform. v1 ships only the statusharbor_lighthouse resource; more
// resources land incrementally as customers ask.
//
// See: https://registry.terraform.io/providers/statusharbor/statusharbor
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/statusharbor/terraform-provider-statusharbor/internal/provider"
)

// version is set at build time via -ldflags. Defaults to "dev" so
// `go run` works for local development.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/statusharbor/statusharbor",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err.Error())
	}
}

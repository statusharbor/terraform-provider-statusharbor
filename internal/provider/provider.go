// Package provider is the entry point for the Status Harbor Terraform
// provider. It wires up the schema, configuration, and resources.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/statusharbor/terraform-provider-statusharbor/internal/client"
)

// Production API endpoint. Hardcoded so customers can't accidentally
// redirect Terraform at a stranger's Console (same security stance
// as the Lighthouse agent's hardcoded ConsoleURL).
//
// Overridable at build time via:
//
//	go build -ldflags="-X github.com/statusharbor/terraform-provider-statusharbor/internal/provider.apiBaseURL=https://staging.example/"
//
// Used by acceptance tests against a local Status Harbor instance.
var apiBaseURL = "https://terraform.statusharbor.io"

var _ provider.Provider = (*statusharborProvider)(nil)

type statusharborProvider struct {
	version string
}

// New returns a constructor that the providerserver invokes once per
// plugin instance. The version is baked in at build time.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &statusharborProvider{version: version}
	}
}

func (p *statusharborProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "statusharbor"
	resp.Version = p.version
}

func (p *statusharborProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Status Harbor resources via Terraform.",
		Attributes: map[string]schema.Attribute{
			"api_token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "team:admin API token. Required. May be omitted in HCL " +
					"and supplied via the STATUSHARBOR_API_TOKEN environment variable. " +
					"Mint a token in the Console under Settings → API Tokens with scope team:admin.",
			},
		},
	}
}

// providerConfig is the parsed HCL block.
type providerConfig struct {
	APIToken types.String `tfsdk:"api_token"`
}

func (p *statusharborProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerConfig
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	token := cfg.APIToken.ValueString()
	if token == "" {
		token = os.Getenv("STATUSHARBOR_API_TOKEN")
	}
	if token == "" {
		resp.Diagnostics.AddError(
			"missing api_token",
			"Set the api_token provider attribute or STATUSHARBOR_API_TOKEN environment variable. "+
				"Mint a team:admin token in the Status Harbor Console under Settings → API Tokens.",
		)
		return
	}

	c := client.New(apiBaseURL, token, p.version)
	resp.ResourceData = c
	resp.DataSourceData = c
}

func (p *statusharborProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewLighthouseResource,
	}
}

func (p *statusharborProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewLighthouseDataSource,
	}
}

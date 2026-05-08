// Acceptance tests for statusharbor_lighthouse. Run with:
//
//	export STATUSHARBOR_API_TOKEN=...
//	export STATUSHARBOR_API_URL=https://staging.example  # optional, only for non-prod
//	TF_ACC=1 go test -race -timeout 300s ./internal/provider/...
//
// Tests skip when TF_ACC isn't set so `go test ./...` stays
// fast and offline. They hit a real Status Harbor instance —
// the test config creates and tears down lighthouses, so point
// at staging or local-dev rather than production.

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// providerFactories registers the provider for acceptance tests using
// the framework's protocol-v6 server. Single instance reused across
// test cases.
var providerFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"statusharbor": providerserver.NewProtocol6WithError(New("test")()),
}

func testAccPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("STATUSHARBOR_API_TOKEN") == "" {
		t.Skip("STATUSHARBOR_API_TOKEN not set; skipping real-Console acceptance test")
	}
}

// 1) Minimal lifecycle: create → read → destroy.
func TestAccLighthouseResource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLighthouseConfigBasic("tf-acc-basic"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("statusharbor_lighthouse.test", "name", "tf-acc-basic"),
					resource.TestCheckResourceAttrSet("statusharbor_lighthouse.test", "id"),
					resource.TestCheckResourceAttrSet("statusharbor_lighthouse.test", "token"),
					resource.TestCheckResourceAttrSet("statusharbor_lighthouse.test", "created_at"),
				),
			},
		},
	})
}

// 2) Update flow: create with defaults, then PATCH paused + flap.
func TestAccLighthouseResource_update(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLighthouseConfigBasic("tf-acc-update"),
				Check: resource.TestCheckResourceAttr(
					"statusharbor_lighthouse.test", "paused", "false",
				),
			},
			{
				Config: testAccLighthouseConfigPausedFlap("tf-acc-update", true, 3),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("statusharbor_lighthouse.test", "paused", "true"),
					resource.TestCheckResourceAttr("statusharbor_lighthouse.test", "flap_protection_threshold", "3"),
				),
			},
		},
	})
}

// 3) Import: bring an existing Lighthouse into state by UUID.
func TestAccLighthouseResource_import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccLighthouseConfigBasic("tf-acc-import"),
			},
			{
				ResourceName:      "statusharbor_lighthouse.test",
				ImportState:       true,
				ImportStateVerify: true,
				// token isn't returned on read; ignore on verify.
				ImportStateVerifyIgnore: []string{"token"},
			},
		},
	})
}

func testAccLighthouseConfigBasic(name string) string {
	return fmt.Sprintf(`
resource "statusharbor_lighthouse" "test" {
  name = %q
}
`, name)
}

func testAccLighthouseConfigPausedFlap(name string, paused bool, flap int) string {
	return fmt.Sprintf(`
resource "statusharbor_lighthouse" "test" {
  name                      = %q
  paused                    = %t
  flap_protection_threshold = %d
}
`, name, paused, flap)
}

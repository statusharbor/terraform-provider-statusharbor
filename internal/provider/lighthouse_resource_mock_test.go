// Hermetic acceptance tests for statusharbor_lighthouse, against an
// in-process fake Console. No real Status Harbor instance required;
// runs in CI on every PR via `go test ./...`.
//
// Companion to lighthouse_resource_test.go which targets a real
// Console (gated on TF_ACC=1).

package provider

import (
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// hermeticTest sets TF_ACC=1 (otherwise the framework skips the
// test) and points the provider at the fakeConsole. Callers receive
// a TestCase fully wired against the fake; they only supply Steps.
func hermeticTest(t *testing.T, steps ...resource.TestStep) {
	t.Helper()
	if err := os.Setenv("TF_ACC", "1"); err != nil {
		t.Fatalf("setenv: %v", err)
	}

	fc := newFakeConsole(t)
	setProviderBaseURL(t, fc.URL())

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps:                    steps,
	})
}

// 1) Basic create + read: declares one lighthouse, asserts the
//    sensitive token output and computed fields populate.
func TestAccLighthouseResource_basic_mock(t *testing.T) {
	hermeticTest(t,
		resource.TestStep{
			Config: providerConfigHCL + `
resource "statusharbor_lighthouse" "test" {
  name = "tf-mock-basic"
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("statusharbor_lighthouse.test", "name", "tf-mock-basic"),
				resource.TestCheckResourceAttrSet("statusharbor_lighthouse.test", "id"),
				resource.TestCheckResourceAttrSet("statusharbor_lighthouse.test", "token"),
				resource.TestCheckResourceAttrSet("statusharbor_lighthouse.test", "created_at"),
				resource.TestCheckResourceAttr("statusharbor_lighthouse.test", "paused", "false"),
				resource.TestCheckResourceAttr("statusharbor_lighthouse.test", "notify_on_lifecycle", "true"),
			),
		},
	)
}

// 2) Update flow: changes paused + flap_protection_threshold and
//    asserts the change was applied.
func TestAccLighthouseResource_update_mock(t *testing.T) {
	configWithDefaults := providerConfigHCL + `
resource "statusharbor_lighthouse" "test" {
  name = "tf-mock-update"
}
`
	configWithEdits := providerConfigHCL + `
resource "statusharbor_lighthouse" "test" {
  name                      = "tf-mock-update"
  paused                    = true
  flap_protection_threshold = 3
}
`

	hermeticTest(t,
		resource.TestStep{
			Config: configWithDefaults,
			Check:  resource.TestCheckResourceAttr("statusharbor_lighthouse.test", "paused", "false"),
		},
		resource.TestStep{
			Config: configWithEdits,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("statusharbor_lighthouse.test", "paused", "true"),
				resource.TestCheckResourceAttr("statusharbor_lighthouse.test", "flap_protection_threshold", "3"),
			),
		},
	)
}

// 3) Rename is rejected — current API doesn't expose rename via
//    PATCH, so the provider refuses cleanly with a useful message.
func TestAccLighthouseResource_renameRefused_mock(t *testing.T) {
	hermeticTest(t,
		resource.TestStep{
			Config: providerConfigHCL + `
resource "statusharbor_lighthouse" "test" {
  name = "tf-mock-original"
}
`,
		},
		resource.TestStep{
			Config: providerConfigHCL + `
resource "statusharbor_lighthouse" "test" {
  name = "tf-mock-renamed"
}
`,
			ExpectError: regexpRenameRefused,
		},
	)
}

// 4) Import: existing fakeConsole row brought into state by id;
//    subsequent plan is empty.
func TestAccLighthouseResource_import_mock(t *testing.T) {
	hermeticTest(t,
		resource.TestStep{
			Config: providerConfigHCL + `
resource "statusharbor_lighthouse" "test" {
  name = "tf-mock-import"
}
`,
		},
		resource.TestStep{
			ResourceName:      "statusharbor_lighthouse.test",
			ImportState:       true,
			ImportStateVerify: true,
			// token is mint-once and never returned on read; ignore on verify.
			ImportStateVerifyIgnore: []string{"token"},
		},
	)
}

// 5) Drift recovery: when the resource is deleted out-of-band,
//    the next plan reports it for re-creation rather than failing.
func TestAccLighthouseResource_outOfBandDelete_mock(t *testing.T) {
	fc := newFakeConsole(t)
	setProviderBaseURL(t, fc.URL())
	if err := os.Setenv("TF_ACC", "1"); err != nil {
		t.Fatalf("setenv: %v", err)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfigHCL + `
resource "statusharbor_lighthouse" "test" {
  name = "tf-mock-drift"
}
`,
				Check: resource.TestCheckResourceAttrSet("statusharbor_lighthouse.test", "id"),
			},
			{
				// Wipe the fake's storage to simulate someone deleting
				// the Lighthouse via the dashboard. The next refresh
				// should mark it for re-creation.
				PreConfig: func() {
					fc.mu.Lock()
					for k := range fc.lighthouse {
						delete(fc.lighthouse, k)
					}
					fc.mu.Unlock()
				},
				Config: providerConfigHCL + `
resource "statusharbor_lighthouse" "test" {
  name = "tf-mock-drift"
}
`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// regexpRenameRefused matches the error message we surface in the
// resource's Update() when a rename is attempted.
var regexpRenameRefused = regexp.MustCompile(`renaming a Lighthouse is not supported`)

.PHONY: build test testacc lint docs install

# Local install path used by ~/.terraformrc dev_overrides during
# development. Override OS/ARCH if not on darwin/arm64.
HOSTNAME      := registry.terraform.io
NAMESPACE     := statusharbor
NAME          := statusharbor
VERSION       := 0.0.1
OS_ARCH       ?= darwin_arm64
INSTALL_PATH  := ~/.terraform.d/plugins/$(HOSTNAME)/$(NAMESPACE)/$(NAME)/$(VERSION)/$(OS_ARCH)

build:
	go build -o terraform-provider-statusharbor

test:
	go test -race -timeout 120s ./...

# Acceptance tests run real `terraform apply` against the configured
# Console. Set STATUSHARBOR_API_TOKEN and STATUSHARBOR_API_URL (the
# latter only needed for non-prod targets — prod is hardcoded).
testacc:
	TF_ACC=1 go test -race -timeout 300s ./internal/provider/...

lint:
	golangci-lint run --timeout=5m

docs:
	@which tfplugindocs > /dev/null || go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
	tfplugindocs generate

# Install the provider into the local Terraform plugin directory so
# you can `terraform apply` the examples without GoReleaser. Pair with
# a dev_overrides block in ~/.terraformrc:
#
#   provider_installation {
#     dev_overrides {
#       "statusharbor/statusharbor" = "/Users/<you>/go/bin"
#     }
#     direct {}
#   }
install: build
	mkdir -p $(INSTALL_PATH)
	mv terraform-provider-statusharbor $(INSTALL_PATH)/

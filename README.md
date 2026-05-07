# terraform-provider-statusharbor

Terraform provider for [Status Harbor](https://www.statusharbor.io). Manage
Lighthouse agents (and, in upcoming releases, monitors and status pages)
declaratively.

## Status

**v0.x** — Pre-release. Currently exposes:

- `statusharbor_lighthouse` (resource + data source)

More resources land incrementally as customers ask. The wire contract
behind the provider is governed by the
[`/v1/` stability policy](https://github.com/statusharbor/status-harbor/blob/master/docs/LIGHTHOUSE_API.md).

## Quick start

```hcl
terraform {
  required_providers {
    statusharbor = {
      source = "statusharbor/statusharbor"
    }
  }
}

provider "statusharbor" {
  # Or set STATUSHARBOR_API_TOKEN
  api_token = var.statusharbor_api_token
}

resource "statusharbor_lighthouse" "prod_vpc" {
  name = "prod-vpc"
}

output "agent_token" {
  value     = statusharbor_lighthouse.prod_vpc.token
  sensitive = true
}
```

Pipe `agent_token` into the [`terraform-lighthouse`](https://github.com/statusharbor/terraform-lighthouse)
modules (Helm / Docker / cloud-init) to actually deploy the agent.

## Configuration

| Field        | Required | Env var                    | Description                              |
| ------------ | -------- | -------------------------- | ---------------------------------------- |
| `api_token`  | yes      | `STATUSHARBOR_API_TOKEN`   | A `team:admin`-scoped token. Mint one in the Console under Settings → API Tokens. |

The provider talks to `https://terraform.statusharbor.io` (hardcoded for
production, same security model as the Lighthouse agent's pinned
`ConsoleURL`). Override at build time via `-ldflags` for local
development against a staging Console.

## Resources

### `statusharbor_lighthouse`

A Lighthouse agent registered to your team. Apply mints the bearer token
the agent needs to authenticate; subsequent applies reconcile metadata
(host hint, paused, flap protection, lifecycle notifications).

```hcl
resource "statusharbor_lighthouse" "homelab" {
  name                      = "homelab"
  notify_on_lifecycle       = true
  flap_protection_threshold = 2
  paused                    = false
}
```

The `token` attribute is **sensitive** and persists in your Terraform
state file. Use a remote, encrypted state backend (Terraform Cloud,
S3+KMS, GCS+KMS) to avoid leaking it.

#### Importing existing Lighthouses

```bash
terraform import statusharbor_lighthouse.foo 7eebc923-44bc-4d10-...
```

The UUID is in the URL of the Lighthouse detail page in the Console
(`/lighthouses/<uuid>`). Note: `terraform import` can't recover the
agent token — the imported state will have an empty `token`. If you
need to rotate, delete and recreate the resource.

## Data sources

### `statusharbor_lighthouse`

Look up an existing Lighthouse by id. Useful when the Lighthouse is
managed in another workspace and you want to read its metadata without
adopting it.

## Development

```bash
go build ./...
go test ./...
```

Acceptance tests against a local Console: see `examples/` for the test
configurations.

## License

Apache 2.0 — see [`LICENSE`](LICENSE).

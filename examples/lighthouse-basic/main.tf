/**
 * Minimal example — declares a single Lighthouse and exposes its
 * agent token as a sensitive output.
 *
 * Usage:
 *   export STATUSHARBOR_API_TOKEN=...
 *   terraform init
 *   terraform apply
 *
 * Pipe `terraform output -raw agent_token` into the
 * terraform-lighthouse modules (Helm / Docker / cloud-init) to
 * actually deploy the agent.
 */

terraform {
  required_providers {
    statusharbor = {
      source = "statusharbor/statusharbor"
    }
  }
}

provider "statusharbor" {}

resource "statusharbor_lighthouse" "example" {
  name = "tf-example"
}

output "lighthouse_id" {
  value = statusharbor_lighthouse.example.id
}

output "agent_token" {
  value     = statusharbor_lighthouse.example.token
  sensitive = true
}

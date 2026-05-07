resource "statusharbor_lighthouse" "prod_vpc" {
  name                      = "prod-vpc"
  notify_on_lifecycle       = true
  flap_protection_threshold = 1
  paused                    = false
}

# The bearer token is sensitive and only present in state. Use a remote,
# encrypted backend (Terraform Cloud, S3+KMS, GCS+KMS) to keep it safe.
output "agent_token" {
  value     = statusharbor_lighthouse.prod_vpc.token
  sensitive = true
}

terraform {
  required_providers {
    statusharbor = {
      source = "statusharbor/statusharbor"
    }
  }
}

# api_token can be omitted from HCL and supplied via the
# STATUSHARBOR_API_TOKEN environment variable. Mint a token
# with scope "team:admin" in the Status Harbor Console under
# Settings → API Tokens.
provider "statusharbor" {
  api_token = var.statusharbor_api_token
}

variable "statusharbor_api_token" {
  type      = string
  sensitive = true
  default   = null
}

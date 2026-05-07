#!/usr/bin/env bash
# Import an existing Lighthouse into Terraform state by its UUID. Find
# the UUID in the URL of the Lighthouse detail page in the Console
# (https://console.statusharbor.io/lighthouses/<uuid>).
#
# Note: import cannot recover the agent's bearer token — the imported
# state will have an empty `token` attribute. To rotate, delete and
# recreate the resource.
terraform import statusharbor_lighthouse.example 7eebc923-44bc-4d10-a856-cd62330bc441

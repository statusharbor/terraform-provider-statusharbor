data "statusharbor_lighthouse" "existing" {
  id = "7eebc923-44bc-4d10-a856-cd62330bc441"
}

output "agent_hostname" {
  value = data.statusharbor_lighthouse.existing.agent_hostname
}

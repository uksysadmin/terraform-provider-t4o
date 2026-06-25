output "workload_id" {
  value = t4o_workload.workload.id
}

output "backup_target_id" {
  value = local.backup_target_id
}

output "policy_id" {
  value = local.policy_id
}

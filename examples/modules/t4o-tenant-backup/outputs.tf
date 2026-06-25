output "workload_id" {
  description = "ID of the created T4O workload."
  value       = t4o_workload.this.id
}

output "backup_target_id" {
  description = "Resolved backup target ID the workload was attached to."
  value       = local.resolved_backup_target_id
}

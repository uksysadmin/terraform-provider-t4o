output "vm_ids" {
  description = "IDs of this tenant's VMs (created in the tenant project)."
  value       = openstack_compute_instance_v2.vm[*].id
}

output "workload_id" {
  description = "ID of the T4O workload protecting the VM."
  value       = module.backup.workload_id
}

output "backup_target_id" {
  description = "Backup target the workload was attached to."
  value       = module.backup.backup_target_id
}

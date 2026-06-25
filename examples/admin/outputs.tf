# These outputs are the contract the per-tenant stacks (../tenant) consume.

output "backup_target_id" {
  description = "Shared backup target ID."
  value       = t4o_backup_target.shared.id
}

output "backup_target_name" {
  description = "Shared backup target name (tenants can look it up by name)."
  value       = t4o_backup_target.shared.name
}

output "policy_id" {
  description = "Shared policy ID — pass this to each tenant stack as policy_id."
  value       = t4o_workload_policy.shared.id
}

output "tenants" {
  description = "Per-tenant identity + network, for wiring up the tenant stacks. The tenant stack creates its own VM in network_id (admin no longer creates VMs)."
  value = {
    for name, _ in var.tenants : name => {
      project_id = local.tenant_project_ids[name]
      user_id    = local.tenant_user_ids[name]
      network_id = var.manage_projects ? try(module.tenant_project[name].network_id, null) : null
    }
  }
}

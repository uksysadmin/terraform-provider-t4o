output "granted_roles" {
  description = "Roles granted to the user on the project."
  value       = var.roles
}

output "role_assignment_ids" {
  description = "IDs of the created role assignments (use for explicit downstream ordering)."
  value       = [for a in openstack_identity_role_assignment_v3.grant : a.id]
}

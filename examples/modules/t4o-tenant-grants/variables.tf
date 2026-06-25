variable "user_id" {
  description = "Keystone user ID to grant the roles to (the tenant's T4O admin user)."
  type        = string
}

variable "project_id" {
  description = "Project ID the roles are scoped to (the tenant's project)."
  type        = string
}

variable "roles" {
  description = <<-EOT
    Roles to grant the user on the project. Must already exist in Keystone.
    Defaults cover both gates T4O imposes:
      - "backup_admin": T4O custom RBAC role for workload ops
      - "_member_", "creator": Keystone trustee roles WLM delegates into the
        per-workload trust (match WLM's `trustee_role` config; "creator" needs Barbican).
    Secure-RBAC clouds typically use "member" instead of "_member_".
  EOT
  type        = list(string)
  default     = ["backup_admin", "_member_", "creator"]
}

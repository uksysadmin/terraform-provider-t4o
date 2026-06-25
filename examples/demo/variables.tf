variable "auth_url" {
  description = "OpenStack Keystone endpoint, e.g. http://10.0.0.1:5000"
  type        = string
}

variable "admin_username" {
  description = "Cloud-admin username used by both the OpenStack and T4O providers."
  type        = string
  default     = "admin"
}

variable "admin_project_name" {
  description = "Project the admin user authenticates against (tenant_name)."
  type        = string
  default     = "admin"
}

variable "admin_password" {
  description = "Admin OpenStack password."
  type        = string
  sensitive   = true
}

variable "admin_project_id" {
  description = "UUID of the admin project (for T4O provider scoping)."
  type        = string
}

variable "domain_name" {
  description = "OpenStack identity domain."
  type        = string
  default     = "Default"
}

variable "external_network_name" {
  description = "Name of the external network (usually 'public' or 'external')."
  type        = string
  default     = "public"
}

variable "image_name" {
  description = "Glance image name to boot VMs from (e.g. cirros-0.6.2, ubuntu-22.04)."
  type        = string
  default     = "cirros"
}

variable "flavor_name" {
  description = "Nova flavor for the demo VMs. Must exist in the cloud (RHOSO/Canonical may not have m1.tiny)."
  type        = string
  default     = "m1.tiny"
}

variable "member_role_name" {
  description = "Keystone role granted to per-project users (usually 'member', older clouds '_member_')."
  type        = string
  default     = "member"
}

variable "manage_trustee_roles" {
  description = "If true, grant the auth user the WLM trustee roles on its project so workload-create trusts succeed. Requires admin and that the roles already exist in Keystone."
  type        = bool
  default     = false
}

variable "trustee_roles" {
  description = "Roles WLM delegates into the workload trust (must match WLM's trustee_role config). Stock T4O default is [\"_member_\", \"creator\"]; secure-RBAC clouds may use [\"member\", \"creator\"]. 'creator' requires Barbican."
  type        = list(string)
  default     = ["_member_", "creator"]
}

variable "nfs_export" {
  description = "NFS export path from your storage, e.g. 10.0.0.5:/exports/tvault"
  type        = string
}

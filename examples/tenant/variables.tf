# ── Tenant credentials (this tenant's own user, scoped to its project) ───────
variable "auth_url" {
  description = "OpenStack Keystone endpoint. Omit to use OS_AUTH_URL from this tenant's sourced openrc."
  type        = string
  default     = null
}

variable "tenant_username" {
  description = "This tenant's username (holds the trust role(s); member of the project). Omit to use OS_USERNAME."
  type        = string
  default     = null
}

variable "tenant_password" {
  description = "This tenant's password. Omit to use OS_PASSWORD from this tenant's sourced openrc."
  type        = string
  sensitive   = true
  default     = null
}

variable "tenant_project_id" {
  description = "This tenant's project UUID (admin output tenants[<name>].project_id). Omit to use OS_PROJECT_ID."
  type        = string
  default     = null
}

variable "domain_name" {
  description = "OpenStack identity domain."
  type        = string
  default     = "Default"
}

# ── Shared resources + network (from the admin stack outputs) ─────────────
variable "policy_id" {
  description = "Shared workload policy ID, assigned to this tenant's project (admin output `policy_id`). Leave \"\" to use jobschedule instead."
  type        = string
  default     = ""
}

variable "jobschedule" {
  description = "Inline schedule for the workload (mutually exclusive with policy_id). Set to test the non-policy path; leave null to use the shared policy."
  type = object({
    enabled             = bool
    start_date          = optional(string)
    end_date            = optional(string)
    interval            = optional(string)
    fullbackup_interval = optional(number)
    retention_days      = optional(number)
    snapshots_to_retain = optional(number)
  })
  default = null
}

variable "backup_target_id" {
  description = "Shared backup target ID (admin output `backup_target_id`)."
  type        = string
}

variable "network_id" {
  description = "Network in this tenant's project to boot the VM into (admin output tenants[<name>].network_id)."
  type        = string
}

# ── This tenant's VM/workload ────────────────────────────────────────────────
variable "workload_name" {
  description = "Name for this tenant's workload (and VM)."
  type        = string
  default     = "tenant-workload"
}

variable "image_name" {
  description = "Glance image to boot (must exist; e.g. cirros-0.6.2)."
  type        = string
  default     = "cirros"
}

variable "flavor_name" {
  description = "Nova flavor for the VM (must exist; e.g. m1.tiny)."
  type        = string
  default     = "m1.tiny"
}

variable "vm_count" {
  description = "Number of VMs to create in this tenant project and protect with one workload. >1 exercises the multi-instance round-trip path."
  type        = number
  default     = 1
}

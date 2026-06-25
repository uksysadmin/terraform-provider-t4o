# ── Cloud connection (admin) ─────────────────────────────────────────────────
variable "auth_url" {
  description = "OpenStack Keystone endpoint, e.g. http://10.0.0.1:5000"
  type        = string
}

variable "admin_username" {
  description = "Cloud-admin username."
  type        = string
  default     = "admin"
}

variable "admin_password" {
  description = "Cloud-admin password."
  type        = string
  sensitive   = true
}

variable "admin_project_id" {
  description = "UUID of the admin project (for T4O provider scoping)."
  type        = string
}

variable "admin_project_name" {
  description = "Admin project name (tenant_name for the openstack provider)."
  type        = string
  default     = "admin"
}

variable "domain_name" {
  description = "OpenStack identity domain."
  type        = string
  default     = "Default"
}

# ── Tenants ──────────────────────────────────────────────────────────────────
variable "tenants" {
  description = <<-EOT
    Map of tenants to provision/onboard, keyed by name. When manage_projects = true
    only subnet_cidr is used (project/user/VM are created). When manage_projects =
    false, supply project_id and user_id of the pre-existing tenant.
  EOT
  type = map(object({
    subnet_cidr = optional(string, "10.20.0.0/24")
    project_id  = optional(string, "")
    user_id     = optional(string, "")
  }))
  default = {
    "demo-tenant-a" = { subnet_cidr = "10.20.1.0/24" }
    "demo-tenant-b" = { subnet_cidr = "10.20.2.0/24" }
  }
}

variable "manage_projects" {
  description = "If true, create the project/user/VM for each tenant (demo). If false, tenants already exist and you pass project_id/user_id in var.tenants."
  type        = bool
  default     = true
}

variable "tenant_roles" {
  description = "Keystone roles to grant each tenant user (must exist). See modules/t4o-tenant-grants."
  type        = list(string)
  default     = ["backup_admin", "_member_", "creator"]
}

# ── Demo project scaffold knobs (only used when manage_projects = true) ───────
variable "external_network_name" {
  type    = string
  default = "public"
}

variable "image_name" {
  type    = string
  default = "cirros"
}

variable "flavor_name" {
  type    = string
  default = "m1.tiny"
}

variable "member_role_name" {
  type    = string
  default = "member"
}

# ── Shared backup target ─────────────────────────────────────────────────────
variable "backup_target_name" {
  type    = string
  default = "shared-nfs"
}

variable "backup_target_type" {
  description = "Backup target type accepted by the T4O 6.2 WLM API: nfs | s3 (MinIO or AWS)."
  type        = string
  default     = "nfs"
  validation {
    condition     = contains(["nfs", "s3"], var.backup_target_type)
    error_message = "backup_target_type must be nfs or s3."
  }
}

variable "nfs_export" {
  description = "NFS export for the shared backup target, e.g. 10.0.0.5:/exports/tvault. Required when backup_target_type=nfs."
  type        = string
  default     = ""
}

variable "s3_endpoint_url" {
  description = "S3 endpoint URL (e.g. http://minio.example.com:9000 for MinIO; https://s3.amazonaws.com for AWS). Required when type=s3."
  type        = string
  default     = ""
}

variable "s3_bucket" {
  description = "S3 bucket name (must already exist). Required when type=s3."
  type        = string
  default     = ""
}

variable "s3_region" {
  description = "S3 region name."
  type        = string
  default     = "us-east-1"
}

variable "s3_access_key" {
  description = "S3 access key id. Stored in Barbican; required when type=s3."
  type        = string
  default     = ""
  sensitive   = true
}

variable "s3_secret_key" {
  description = "S3 secret access key. Stored in Barbican; required when type=s3."
  type        = string
  default     = ""
  sensitive   = true
}

# ── Shared policy ────────────────────────────────────────────────────────────
variable "policy_name" {
  type    = string
  default = "shared-daily"
}

variable "schedule_start_date" {
  type    = string
  default = "2026-07-01 02:00:00"
}

variable "schedule_interval_hours" {
  description = "Hourly interval. WLM accepts only 1, 2, 3, 4, 6, 8, 12, 24."
  type        = number
  default     = 24
  validation {
    condition     = contains([1, 2, 3, 4, 6, 8, 12, 24], var.schedule_interval_hours)
    error_message = "schedule_interval_hours must be one of 1, 2, 3, 4, 6, 8, 12, 24."
  }
}

variable "retention_days" {
  type    = number
  default = 30
}

variable "snapshots_to_retain" {
  type    = number
  default = 30
}

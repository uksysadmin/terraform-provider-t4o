variable "workload_name" {
  description = "Name for the T4O workload."
  type        = string
}

variable "instance_ids" {
  description = "List of Nova instance UUIDs to protect."
  type        = list(string)
}

# Backup target
variable "create_backup_target" {
  description = "Set true for the first project. Set false and pass backup_target_id for subsequent projects."
  type        = bool
  default     = true
}

variable "backup_target_id" {
  description = "Existing backup target ID (when create_backup_target = false)."
  type        = string
  default     = ""
}

variable "backup_target_name" {
  description = "Name for the backup target (used when create_backup_target = true)."
  type        = string
  default     = "demo-nfs"
}

variable "nfs_export" {
  description = "NFS export path, e.g. 10.0.0.5:/exports/tvault"
  type        = string
  default     = ""
}

variable "backup_target_is_default" {
  type    = bool
  default = false
}

# Policy
variable "create_policy" {
  description = "Set true for the first project. Set false and pass policy_id for subsequent projects."
  type        = bool
  default     = true
}

variable "policy_id" {
  description = "Existing policy ID (when create_policy = false)."
  type        = string
  default     = ""
}

variable "policy_name" {
  description = "Name for the workload policy (used when create_policy = true)."
  type        = string
  default     = "standard-daily"
}

# Schedule
variable "schedule_start_date" {
  description = "Schedule start date, format: YYYY-MM-DD HH:MM:SS"
  type        = string
  default     = "2026-07-01 02:00:00"
}

variable "schedule_interval_hours" {
  description = "Hourly backup interval. WLM 6.2 only accepts one of: 1, 2, 3, 4, 6, 8, 12, 24 (24 = daily)."
  type        = number
  default     = 24
  validation {
    condition     = contains([1, 2, 3, 4, 6, 8, 12, 24], var.schedule_interval_hours)
    error_message = "schedule_interval_hours must be one of 1, 2, 3, 4, 6, 8, 12, 24 (WLM advanced-scheduler hourly whitelist)."
  }
}

variable "fullbackup_interval" {
  description = "Run a full backup every N incremental backups."
  type        = number
  default     = 7
}

variable "retention_days" {
  description = "Number of days to retain snapshots."
  type        = number
  default     = 30
}

variable "snapshots_to_retain" {
  description = "Maximum number of snapshots to keep."
  type        = number
  default     = 30
}

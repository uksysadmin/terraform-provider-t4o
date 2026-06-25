variable "workload_name" {
  description = "Name for the T4O workload."
  type        = string
}

variable "description" {
  description = "Workload description."
  type        = string
  default     = "Managed by Terraform (T4O tenant module)."
}

variable "instance_ids" {
  description = "Nova instance UUIDs (in this tenant's project) to protect."
  type        = list(string)
}

variable "policy_id" {
  description = "ID of a shared workload policy already assigned to this tenant's project (from the admin stack output). Leave \"\" to use an inline jobschedule instead."
  type        = string
  default     = ""
}

variable "jobschedule" {
  description = "Inline schedule for the workload (mutually exclusive with policy_id). Leave null to use a shared policy."
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

variable "backup_target_name" {
  description = "Name of the shared backup target to use (looked up at apply time). Ignored if backup_target_id is set."
  type        = string
  default     = ""
}

variable "backup_target_id" {
  description = "Explicit shared backup target ID. Takes precedence over backup_target_name."
  type        = string
  default     = ""
}

variable "workload_type_id" {
  description = "Workload type ID. Defaults to the first type the API returns (e.g. Serialized/Parallel)."
  type        = string
  default     = ""
}

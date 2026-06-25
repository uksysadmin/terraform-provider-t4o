variable "auth_url" {
  description = "OpenStack Keystone endpoint."
  type        = string
  default     = "http://10.0.0.10:5000"
}

variable "username" {
  description = "OpenStack username."
  type        = string
  default     = "admin"
}

variable "password" {
  description = "OpenStack password."
  type        = string
  sensitive   = true
}

variable "project_id" {
  description = "Target OpenStack project UUID (tfprov-test)."
  type        = string
}

variable "domain_name" {
  description = "OpenStack domain."
  type        = string
  default     = "Default"
}

variable "nfs_export" {
  description = "NFS export path for the test backup target, e.g. 10.0.0.5:/exports/tvault"
  type        = string
  default     = "10.0.0.5:/exports/tvault2"
}

variable "instance_id" {
  description = "Nova instance UUID to protect. Leave empty to skip workload resource."
  type        = string
  default     = ""
}

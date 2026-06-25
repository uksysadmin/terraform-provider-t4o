variable "project_name" {
  description = "Name prefix for all resources in this project."
  type        = string
}

variable "user_password" {
  description = "Password for the per-project demo user."
  type        = string
  sensitive   = true
  default     = "Demo1234!"
}

variable "subnet_cidr" {
  description = "CIDR for the project subnet."
  type        = string
  default     = "10.10.0.0/24"
}

variable "external_network_name" {
  description = "Name of the external/provider network for router uplink (usually 'public' or 'external')."
  type        = string
  default     = "public"
}

variable "image_name" {
  description = "Glance image name to boot VMs from."
  type        = string
  default     = "cirros"
}

variable "flavor_name" {
  description = "Nova flavor name for the demo VM (e.g. m1.tiny, m1.small). Must exist in the cloud."
  type        = string
  default     = "m1.tiny"
}

variable "member_role_name" {
  description = "Keystone role granted to the per-project user. Usually 'member' (older clouds: '_member_')."
  type        = string
  default     = "member"
}

variable "create_vm" {
  description = "Create the demo VM (+ data volume) in THIS project. Set false for the admin flow, where the VM lands in the wrong (admin) project — the tenant stack creates it instead."
  type        = bool
  default     = true
}

variable "data_volume_size_gb" {
  description = "Size in GB of the Cinder data volume attached to the VM."
  type        = number
  default     = 1
}

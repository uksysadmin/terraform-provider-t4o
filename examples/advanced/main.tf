# Advanced features: encrypted workloads + per-project backup quota.
# Reference snippet — fill in real IDs/creds (or wire to the admin/tenant stacks).

terraform {
  required_providers {
    t4o = {
      source  = "trilio-demo/t4o"
      version = "~> 0.1"
    }
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "~> 2.0"
    }
  }
}

provider "t4o" {
  auth_url    = "http://<keystone-host>:5000"
  username    = "admin"
  password    = var.os_password
  project_id  = var.project_id
  domain_name = "Default"
}

provider "openstack" {
  auth_url    = "http://<keystone-host>:5000/v3"
  user_name   = "admin"
  password    = var.os_password
  tenant_id   = var.project_id
  domain_name = "Default"
}

variable "os_password" {
  type      = string
  sensitive = true
}
variable "project_id" { type = string }

# ── Encrypted workload ───────────────────────────────────────────────────────
# The encryption passphrase lives in a Barbican secret; the workload references it.
resource "openstack_keymanager_secret_v1" "enc" {
  name                 = "wl-enc-passphrase"
  secret_type          = "passphrase"
  payload_content_type = "text/plain"
  payload              = var.os_password # use a dedicated strong passphrase in practice
}

data "t4o_workload_types" "all" {}

resource "t4o_workload" "encrypted" {
  name             = "encrypted-web-tier"
  workload_type_id = data.t4o_workload_types.all.workload_types[0].id
  instance_ids     = ["<vm-uuid-1>"]
  encryption       = true
  secret_uuid      = openstack_keymanager_secret_v1.enc.id

  jobschedule = {
    enabled             = true
    interval            = "24"
    retention_days      = 30
    snapshots_to_retain = 30
  }
}

# ── Per-project backup quota ─────────────────────────────────────────────────
# Discover the quota_type_id from the data source, then cap the project.
data "t4o_quota_types" "all" {}

resource "t4o_project_quota" "workloads_cap" {
  project_id    = var.project_id
  quota_type_id = data.t4o_quota_types.all.quota_types[0].id # pick the type you want to cap
  allowed_value = 50
}

output "quota_type_options" {
  description = "Available quota types (id + name) to choose quota_type_id from."
  value       = data.t4o_quota_types.all.quota_types
}

# ── Project setting (end-to-end config) ──────────────────────────────────────
# A per-project WLM setting — here, an email-notification address.
variable "notify_user_id" { type = string }

resource "t4o_setting" "notify_email" {
  name  = "user_email_address_${var.notify_user_id}"
  value = "ops@example.com"
  type  = "email"
}

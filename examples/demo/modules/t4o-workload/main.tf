# Creates a T4O backup target, workload policy, and workload for a given project+VM.
# Reusable across projects — just call this module once per project.

terraform {
  required_providers {
    t4o = {
      source  = "registry.terraform.io/trilio-demo/t4o"
      version = "0.1.0"
    }
  }
}

# ── Shared backup target (created once, reused across workloads) ─────────────
# Only created when create_backup_target = true (first project call).
# Second project passes backup_target_id directly.

resource "t4o_backup_target" "nfs" {
  count             = var.create_backup_target ? 1 : 0
  name              = var.backup_target_name
  type              = "nfs"
  filesystem_export = var.nfs_export
  is_default        = var.backup_target_is_default
}

locals {
  backup_target_id = var.create_backup_target ? t4o_backup_target.nfs[0].id : var.backup_target_id
}

# ── Workload policy ──────────────────────────────────────────────────────────

resource "t4o_workload_policy" "policy" {
  count       = var.create_policy ? 1 : 0
  name        = var.policy_name
  description = "Daily backup — ${var.retention_days}d retention. Managed by Terraform."

  jobschedule = {
    enabled             = true
    start_date          = var.schedule_start_date
    end_date            = "2030-12-31 00:00:00"
    interval            = tostring(var.schedule_interval_hours)
    fullbackup_interval = var.fullbackup_interval
    retention_days      = var.retention_days
    snapshots_to_retain = var.snapshots_to_retain
  }
}

locals {
  policy_id = var.create_policy ? t4o_workload_policy.policy[0].id : var.policy_id
}

# ── Workload ─────────────────────────────────────────────────────────────────

data "t4o_workload_types" "all" {}

resource "t4o_workload" "workload" {
  name             = var.workload_name
  description      = "Protects ${join(", ", var.instance_ids)}"
  workload_type_id = data.t4o_workload_types.all.workload_types[0].id
  instance_ids     = var.instance_ids
  backup_target_id = local.backup_target_id
  policy_id        = local.policy_id
}

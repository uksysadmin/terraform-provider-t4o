# t4o-tenant-backup — TENANT layer (runs under project-scoped tenant credentials)
#
# Creates T4O workload(s) protecting the tenant's OWN instances, using:
#   * a SHARED backup target the admin already created (looked up by name, or
#     passed by id), and
#   * a SHARED workload policy the admin already created AND assigned to this
#     tenant's project (passed by id — there is no policy data source yet).
#
# The tenant does NOT create backup targets or policies — under T4O's custom RBAC
# those are admin-only. The calling root module configures the t4o provider with
# the tenant's own credentials; this reusable module declares no provider block.

data "t4o_workload_types" "all" {}

data "t4o_backup_targets" "all" {}

locals {
  resolved_backup_target_id = (
    var.backup_target_id != "" ? var.backup_target_id :
    one([for bt in data.t4o_backup_targets.all.backup_targets : bt.id if bt.name == var.backup_target_name])
  )

  resolved_workload_type_id = (
    var.workload_type_id != "" ? var.workload_type_id :
    data.t4o_workload_types.all.workload_types[0].id
  )
}

resource "t4o_workload" "this" {
  name             = var.workload_name
  description      = var.description
  workload_type_id = local.resolved_workload_type_id
  instance_ids     = var.instance_ids
  backup_target_id = local.resolved_backup_target_id

  # A workload is driven by EITHER a shared policy (policy_id) OR its own inline
  # jobschedule — mutually exclusive. Pass exactly one; the other stays unset/null.
  policy_id   = var.policy_id != "" ? var.policy_id : null
  jobschedule = var.jobschedule

  lifecycle {
    precondition {
      condition     = local.resolved_backup_target_id != null
      error_message = "No backup target found. Set backup_target_id, or ensure a target named '${var.backup_target_name}' is visible to this tenant."
    }
    precondition {
      condition     = (var.policy_id != "" && var.jobschedule == null) || (var.policy_id == "" && var.jobschedule != null)
      error_message = "Provide exactly ONE of: policy_id (shared policy, must be assigned to this project) OR jobschedule (inline schedule). Not both, not neither."
    }
  }
}

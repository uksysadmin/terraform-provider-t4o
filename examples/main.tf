terraform {
  required_providers {
    t4o = {
      source  = "registry.terraform.io/trilio-demo/t4o"
      version = "~> 0.1"
    }
  }
}

provider "t4o" {
  auth_url    = var.auth_url
  username    = var.username
  password    = var.password
  project_id  = var.project_id
  domain_name = var.domain_name
}

# -----------------------------------------------------------------------
# Data sources
# -----------------------------------------------------------------------

data "t4o_workload_types" "all" {}

output "workload_types" {
  value = data.t4o_workload_types.all.workload_types
}

data "t4o_backup_targets" "existing" {}

output "existing_backup_targets" {
  value = data.t4o_backup_targets.existing.backup_targets
}

# -----------------------------------------------------------------------
# Backup target (NFS — secondary export, is_default=false so it can be destroyed)
# /exports/tvault is the standing permanent default — do not manage it via Terraform.
# -----------------------------------------------------------------------

resource "t4o_backup_target" "nfs" {
  name              = "tfprov-test-nfs"
  type              = "nfs"
  filesystem_export = var.nfs_export
  is_default        = false
}

output "backup_target_id" {
  value = t4o_backup_target.nfs.id
}

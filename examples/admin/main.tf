# ============================================================================
# ADMIN stack — run ONCE by a cloud administrator.
#
# Provisions the shared, admin-owned T4O scaffolding that tenants consume:
#   1. (demo only) a project + user + VM per tenant
#   2. the Keystone role grants each tenant user needs (T4O RBAC + trustee roles)
#   3. a shared backup target
#   4. a shared workload policy, assigned to every tenant project
#
# Outputs feed the per-tenant stacks in ../tenant (see ../README.md).
# In a real cloud, tenants/users already exist — set manage_projects = false and
# pass existing project/user IDs via var.tenants instead.
# ============================================================================

provider "openstack" {
  auth_url    = var.auth_url
  user_name   = var.admin_username
  password    = var.admin_password
  tenant_name = var.admin_project_name
  domain_name = var.domain_name
}

provider "t4o" {
  auth_url    = var.auth_url
  username    = var.admin_username
  password    = var.admin_password
  project_id  = var.admin_project_id
  domain_name = var.domain_name
}

# ── Per-tenant OpenStack scaffold (demo convenience) ─────────────────────────
# Reuses the existing project module. Skip by setting manage_projects = false and
# supplying project_id/user_id in var.tenants.
module "tenant_project" {
  source   = "../demo/modules/openstack-project"
  for_each = var.manage_projects ? var.tenants : {}

  project_name          = each.key
  subnet_cidr           = each.value.subnet_cidr
  external_network_name = var.external_network_name
  image_name            = var.image_name
  flavor_name           = var.flavor_name
  member_role_name      = var.member_role_name

  # The VM must be created by the TENANT stack (scoped to the tenant project).
  # An admin-scoped VM here would land in the admin project and be invisible to WLM.
  create_vm = false
}

locals {
  # Normalize tenant identity whether projects are TF-managed or pre-existing.
  tenant_project_ids = {
    for name, t in var.tenants : name => (
      var.manage_projects ? try(module.tenant_project[name].project_id, null) : t.project_id
    )
  }
  tenant_user_ids = {
    for name, t in var.tenants : name => (
      var.manage_projects ? try(module.tenant_project[name].user_id, null) : t.user_id
    )
  }
}

# ── Role grants: give each tenant user the roles T4O requires ────────────────
module "tenant_grants" {
  source   = "../modules/t4o-tenant-grants"
  for_each = var.tenants

  user_id    = local.tenant_user_ids[each.key]
  project_id = local.tenant_project_ids[each.key]
  roles      = var.tenant_roles
}

# ── Shared backup target (admin-owned) ────────────────────────────────────
# Supports NFS (default) and S3 (MinIO / AWS). For S3, the T4O 6.2 WLM API
# wants type="s3" plus a Barbican secret_ref holding the credentials — so when
# backup_target_type="s3" we first store the creds in Barbican via
# openstack_keymanager_secret_v1, then reference it from the backup target.
locals {
  is_s3 = var.backup_target_type == "s3"
  # WLM/DMS export string: "<endpoint-netloc>/<bucket>" (no scheme).
  s3_netloc = replace(replace(var.s3_endpoint_url, "https://", ""), "http://", "")
  s3_secret_payload = jsonencode({
    VAULT_S3_ACCESS_KEY_ID     = var.s3_access_key
    VAULT_S3_SECRET_ACCESS_KEY = var.s3_secret_key
    VAULT_S3_BUCKET            = var.s3_bucket
    VAULT_STORAGE_S3_EXPORT    = "${local.s3_netloc}/${var.s3_bucket}"
    VAULT_S3_ENDPOINT_URL      = var.s3_endpoint_url
    VAULT_S3_REGION_NAME       = var.s3_region
    VAULT_S3_SSL               = startswith(var.s3_endpoint_url, "https://") ? "True" : "False"
    VAULT_S3_SSL_VERIFY        = startswith(var.s3_endpoint_url, "https://") ? "True" : "False"
  })
}

# Barbican secret with the S3 credentials (only when type="s3").
resource "openstack_keymanager_secret_v1" "s3_creds" {
  count       = local.is_s3 ? 1 : 0
  name        = "${var.backup_target_name}-s3-creds"
  secret_type = "opaque"
  payload     = local.s3_secret_payload
  # text/plain (not application/octet-stream): the payload is a JSON string, and
  # octet-stream secrets do not round-trip in terraform-provider-openstack — the
  # payload reads back re-encoded, forcing a perpetual secret replacement that
  # cascades a new secret_ref and replaces the backup target on every apply.
  payload_content_type = "text/plain"
}

resource "t4o_backup_target" "shared" {
  name = var.backup_target_name
  type = var.backup_target_type

  # NFS
  filesystem_export = local.is_s3 ? null : var.nfs_export

  # S3
  s3_endpoint_url = local.is_s3 ? var.s3_endpoint_url : null
  s3_bucket       = local.is_s3 ? var.s3_bucket : null
  secret_ref      = local.is_s3 ? openstack_keymanager_secret_v1.s3_creds[0].secret_ref : null

  is_default = false

  lifecycle {
    precondition {
      condition     = local.is_s3 ? (var.s3_endpoint_url != "" && var.s3_bucket != "" && var.s3_access_key != "" && var.s3_secret_key != "") : (var.nfs_export != "")
      error_message = "type=nfs needs nfs_export; type=s3 needs s3_endpoint_url, s3_bucket, s3_access_key, s3_secret_key."
    }
  }
}

# ── Shared policy, assigned to every tenant project so tenants can use it ─────
resource "t4o_workload_policy" "shared" {
  name        = var.policy_name
  description = "Shared daily policy — ${var.retention_days}d retention. Managed by admin."

  assigned_projects = values(local.tenant_project_ids)

  jobschedule = {
    enabled             = true
    start_date          = var.schedule_start_date
    end_date            = "2030-12-31 00:00:00"
    interval            = tostring(var.schedule_interval_hours)
    fullbackup_interval = 7
    retention_days      = var.retention_days
    snapshots_to_retain = var.snapshots_to_retain
  }

  # Assignment targets the tenant projects, so the grants should land first.
  depends_on = [module.tenant_grants]
}

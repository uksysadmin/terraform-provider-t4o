terraform {
  required_providers {
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "~> 2.0"
    }
    t4o = {
      source  = "trilio-demo/t4o"
      version = "~> 0.1"
    }
  }
}

# ── Provider config ──────────────────────────────────────────────────────────

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

# ── Trust prerequisite (opt-in) ──────────────────────────────────────────────
# T4O builds a Keystone trust on workload create and expects the auth user to hold
# the roles in WLM's `trustee_role` config (stock default: "_member_, creator").
# `creator` exists where Barbican is deployed; `_member_` is legacy. Make sure the
# auth user holds these roles before creating workloads.
# Set manage_trustee_roles=true to have the module grant the auth (admin) user those
# roles on its own project. Requires admin and the roles to already exist in Keystone —
# if a role is absent, create it first or trim WLM's trustee_role to match the cloud.

data "openstack_identity_auth_scope_v3" "current" {
  name = "terraform-trust-scope"
}

data "openstack_identity_role_v3" "trustee" {
  for_each = var.manage_trustee_roles ? toset(var.trustee_roles) : toset([])
  name     = each.value
}

resource "openstack_identity_role_assignment_v3" "trustee" {
  for_each   = var.manage_trustee_roles ? toset(var.trustee_roles) : toset([])
  user_id    = data.openstack_identity_auth_scope_v3.current.user_id
  project_id = var.admin_project_id
  role_id    = data.openstack_identity_role_v3.trustee[each.key].id
}

# ── Project A ────────────────────────────────────────────────────────────────

module "project_a" {
  source = "./modules/openstack-project"

  project_name          = "demo-project-a"
  subnet_cidr           = "10.10.1.0/24"
  external_network_name = var.external_network_name
  image_name            = var.image_name
  flavor_name           = var.flavor_name
  member_role_name      = var.member_role_name
}

module "backup_a" {
  source = "./modules/t4o-workload"

  workload_name = "workload-project-a"
  instance_ids  = [module.project_a.vm_id]

  create_backup_target     = true
  backup_target_name       = "demo-nfs"
  nfs_export               = var.nfs_export
  backup_target_is_default = false

  create_policy           = true
  policy_name             = "standard-daily"
  schedule_interval_hours = 24
  retention_days          = 30
  snapshots_to_retain     = 30

  # Ensure the trust roles are granted before the workload create triggers the trust.
  depends_on = [openstack_identity_role_assignment_v3.trustee]
}

# ── Project B ────────────────────────────────────────────────────────────────

module "project_b" {
  source = "./modules/openstack-project"

  project_name          = "demo-project-b"
  subnet_cidr           = "10.10.2.0/24"
  external_network_name = var.external_network_name
  image_name            = var.image_name
  flavor_name           = var.flavor_name
  member_role_name      = var.member_role_name
}

module "backup_b" {
  source = "./modules/t4o-workload"

  workload_name = "workload-project-b"
  instance_ids  = [module.project_b.vm_id]

  create_backup_target = false
  backup_target_id     = module.backup_a.backup_target_id

  create_policy = false
  policy_id     = module.backup_a.policy_id

  depends_on = [openstack_identity_role_assignment_v3.trustee]
}

# ── Outputs ──────────────────────────────────────────────────────────────────

output "project_a" {
  value = {
    project_id  = module.project_a.project_id
    vm_id       = module.project_a.vm_id
    workload_id = module.backup_a.workload_id
  }
}

output "project_b" {
  value = {
    project_id  = module.project_b.project_id
    vm_id       = module.project_b.vm_id
    workload_id = module.backup_b.workload_id
  }
}

output "shared" {
  value = {
    backup_target_id = module.backup_a.backup_target_id
    policy_id        = module.backup_a.policy_id
  }
}

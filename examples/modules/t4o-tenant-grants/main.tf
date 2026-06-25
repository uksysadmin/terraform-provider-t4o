# t4o-tenant-grants — ADMIN layer (requires cloud-admin credentials)
#
# Grants a single tenant user the Keystone roles required to use T4O inside their
# own project:
#
#   * the T4O RBAC role (default "backup_admin") — WLM's custom policy authorizes
#     workload create/backup/restore against this role (plain "member" is denied).
#   * the Keystone trustee roles (default "_member_", "creator") — WLM builds a
#     per-workload trust and validates the trustor holds EVERY role in its
#     `trustee_role` config. "creator" only exists where Barbican is deployed;
#     "_member_" is legacy (secure-RBAC clouds use "member").
#
# All listed roles MUST already exist in Keystone. Create the custom T4O roles once:
#     openstack role create backup_admin
#     openstack role create restore_only
# If a role in `roles` does not exist, the data lookup below fails with a clear
# error — create it, or trim `roles`/WLM's trustee_role to match the cloud.
#
# This module declares no provider block (HashiCorp best practice): the calling
# (admin) root module owns the cloud-admin openstack provider and passes it in.

data "openstack_identity_role_v3" "role" {
  for_each = toset(var.roles)
  name     = each.value
}

resource "openstack_identity_role_assignment_v3" "grant" {
  for_each   = toset(var.roles)
  user_id    = var.user_id
  project_id = var.project_id
  role_id    = data.openstack_identity_role_v3.role[each.key].id
}

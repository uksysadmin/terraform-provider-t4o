# ============================================================================
# TENANT stack — run by (or for) ONE tenant, with that tenant's OWN project-scoped
# credentials. Both providers below authenticate as the tenant and are scoped to the
# tenant project, so the VM is created IN the tenant project (where WLM can see it)
# and the workload that protects it is created in the same scope.
#
# Prerequisites (done once by ../admin):
#   * the tenant project + user + network exist; the user holds the trust role(s)
#   * a shared backup target exists; a shared policy exists AND is assigned to this project
# You pass in the network_id, policy_id, and backup_target_id from the admin outputs.
# ============================================================================

provider "openstack" {
  auth_url    = var.auth_url
  user_name   = var.tenant_username
  password    = var.tenant_password
  tenant_id   = var.tenant_project_id
  domain_name = var.domain_name
}

provider "t4o" {
  auth_url    = var.auth_url
  username    = var.tenant_username
  password    = var.tenant_password
  project_id  = var.tenant_project_id
  domain_name = var.domain_name
}

# ── The tenant's own VM (created in the tenant project) ──────────────────────
data "openstack_images_image_v2" "img" {
  name        = var.image_name
  most_recent = true
}

data "openstack_compute_flavor_v2" "flv" {
  name = var.flavor_name
}

resource "openstack_networking_port_v2" "port" {
  count      = var.vm_count
  name       = "${var.workload_name}-port-${count.index}"
  network_id = var.network_id
}

resource "openstack_compute_instance_v2" "vm" {
  count     = var.vm_count
  name      = "${var.workload_name}-vm-${count.index}"
  image_id  = data.openstack_images_image_v2.img.id
  flavor_id = data.openstack_compute_flavor_v2.flv.id

  network {
    port = openstack_networking_port_v2.port[count.index].id
  }
}

# ── Protect all of them with one workload (tests the multi-instance path) ────
module "backup" {
  source = "../modules/t4o-tenant-backup"

  workload_name    = var.workload_name
  instance_ids     = openstack_compute_instance_v2.vm[*].id
  backup_target_id = var.backup_target_id
  policy_id        = var.policy_id
  jobschedule      = var.jobschedule
}

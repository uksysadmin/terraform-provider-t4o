# Creates an isolated OpenStack project with its own user, network, router, Cirros VM,
# and an attached Cinder data volume.
# Works on Kolla, RHOSO, and Canonical OpenStack — standard Nova/Neutron/Keystone/Cinder APIs.

terraform {
  required_providers {
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "~> 2.0"
    }
  }
}

# ── Auto-discover cloud resources ────────────────────────────────────────────

data "openstack_images_image_v2" "cirros" {
  name        = var.image_name
  most_recent = true
}

data "openstack_compute_flavor_v2" "tiny" {
  name = var.flavor_name
}

data "openstack_networking_network_v2" "external" {
  name     = var.external_network_name
  external = true
}

data "openstack_identity_role_v3" "member" {
  name = var.member_role_name
}

# ── Project & user ──────────────────────────────────────────────────────────

resource "openstack_identity_project_v3" "project" {
  name        = var.project_name
  description = "T4O demo project — managed by Terraform"
}

resource "openstack_identity_user_v3" "user" {
  name               = "${var.project_name}-user"
  password           = var.user_password
  default_project_id = openstack_identity_project_v3.project.id
}

resource "openstack_identity_role_assignment_v3" "member" {
  user_id    = openstack_identity_user_v3.user.id
  project_id = openstack_identity_project_v3.project.id
  role_id    = data.openstack_identity_role_v3.member.id
}

# ── Network ─────────────────────────────────────────────────────────────────

resource "openstack_networking_network_v2" "net" {
  name           = "${var.project_name}-net"
  admin_state_up = true
  tenant_id      = openstack_identity_project_v3.project.id
}

resource "openstack_networking_subnet_v2" "subnet" {
  name       = "${var.project_name}-subnet"
  network_id = openstack_networking_network_v2.net.id
  cidr       = var.subnet_cidr
  ip_version = 4
  tenant_id  = openstack_identity_project_v3.project.id
}

resource "openstack_networking_router_v2" "router" {
  name                = "${var.project_name}-router"
  admin_state_up      = true
  external_network_id = data.openstack_networking_network_v2.external.id
  tenant_id           = openstack_identity_project_v3.project.id
}

resource "openstack_networking_router_interface_v2" "iface" {
  router_id = openstack_networking_router_v2.router.id
  subnet_id = openstack_networking_subnet_v2.subnet.id
}

# ── Security group ───────────────────────────────────────────────────────────

resource "openstack_networking_secgroup_v2" "sg" {
  name        = "${var.project_name}-sg"
  description = "Allow SSH and ICMP"
  tenant_id   = openstack_identity_project_v3.project.id
}

resource "openstack_networking_secgroup_rule_v2" "ssh" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 22
  port_range_max    = 22
  security_group_id = openstack_networking_secgroup_v2.sg.id
}

resource "openstack_networking_secgroup_rule_v2" "icmp" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "icmp"
  security_group_id = openstack_networking_secgroup_v2.sg.id
}

# ── VM ───────────────────────────────────────────────────────────────────────

# NOTE: a VM created here lands in the *authenticating* token's project — there is no
# per-resource project override in Nova. So for multi-tenant use the admin sets
# create_vm = false and the tenant stack (scoped to its own project) creates the VM.
# The all-in-one demo keeps create_vm = true (admin creates everything in one project).

resource "openstack_networking_port_v2" "port" {
  count              = var.create_vm ? 1 : 0
  name               = "${var.project_name}-port"
  network_id         = openstack_networking_network_v2.net.id
  security_group_ids = [openstack_networking_secgroup_v2.sg.id]
  depends_on         = [openstack_networking_subnet_v2.subnet]
}

resource "openstack_compute_instance_v2" "vm" {
  count     = var.create_vm ? 1 : 0
  name      = "${var.project_name}-vm"
  image_id  = data.openstack_images_image_v2.cirros.id
  flavor_id = data.openstack_compute_flavor_v2.tiny.id

  network {
    port = openstack_networking_port_v2.port[0].id
  }
}

# ── Data volume (Cinder) ─────────────────────────────────────────────────────

resource "openstack_blockstorage_volume_v3" "data" {
  count       = var.create_vm ? 1 : 0
  name        = "${var.project_name}-data"
  size        = var.data_volume_size_gb
  description = "Demo data volume — managed by Terraform"
}

resource "openstack_compute_volume_attach_v2" "attach" {
  count       = var.create_vm ? 1 : 0
  instance_id = openstack_compute_instance_v2.vm[0].id
  volume_id   = openstack_blockstorage_volume_v3.data[0].id
}

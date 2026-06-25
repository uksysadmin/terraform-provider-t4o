output "project_id" {
  value = openstack_identity_project_v3.project.id
}

output "user_id" {
  value = openstack_identity_user_v3.user.id
}

output "vm_id" {
  value = try(openstack_compute_instance_v2.vm[0].id, null)
}

output "vm_name" {
  value = try(openstack_compute_instance_v2.vm[0].name, null)
}

output "network_id" {
  value = openstack_networking_network_v2.net.id
}

output "subnet_id" {
  value = openstack_networking_subnet_v2.subnet.id
}

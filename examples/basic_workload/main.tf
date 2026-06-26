terraform {
  required_providers {
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "~> 2.0"
    }
    t4o = {
      source  = "registry.terraform.io/trilio-demo/t4o"
      version = "0.1.0"
    }
  }
}

# The providers will automatically pick up standard OpenStack OS_ environment variables.
# You can also specify them explicitly using the variables defined in terraform.tfvars.
provider "openstack" {}

provider "t4o" {}

variable "image_id" {
  description = "The UUID of the image to use for the VM"
  type        = string
}

variable "flavor_id" {
  description = "The UUID of the flavor to use for the VM"
  type        = string
}

variable "network_id" {
  description = "The UUID of the network to attach the VM to"
  type        = string
}

variable "backup_target_id" {
  description = "The UUID of an existing backup target (optional, defaults to project default)"
  type        = string
  default     = ""
}

# 1. Create a VM named "example"
resource "openstack_compute_instance_v2" "example_vm" {
  name            = "example"
  image_id        = var.image_id
  flavor_id       = var.flavor_id
  
  network {
    uuid = var.network_id
  }
}

# 2. Retrieve workload types from T4O
data "t4o_workload_types" "all" {}

# 3. Create a workload named "example-workload" protecting the VM
resource "t4o_workload" "example_workload" {
  name             = "example-workload"
  description      = "Basic workload containing the example VM"
  workload_type_id = data.t4o_workload_types.all.workload_types[0].id
  
  instance_ids     = [
    openstack_compute_instance_v2.example_vm.id
  ]
  
  # If a specific backup target is provided, bind the workload to it
  backup_target_id = var.backup_target_id != "" ? var.backup_target_id : null
}

output "vm_id" {
  value = openstack_compute_instance_v2.example_vm.id
}

output "workload_id" {
  value = t4o_workload.example_workload.id
}

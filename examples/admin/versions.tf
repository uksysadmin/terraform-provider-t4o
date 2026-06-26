terraform {
  required_version = ">= 1.3"
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

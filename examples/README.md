# T4O Terraform examples

This directory provides working examples of how to consume the TrilioVault for OpenStack (T4O) provider.

## Included Examples

- [`demo/`](./demo) - A comprehensive example that creates projects, VMs, a shared backup target, a backup policy, and assigns workloads. This is the quickest way to see the provider work end-to-end.

## Roles and Permissions (RBAC)

T4O relies on standard OpenStack Role-Based Access Control (RBAC). The provider does not enforce "admin" or "tenant" limitations on the client side; instead, it delegates this entirely to the WLM API. 

When you apply a Terraform configuration, the provider uses the credentials you supply (via `OS_` environment variables or directly in the provider config). If your user does not have permission to perform an action—for example, if a standard tenant user attempts to create a shared backup policy—the WLM API will return a `403 Forbidden` error, and Terraform will fail quickly and cleanly.

This design means you don't need to arbitrarily split your Terraform state into "admin stacks" and "tenant stacks" unless it fits your organizational model. Simply declare your desired configuration, run Terraform, and the system handles the permissions seamlessly.

## Two hard prerequisites T4O imposes

1. **RBAC role.** Under T4O's custom policy, workload management is typically granted via the `backup_admin` role on the project (`restore_only` for read+restore). Create the custom roles once: `openstack role create backup_admin restore_only`, then assign them to users as needed.

2. **Keystone trust roles.** On workload create, T4O builds a Keystone trust and expects the user to hold the roles in WLM's `trustee_role` config (stock default `_member_, creator`). `creator` exists where Barbican is deployed; secure-RBAC clouds use `member` rather than `_member_`.

Both should be in place before workload create so the RBAC check and trust build succeed.
## Run order

Credentials come from your environment in every stack — `source` the right openrc, then run.
Nothing below puts a password in a file.

```bash
# 1. Admin (cloud-admin), once:
source ~/admin-openrc.sh                        # OS_* for the cloud admin
cd admin
cp terraform.tfvars.example terraform.tfvars    # only nfs_export + tenant map (no creds)
terraform init && terraform apply
terraform output            # note policy_id, backup_target_name, tenants[*]

# 2. Each tenant, with THAT tenant's own (non-admin) creds:
source ~/tenant-a-openrc.sh                      # OS_* for this tenant's user
cd ../tenant
cp terraform.tfvars.example terraform.tfvars     # only the admin outputs (network_id, policy_id, …)
terraform init && terraform apply
```

## Reusable modules (`modules/`)

- [`t4o-tenant-grants`](./modules/t4o-tenant-grants) — admin-side; grants a tenant user the T4O + trustee roles on their project. Provider-less; the root passes the admin `openstack` provider.
- [`t4o-tenant-backup`](./modules/t4o-tenant-backup) — tenant-side; creates workload(s) against a shared target (looked up by name) + a shared policy id. Provider-less; the root passes the tenant `t4o` provider.

The `demo/` example keeps its own bundled `modules/` for the single-apply quickstart.

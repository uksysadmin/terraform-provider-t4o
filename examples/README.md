# T4O Terraform examples

Three ways to consume the provider, from simplest to most production-shaped.

| Example | Who runs it | Creates | Auth |
|---|---|---|---|
| [`demo/`](./demo) | cloud-admin | Everything in one apply: 2 projects + VMs, a backup target, a shared policy, and a workload per project. Quickest way to see the provider work end to end. | admin |
| [`admin/`](./admin) | cloud-admin (once) | The **shared** scaffolding tenants consume: tenant projects/users (demo), the **role grants** each tenant needs, one shared backup target, and one shared policy **assigned** to every tenant project. | admin |
| [`tenant/`](./tenant) | one per tenant | Only that tenant's **workload(s)**, referencing the shared target (by name) and the shared policy (by id). | tenant |

## Why the admin/tenant split

T4O is multi-tenant, but the demo's all-in-one config is an **admin tool** — it creates projects, users and role assignments, which a tenant can't do. A tenant also can't create backup targets or policies (admin-only under T4O's RBAC). So a realistic, reusable shape separates concerns:

- **Admin layer** (cloud-admin): tenants/roles, shared backup target, shared policy + assignment. Run once.
- **Tenant layer** (project-scoped creds): just `t4o_workload`, pointing at the shared resources. One root config per tenant — clean state and credential isolation. This follows HashiCorp's guidance to keep reusable modules provider-less and let each root own its provider configuration.

```
admin/ (admin)  ──outputs──►  tenant/ (tenant A creds)
  shared target                    workload A  ─┐
  shared policy ───assigned to───► tenant/ (tenant B creds)
  role grants                       workload B  ─┘
```

## Two hard prerequisites T4O imposes

1. **RBAC role.** Under T4O's custom policy, workload management is granted via the `backup_admin` role on the project (`restore_only` for read+restore). Create the custom roles once: `openstack role create backup_admin restore_only`, then assign them to each tenant user.

2. **Keystone trust roles.** On workload create, T4O builds a Keystone trust and expects the user to hold the roles in WLM's `trustee_role` config (stock default `_member_, creator`). `creator` exists where Barbican is deployed; secure-RBAC clouds use `member` rather than `_member_`. Match `tenant_roles`/`trustee_role` to your cloud and grant them to the tenant user (the admin stack does this via `modules/t4o-tenant-grants`).

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

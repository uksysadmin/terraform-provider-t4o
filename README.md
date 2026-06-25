# Terraform Provider for TrilioVault for OpenStack (T4O)

Manage TrilioVault for OpenStack resources — backup targets, workloads, and policies — using Terraform.

Supports T4O v6.2 on Kolla Ansible (Flamingo / 2025.2 and Epoxy / 2025.1 — the Trilio-supported Kolla releases), RHOSO, and DevStack deployments.

---

## What it can do (validated)

End-to-end tested on Kolla Flamingo 2025.2 (the Trilio-supported release).

**Backup targets**
- Create and manage **NFS** and **S3** targets (S3 via Barbican `secret_ref`), including **immutable / Object-Lock** S3
- Update in place, delete, and `terraform import` existing targets

**Backup policies**
- Create advanced-scheduler policies (schedule + retention)
- Update an unassigned policy in place; assign / reassign to projects
- Import existing policies

**Workloads**
- Create single- or **multi-VM** workloads, policy-driven
- Update (add/remove VMs) in place; import existing workloads; delete

**Governance & config**
- Per-project backup **quotas** (`t4o_project_quota`) and per-project **settings** (`t4o_setting`, e.g. notification email) as code

**Multi-tenant**
- Admin layer (projects, roles, shared target + policy) and tenant layer (each tenant manages only its own workloads)
- Custom RBAC: `backup_admin` authorized, plain `member` denied

**Guarantees**
- Idempotent `plan` (no changes on a second run) and clean `destroy` (no orphans)

> Backup/restore *execution* (snapshot/restore) is a WLM operation, not a Terraform resource — out of the provider's scope.

---

## Quick start

```bash
# 1. Build & install the provider from source (see Install below)
git clone https://github.com/trilio-demo/terraform-provider-t4o.git
cd terraform-provider-t4o && make install

# 2. Try the bundled end-to-end example (cloud-admin): 2 projects + VMs, a backup target,
#    a shared policy, and a workload per project — all in one apply.
cd examples/demo
cp terraform.tfvars.example terraform.tfvars   # fill in your cloud creds + nfs_export
terraform init && terraform apply
```

`examples/` has three shapes: [`demo/`](./examples/demo) (everything in one apply — start here),
[`admin/`](./examples/admin) + [`tenant/`](./examples/tenant) (the realistic multi-tenant split,
run by different credentials). See [`examples/README.md`](./examples/README.md).

---

## Requirements

- [Terraform >= 1.3](https://developer.hashicorp.com/terraform/install)
- [Go 1.22+](https://go.dev/dl/) — verify with `go version`
- Git
- OpenStack cloud with T4O 6.2 installed and the WLM service registered in Keystone

---

## Cloud prerequisites

Have these in place on the OpenStack cloud before you run anything, or you'll hit cryptic `403`/`500`s:

- **T4O 6.2 deployed** with the WLM service registered in Keystone (service type `workloads`).
- **A valid, unexpired T4O license** applied — without it every workload create returns `500 "License not found"`.
- **Keystone trust roles**: the user that creates workloads must hold the roles in WLM's `trustee_role`
  (typically `creator`; some clouds also `member`). Missing → `500 Invalid roles […]` at workload create.
- **RBAC role** for non-admin users: T4O's custom policy needs `backup_admin` to create workloads
  (plain `member` is denied). Skip if your cloud runs the default permissive policy.
- **Barbican** (key-manager) deployed **only if you'll use S3** backup targets — the S3 `secret_ref` lives there.

---

## Install

This is a **source build** (not yet published on the Terraform Registry). Clone and run `make install` —
no manual paths, no environment variables.

```bash
git clone https://github.com/trilio-demo/terraform-provider-t4o.git
cd terraform-provider-t4o
make install
```

This compiles the provider from source on your machine and installs it into `~/.terraform.d/plugins/`. Nothing outside your home directory is touched.

Then in your Terraform config:

```hcl
terraform {
  required_providers {
    t4o = {
      source  = "trilio-demo/t4o"
      version = "~> 0.1"
    }
  }
}
```

---

## Provider Configuration

```hcl
provider "t4o" {
  auth_url    = "http://<keystone-host>:5000"
  username    = "admin"
  password    = var.os_password
  project_id  = "<project-uuid>"
  domain_name = "Default"
}
```

The provider discovers the WLM endpoint automatically from the Keystone service catalog (service type `workloads`, falling back to `workloadmgr`). No hardcoded ports needed.

| Argument | Description |
|---|---|
| `auth_url` | Keystone v3 endpoint |
| `username` | OpenStack username |
| `password` | OpenStack password (use a variable, not plaintext) |
| `project_id` | Project UUID to scope operations to |
| `domain_name` | Identity domain (default: `Default`) |

---

## Resources

### `t4o_backup_target`

Registers a backup storage destination (NFS or S3-compatible).

```hcl
resource "t4o_backup_target" "nfs" {
  name              = "prod-nfs"
  type              = "nfs"
  filesystem_export = "10.0.0.5:/exports/tvault"
  is_default        = true
}

# S3 stores its credentials in an OpenStack Barbican secret; the target only references it
# via `secret_ref`. (Requires Barbican deployed on the cloud.)
resource "openstack_keymanager_secret_v1" "s3_creds" {
  name                 = "prod-s3-creds"
  secret_type          = "opaque"
  payload_content_type = "text/plain"   # NOT octet-stream — that does not round-trip
  payload = jsonencode({
    VAULT_S3_ACCESS_KEY_ID     = var.s3_access_key
    VAULT_S3_SECRET_ACCESS_KEY = var.s3_secret_key
    VAULT_S3_BUCKET            = "tvault-backups"
    VAULT_STORAGE_S3_EXPORT    = "s3.example.com/tvault-backups"   # "<endpoint-host>/<bucket>"
    VAULT_S3_ENDPOINT_URL      = "https://s3.example.com"
    VAULT_S3_REGION_NAME       = "us-east-1"
    VAULT_S3_SSL               = "True"
  })
}

resource "t4o_backup_target" "s3" {
  name            = "prod-s3"
  type            = "s3"
  s3_endpoint_url = "https://s3.example.com"
  s3_bucket       = "tvault-backups"
  secret_ref      = openstack_keymanager_secret_v1.s3_creds.secret_ref
  immutable       = true   # S3 Object Lock — the bucket must be created lock-enabled
}
```

| Argument | Required | Description |
|---|---|---|
| `name` | yes | Display name |
| `type` | yes | `nfs` or `s3` (the 6.2 WLM API values — `amazon_s3`/`other_s3_compatible` are Horizon aliases, not accepted here). Forces replacement if changed |
| `filesystem_export` | NFS only | Export path, e.g. `10.0.0.5:/exports/tvault` |
| `s3_endpoint_url` | S3 only | S3 endpoint URL |
| `s3_bucket` | S3 only | Bucket name (must already exist) |
| `secret_ref` | **S3 only, required** | Barbican secret href holding the S3 credentials (see the `openstack_keymanager_secret_v1` above) |
| `is_default` | no | Set as the default target (default: `false`) |
| `immutable` | S3 only | Enable S3 Object Lock immutability (the bucket must be Object-Lock enabled) |

---

### `t4o_workload`

A workload is a named group of VMs with a backup schedule. Running a workload creates snapshots.

```hcl
data "t4o_workload_types" "all" {}

resource "t4o_workload" "web_tier" {
  name             = "web-tier-backup"
  description      = "Weekly backup of web VMs"
  workload_type_id = data.t4o_workload_types.all.workload_types[0].id
  instance_ids     = ["<vm-uuid-1>", "<vm-uuid-2>"]
  backup_target_id = t4o_backup_target.nfs.id

  jobschedule = {
    enabled             = true
    start_date          = "2026-07-01 02:00:00"
    end_date            = "2030-12-31 00:00:00"
    interval            = "168"   # hours — weekly
    fullbackup_interval = 4       # full every 4th run
    retention_days      = 30
    snapshots_to_retain = 10
  }
}
```

| Argument | Required | Description |
|---|---|---|
| `name` | yes | Workload name |
| `workload_type_id` | yes | From `t4o_workload_types` data source |
| `instance_ids` | yes | List of Nova instance UUIDs to protect |
| `backup_target_id` | no | Bind to a specific backup target (the provider sends WLM's `backup_target_types`; omit to use the project default) |
| `policy_id` | no | Attach a shared workload policy |
| `encryption` | no | Encrypt this workload's backups (requires `secret_uuid`). Forces recreation if changed |
| `secret_uuid` | no | Barbican secret UUID with the encryption passphrase (required when `encryption = true`) — create it with `openstack_keymanager_secret_v1` |
| `jobschedule` | no | Inline schedule block (see above) |

> **Note:** T4O requires a Keystone trust for unattended scheduled backups. Run the `wlm_cloud_trust` playbook before enabling a schedule, or snapshots will fail silently.

---

### `t4o_workload_policy`

A reusable backup policy (schedule) that can be assigned to multiple workloads.

```hcl
resource "t4o_workload_policy" "daily" {
  name        = "daily-30d"
  description = "Daily backups, 30-day retention"

  jobschedule = {
    enabled             = true
    start_date          = "2026-07-01 02:00:00"
    end_date            = "2030-12-31 00:00:00"
    interval            = "24"
    fullbackup_interval = 7
    retention_days      = 30
    snapshots_to_retain = 30
  }
}
```

---

### `t4o_project_quota`

Sets a per-project backup quota — useful for governing tenants as code.

```hcl
resource "t4o_project_quota" "tenant_a_workloads" {
  project_id    = "<tenant-project-uuid>"
  quota_type_id = "<quota-type-uuid>"   # from the project's available quota types
  allowed_value = 50
}
```

| Argument | Required | Description |
|---|---|---|
| `project_id` | yes | Project the quota applies to (forces replacement if changed) |
| `quota_type_id` | yes | Quota-type UUID (forces replacement if changed) |
| `allowed_value` | yes | Allowed value for this quota type |
| `high_watermark` | no | Warning threshold (defaults to `allowed_value`) |

---

### `t4o_setting`

Manages a per-project T4O setting (a WLM key/value) — for example an email-notification address, so
you can configure the cloud end-to-end as code. Settings are keyed by `name`, so the resource `id`
is the setting name.

```hcl
resource "t4o_setting" "notify_email" {
  name  = "user_email_address_${var.user_id}"
  value = "ops@example.com"
  type  = "email"
}
```

| Argument | Required | Description |
|---|---|---|
| `name` | yes | Setting name, unique per project (forces replacement if changed) |
| `value` | yes | Setting value |
| `type` | no | Type hint (e.g. `email`, `string`); defaults to what WLM stores |
| `description` | no | Human-readable description |
| `category` | no | Setting category |

> **Reserved settings.** WLM-managed settings (`trust_id`, `cloud_unique_id`, `backup_target_id`)
> are rejected by this resource — managing them from Terraform can silently break scheduled backups.

---

## Data Sources

| Data source | Description |
|---|---|
| `t4o_workload_types` | Lists available workload types from the WLM |
| `t4o_workloads` | Lists all workloads in the project |
| `t4o_backup_targets` | Lists all registered backup targets |
| `t4o_license` | Returns current T4O license info |
| `t4o_quota` | Returns backup quota for the project |
| `t4o_quota_types` | Lists available backup quota types (use a `quota_types[*].id` as `t4o_project_quota.quota_type_id`) |

---

## Import

All resources support `terraform import`:

```bash
terraform import t4o_backup_target.nfs <backup-target-uuid>
terraform import t4o_workload.web_tier <workload-uuid>
terraform import t4o_workload_policy.daily <policy-uuid>
terraform import t4o_project_quota.tenant_a_workloads <allowed-quota-uuid>
terraform import t4o_setting.notify_email <setting-name>
```

---

## Examples

See [`examples/`](./examples/) for a complete working configuration covering a backup target, workload types data source, and NFS target lifecycle.

---

## Known Limitations

- **Snapshots and restores** are async imperative jobs and are out of scope for this provider. Use the T4O CLI, Horizon plugin, or DMS API directly.
- **`terraform import`** registers the resource and never crashes, but a follow-up `plan` may show a diff: WLM doesn't echo back every field (e.g. `jobschedule`/`secret_ref`) the way it accepts them, so imported state doesn't fully round-trip. Expected — reconcile by writing the config to match.
- **A policy already attached to a workload can't be updated** — WLM returns `500 "… assigned to workloads"`. Detach, edit, re-attach (or edit the policy before assigning).
- The WLM API returns `backup_targets` (plural) even for single-object responses — the provider handles this transparently.

---

## License

Apache 2.0

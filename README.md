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

**Role-Based Access Control (RBAC)**
- The provider delegates all permission checks natively to OpenStack. If a user lacks permission for an action (e.g., creating a shared policy), the API returns a 403 Forbidden and Terraform fails cleanly.

**Guarantees**
- Idempotent `plan` (no changes on a second run) and clean `destroy` (no orphans)

> Backup/restore *execution* (snapshot/restore) is a WLM operation, not a Terraform resource — out of the provider's scope.

---

## Quick start

```bash
# 1. Build & install the provider from source (see Install below)
git clone https://github.com/trilio-demo/terraform-provider-t4o.git
cd terraform-provider-t4o && make install

# 2. Authenticate the way you already do with OpenStack — source your openrc.
#    Both the t4o and openstack providers read the standard OS_* variables, so
#    credentials never touch your Terraform files.
source ~/openrc.sh                              # sets OS_AUTH_URL, OS_USERNAME, OS_PASSWORD, OS_PROJECT_ID, …

# 3. Try the bundled end-to-end example (cloud-admin): 2 projects + VMs, a backup target,
#    a shared policy, and a workload per project — all in one apply.
cd examples/demo
cp terraform.tfvars.example terraform.tfvars   # only non-credential inputs (nfs_export, image_name)
terraform init && terraform apply
```

> **Credentials come from the environment.** The provider reads `OS_AUTH_URL`, `OS_USERNAME`,
> `OS_PASSWORD`, `OS_PROJECT_ID`, and `OS_USER_DOMAIN_NAME` — so `source openrc.sh` is all you need.
> No passwords in `.tf` or `.tfvars`. (You can still set them explicitly if you prefer.)

`examples/` has three shapes. **As a regular project user**, [`tenant/`](./examples/tenant) is your
path — it just creates workloads, no admin rights. [`admin/`](./examples/admin) is the one-time
cloud-admin setup (shared target, policy, role grants), and [`demo/`](./examples/demo) bundles
everything into a single admin apply to see the provider work end to end. See
[`examples/README.md`](./examples/README.md).

---

## Requirements

- [Terraform >= 1.3](https://developer.hashicorp.com/terraform/install)
- [Go 1.22+](https://go.dev/dl/) — verify with `go version`
- Git
- OpenStack cloud with T4O 6.2 installed and the WLM service registered in Keystone

---

## Cloud prerequisites

Have these in place on the OpenStack cloud before you run anything:

- **T4O 6.2 deployed** with the WLM service registered in Keystone (service type `workloads`).
- **A valid, unexpired T4O license** applied before creating workloads.
- **Keystone trust roles**: the user that creates workloads should hold the roles in WLM's `trustee_role`
  (typically `creator`; some clouds also `member`). T4O uses a Keystone trust to run scheduled backups on the user's behalf.
- **RBAC role** for non-admin users: T4O's custom policy grants workload management via the `backup_admin` role.
  Not required if your cloud runs the default permissive policy.
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
      source  = "registry.terraform.io/trilio-demo/t4o"
      version = "~> 0.1"
    }
  }
}
```

---

## Provider Configuration

The provider reads the standard OpenStack `OS_*` environment variables, so after `source openrc.sh`
the provider block can be **empty**:

```hcl
# Recommended: credentials from the environment (source your openrc first).
provider "t4o" {}
```

It picks up `OS_AUTH_URL`, `OS_USERNAME`, `OS_PASSWORD`, `OS_PROJECT_ID`, and
`OS_USER_DOMAIN_NAME` (default domain `Default`). This keeps secrets out of your configuration —
the recommended practice for OpenStack + Terraform.

If you'd rather be explicit (e.g. managing several clouds in one config), every value can be set
directly and overrides the environment:

```hcl
provider "t4o" {
  auth_url    = "http://<keystone-host>:5000"
  username    = "backup-user"          # any project user with the backup role — admin not required
  password    = var.os_password        # supply via TF_VAR_os_password, not plaintext
  project_id  = "<project-uuid>"
  domain_name = "Default"
}
```

The provider discovers the WLM endpoint automatically from the Keystone service catalog (service type `workloads`, falling back to `workloadmgr`). No hardcoded ports needed.

| Argument | Description |
|---|---|
| `auth_url` | Keystone v3 endpoint (or `OS_AUTH_URL`) |
| `username` | OpenStack username (or `OS_USERNAME`) — a normal project user; admin is not required |
| `password` | OpenStack password (or `OS_PASSWORD`; use a variable, never plaintext) |
| `project_id` | Project UUID to scope operations to (or `OS_PROJECT_ID`) |
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

> **Note:** T4O uses a Keystone trust to run unattended scheduled backups. Ensure the trust is configured (e.g. via the `wlm_cloud_trust` playbook) before enabling a schedule.

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
> are rejected by this resource, since they're maintained by T4O itself and are not meant to be set by hand.

---

### `t4o_restore`

Manages a TrilioVault workload restore operation. This is most commonly used in Disaster Recovery scenarios to trigger a restore and adopt the resulting VMs into Terraform state.

```hcl
resource "t4o_restore" "dr_restore" {
  name        = "dr-failover-restore"
  description = "One-click restore triggered via Terraform"
  snapshot_id = "aa881fac-7189-4c9e-b18b-db508fc5af1f"
  type        = "oneclick"
}
```

| Argument | Required | Description |
|---|---|---|
| `name` | yes | Restore name (forces replacement if changed) |
| `snapshot_id` | yes | UUID of the snapshot to restore (forces replacement if changed) |
| `description` | no | Restore description (forces replacement if changed) |
| `type` | no | Restore type: `oneclick` (default), `selective`, or `inplace` (forces replacement if changed) |

> **Note:** A restore is inherently a one-time operation. Updating the resource's properties in Terraform will force a recreation, triggering a brand-new restore.

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
| `t4o_restore_details` | Retrieves details about a restore operation, crucially including the mapped IDs of the `restored_instances` to enable dynamic state adoption |

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

## Scope & operational notes

- **Snapshots and restores** are on-demand operations and are out of scope for this provider by design — it manages declarative configuration (targets, workloads, policies, quotas, settings). Trigger and manage backup runs with the T4O CLI, Horizon plugin, or DMS API.
- **`terraform import`** brings a resource under management; a follow-up `plan` may show a diff for fields the API doesn't echo back identically (e.g. `jobschedule`, `secret_ref`). Reconcile by writing the config to match the imported resource.
- **Update a policy before assigning it** to workloads. To change a policy that's already assigned, detach it, edit, and re-attach.
- Single-object and list responses are normalized transparently by the provider.

---

## License

Apache 2.0

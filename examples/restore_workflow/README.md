# Terraform Workflow for Trilio Restores

This example demonstrates how to use the Trilio `t4o` provider alongside the `openstack` provider to perform a restore and seamlessly adopt the newly restored VM into your Terraform state without it trying to recreate the missing original VM.

## The Challenge

When you manage an OpenStack VM with Terraform (`openstack_compute_instance_v2`), Terraform tracks it by its OpenStack UUID.
If that VM is deleted, and you restore it using Trilio, Trilio spins up a **brand new VM** with a **new UUID**.
If you simply run `terraform apply`, Terraform will notice the old UUID is gone and try to spin up a new VM from scratch, ignoring your restored VM.

## The Solution: Dynamic Import Blocks

Terraform 1.5 introduced `import` blocks, which allow you to tell Terraform to adopt an existing resource during the `plan` phase. By combining this with the `t4o_restore_details` data source, we can completely automate the adoption!

### Step-by-Step Workflow

#### 1. Initial Setup
Run `terraform apply` to create your initial VM and your `t4o_workload`. Let's say you take a snapshot of this workload.

#### 2. The Disaster
The VM is deleted from OpenStack.

#### 3. Trigger the Restore
In your `main.tf`, uncomment the `t4o_restore` resource (Step 2 in the file) and provide the `snapshot_id`. 

Run the apply targeting *only* the restore resource so that Trilio performs the restore first:
```bash
terraform apply -target=t4o_restore.dr_restore
```
*(Wait for the restore to complete...)*

#### 4. Clear the Dead State
Because the original VM was deleted in OpenStack but still exists in your local Terraform state file, Terraform will try to recreate it from scratch. You must tell Terraform to "forget" the old VM so it is allowed to adopt the new one.
```bash
terraform state rm openstack_compute_instance_v2.example_vm
```

#### 5. Adopt the State
Now, uncomment the dynamic `import` block and the `data.t4o_restore_details` source (Step 3 in the file).

Run a standard apply:
```bash
terraform apply
```

**What happens behind the scenes:**
1. Terraform looks at your `import` block.
2. It queries the `data.t4o_restore_details` data source to find out what VMs were created by `t4o_restore.dr_restore`.
3. It finds the new OpenStack UUID for your VM.
4. It intercepts the `openstack_compute_instance_v2.example_vm` resource, injects the new UUID into the state, and reports **0 to add, 0 to change, 0 to destroy**!

Your Terraform state is completely healed, and you can continue managing the restored VM as if nothing ever happened.

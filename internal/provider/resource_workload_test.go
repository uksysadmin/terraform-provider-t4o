package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// WLM (workloadmgr api/v1/workloads.py) reads the policy id from
// metadata["policy_id"] on workload create/update, not from a top-level
// field. workloadModelToRequest must therefore mirror policy_id into
// metadata, otherwise WLM rejects the create with
// "Please provide policy id from available policies: [...]".
func TestWorkloadModelToRequest_PolicyIDInMetadata(t *testing.T) {
	ctx := context.Background()

	instances, _ := types.ListValueFrom(ctx, types.StringType, []string{"vm-1"})
	m := &workloadModel{
		Name:           types.StringValue("tenant-a-workload"),
		WorkloadTypeID: types.StringValue("wt-1"),
		InstanceIDs:    instances,
		BackupTargetID: types.StringValue("bt-1"),
		PolicyID:       types.StringValue("pol-123"),
		JobSchedule:    types.ObjectNull(jobScheduleObjectType.AttrTypes),
	}

	req := workloadModelToRequest(ctx, m)

	if got := req.Metadata["policy_id"]; got != "pol-123" {
		t.Fatalf("expected metadata[policy_id]=pol-123, got %q (metadata=%v)", got, req.Metadata)
	}
	if req.PolicyID != "pol-123" {
		t.Fatalf("expected top-level PolicyID preserved, got %q", req.PolicyID)
	}
}

// When no policy is set, metadata must not carry an empty policy_id key.
func TestWorkloadModelToRequest_NoPolicyNoMetadata(t *testing.T) {
	ctx := context.Background()

	instances, _ := types.ListValueFrom(ctx, types.StringType, []string{"vm-1"})
	m := &workloadModel{
		Name:           types.StringValue("scheduled-workload"),
		WorkloadTypeID: types.StringValue("wt-1"),
		InstanceIDs:    instances,
		BackupTargetID: types.StringValue("bt-1"),
		PolicyID:       types.StringValue(""),
		JobSchedule:    types.ObjectNull(jobScheduleObjectType.AttrTypes),
	}

	req := workloadModelToRequest(ctx, m)

	if _, ok := req.Metadata["policy_id"]; ok {
		t.Fatalf("expected no policy_id in metadata when unset, got %v", req.Metadata)
	}
}

// WLM does not preserve instance ordering: the workload GET/create response can
// return the same instances in a different order than the configured list.
// instance_ids is an ordered ListAttribute, so adopting the API order verbatim
// triggers "provider produced inconsistent result after apply" and a perpetual
// diff. reorderToConfigured must keep the configured order for overlapping
// members and append only genuinely-new IDs.
func TestReorderToConfigured(t *testing.T) {
	ctx := context.Background()

	asSlice := func(l types.List) []string {
		var out []string
		l.ElementsAs(ctx, &out, false)
		return out
	}
	eq := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	configured, _ := types.ListValueFrom(ctx, types.StringType, []string{"vm-a", "vm-b"})

	// API returns the same set in a different order -> keep configured order.
	got := asSlice(reorderToConfigured(ctx, configured, []string{"vm-b", "vm-a"}))
	if !eq(got, []string{"vm-a", "vm-b"}) {
		t.Fatalf("reorder same-set: expected [vm-a vm-b], got %v", got)
	}

	// A new instance was added (update path) -> existing order kept, new appended.
	got = asSlice(reorderToConfigured(ctx, configured, []string{"vm-c", "vm-b", "vm-a"}))
	if !eq(got, []string{"vm-a", "vm-b", "vm-c"}) {
		t.Fatalf("reorder add: expected [vm-a vm-b vm-c], got %v", got)
	}

	// An instance was removed -> it drops out (genuine drift still surfaces).
	got = asSlice(reorderToConfigured(ctx, configured, []string{"vm-a"}))
	if !eq(got, []string{"vm-a"}) {
		t.Fatalf("reorder remove: expected [vm-a], got %v", got)
	}

	// No usable configured order (null) -> fall back to API order as-is.
	got = asSlice(reorderToConfigured(ctx, types.ListNull(types.StringType), []string{"vm-x", "vm-y"}))
	if !eq(got, []string{"vm-x", "vm-y"}) {
		t.Fatalf("reorder null-config: expected [vm-x vm-y], got %v", got)
	}
}

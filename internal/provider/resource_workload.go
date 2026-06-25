package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

var _ resource.Resource = &WorkloadResource{}
var _ resource.ResourceWithImportState = &WorkloadResource{}

func NewWorkloadResource() resource.Resource { return &WorkloadResource{} }

type WorkloadResource struct{ client *wlm.Client }

// jobScheduleObjectType defines the attr.Type for a jobschedule object.
var jobScheduleObjectType = types.ObjectType{
	AttrTypes: map[string]attr.Type{
		"enabled":             types.BoolType,
		"start_date":          types.StringType,
		"end_date":            types.StringType,
		"interval":            types.StringType,
		"fullbackup_interval": types.Int64Type,
		"retention_days":      types.Int64Type,
		"snapshots_to_retain": types.Int64Type,
	},
}

type workloadModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	WorkloadTypeID types.String `tfsdk:"workload_type_id"`
	InstanceIDs    types.List   `tfsdk:"instance_ids"`
	BackupTargetID types.String `tfsdk:"backup_target_id"`
	PolicyID       types.String `tfsdk:"policy_id"`
	Encryption     types.Bool   `tfsdk:"encryption"`
	SecretUUID     types.String `tfsdk:"secret_uuid"`
	JobSchedule    types.Object `tfsdk:"jobschedule"`
}

type jobScheduleModel struct {
	Enabled            types.Bool   `tfsdk:"enabled"`
	StartDate          types.String `tfsdk:"start_date"`
	EndDate            types.String `tfsdk:"end_date"`
	Interval           types.String `tfsdk:"interval"`
	FullBackupInterval types.Int64  `tfsdk:"fullbackup_interval"`
	RetentionDays      types.Int64  `tfsdk:"retention_days"`
	SnapshotsToRetain  types.Int64  `tfsdk:"snapshots_to_retain"`
}

func (r *WorkloadResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workload"
}

func (r *WorkloadResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a TrilioVault for OpenStack workload (protection plan).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Workload UUID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Workload name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional description.",
			},
			"workload_type_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Workload type UUID from `data.t4o_workload_types`. Changing forces recreation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"instance_ids": schema.ListAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "List of Nova instance UUIDs to include in this workload.",
			},
			"backup_target_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "UUID of the backup target.",
			},
			"policy_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "UUID of a workload policy. Mutually exclusive with inline `jobschedule`.",
			},
			"encryption": schema.BoolAttribute{
				Optional: true,
				MarkdownDescription: "Encrypt this workload's backups (Barbican-backed). Requires `secret_uuid`. " +
					"Set at creation — changing it forces recreation.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"secret_uuid": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Barbican secret UUID holding the encryption passphrase (required when " +
					"`encryption = true`). Create it with `openstack_keymanager_secret_v1`. Forces recreation if changed.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"jobschedule": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Inline backup schedule. If `enabled = true` without a cloud trust, a warning is emitted.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Required:            true,
						MarkdownDescription: "Enable automatic backup scheduling.",
					},
					"start_date": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Schedule start date (ISO 8601).",
					},
					"end_date": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Schedule end date (ISO 8601).",
					},
					"interval": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Backup frequency, e.g. `daily`, `weekly`, `hours 12`.",
					},
					"fullbackup_interval": schema.Int64Attribute{
						Optional:            true,
						MarkdownDescription: "Full backup every N incremental runs.",
					},
					"retention_days": schema.Int64Attribute{
						Optional:            true,
						MarkdownDescription: "Retain snapshots for N days.",
					},
					"snapshots_to_retain": schema.Int64Attribute{
						Optional:            true,
						MarkdownDescription: "Number of snapshots to keep.",
					},
				},
			},
		},
	}
}

func (r *WorkloadResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*wlm.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *wlm.Client, got %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *WorkloadResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workloadModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := workloadModelToRequest(ctx, &plan)

	// WLM 6.2 binds a workload to a target via `backup_target_types` (a backup_target_type
	// name/UUID), NOT `backup_target_id` — which it silently ignores, sending backups to the
	// DEFAULT target. Resolve the chosen target's backup_target_type name and send it.
	r.bindBackupTargetType(ctx, &plan, &apiReq, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if scheduleEnabled(&plan, ctx) {
		resp.Diagnostics.AddWarning(
			"Cloud trust required for scheduled backups",
			"jobschedule.enabled is true. Ensure a Keystone trust (trustee role) exists for this project. "+
				"Run the wlm_cloud_trust playbook; without it, scheduled backups will fail silently.",
		)
	}

	wl, err := r.client.CreateWorkload(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Create workload failed", err.Error())
		return
	}

	wl, err = r.client.PollWorkload(ctx, wl.ID, 30*time.Minute)
	if err != nil {
		resp.Diagnostics.AddError("Workload did not become available", err.Error())
		return
	}

	workloadToModel(ctx, wl, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WorkloadResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workloadModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wl, err := r.client.GetWorkload(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read workload failed", err.Error())
		return
	}
	if wl == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	workloadToModel(ctx, wl, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *WorkloadResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan workloadModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := workloadModelToRequest(ctx, &plan)

	r.bindBackupTargetType(ctx, &plan, &apiReq, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	wl, err := r.client.UpdateWorkload(ctx, plan.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Update workload failed", err.Error())
		return
	}

	wl, err = r.client.PollWorkload(ctx, wl.ID, 15*time.Minute)
	if err != nil {
		resp.Diagnostics.AddError("Workload did not become available after update", err.Error())
		return
	}

	workloadToModel(ctx, wl, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WorkloadResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workloadModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteWorkload(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete workload failed", err.Error())
	}
}

func (r *WorkloadResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	wl, err := r.client.GetWorkload(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Import workload failed", err.Error())
		return
	}
	if wl == nil {
		resp.Diagnostics.AddError("Import failed", fmt.Sprintf("workload %q not found", req.ID))
		return
	}
	var state workloadModel
	workloadToModel(ctx, wl, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

func scheduleEnabled(m *workloadModel, ctx context.Context) bool {
	if m.JobSchedule.IsNull() || m.JobSchedule.IsUnknown() {
		return false
	}
	var s jobScheduleModel
	m.JobSchedule.As(ctx, &s, basetypes.ObjectAsOptions{})
	return s.Enabled.ValueBool()
}

// bindBackupTargetType resolves the configured backup_target_id to the backup_target_type
// NAME that WLM 6.2 actually binds on (sent as `backup_target_types`). WLM ignores
// `backup_target_id`, so without this a workload silently lands on the default target.
func (r *WorkloadResource) bindBackupTargetType(ctx context.Context, m *workloadModel, req *wlm.WorkloadRequest, diags *diag.Diagnostics) {
	id := m.BackupTargetID.ValueString()
	if id == "" {
		return // no explicit target → WLM uses the project default (intended)
	}
	bt, err := r.client.GetBackupTarget(ctx, id)
	if err != nil {
		diags.AddError("Resolve backup target failed",
			fmt.Sprintf("could not read backup_target %q to bind the workload: %s", id, err))
		return
	}
	if bt == nil || len(bt.BackupTargetType) == 0 || bt.BackupTargetType[0].Name == "" {
		diags.AddError("Cannot bind workload to backup target",
			fmt.Sprintf("backup_target %q exposes no backup_target_types; WLM 6.2 binds workloads via "+
				"backup_target_types, so it would otherwise send backups to the default target.", id))
		return
	}
	req.BackupTargetTypes = bt.BackupTargetType[0].Name
}

func workloadModelToRequest(ctx context.Context, m *workloadModel) wlm.WorkloadRequest {
	req := wlm.WorkloadRequest{
		Name:           m.Name.ValueString(),
		Description:    m.Description.ValueString(),
		WorkloadTypeID: m.WorkloadTypeID.ValueString(),
		BackupTargetID: m.BackupTargetID.ValueString(),
		PolicyID:       m.PolicyID.ValueString(),
		Encryption:     m.Encryption.ValueBool(),
		SecretUUID:     m.SecretUUID.ValueString(),
	}

	// WLM reads the policy id from metadata["policy_id"] on create/update
	// (workloadmgr api/v1/workloads.py: policy_id = metadata.get("policy_id")),
	// not from a top-level field. Without it, WLM rejects the create with
	// "Please provide policy id from available policies: [...]" even when the
	// policy is assigned to the project.
	if pid := m.PolicyID.ValueString(); pid != "" {
		req.Metadata = map[string]string{"policy_id": pid}
	}

	var instanceIDs []string
	m.InstanceIDs.ElementsAs(ctx, &instanceIDs, false)
	for _, id := range instanceIDs {
		req.Instances = append(req.Instances, wlm.WorkloadInstance{
			InstanceID: id,
			Include:    true,
		})
	}

	if !m.JobSchedule.IsNull() && !m.JobSchedule.IsUnknown() {
		var s jobScheduleModel
		m.JobSchedule.As(ctx, &s, basetypes.ObjectAsOptions{})
		req.JobSchedule = &wlm.JobSchedule{
			Enabled:            s.Enabled.ValueBool(),
			StartDate:          s.StartDate.ValueString(),
			EndDate:            s.EndDate.ValueString(),
			Interval:           s.Interval.ValueString(),
			FullBackupInterval: int(s.FullBackupInterval.ValueInt64()),
			RetentionDays:      int(s.RetentionDays.ValueInt64()),
			SnapshotsToRetain:  int(s.SnapshotsToRetain.ValueInt64()),
		}
	}

	return req
}

func workloadToModel(ctx context.Context, wl *wlm.Workload, m *workloadModel) {
	m.ID = types.StringValue(wl.ID)
	m.Name = types.StringValue(wl.Name)
	// WLM substitutes the sentinel "no-description" when no description was
	// supplied and echoes it back on create/GET. description is an Optional
	// (non-Computed) attribute, so adopting that sentinel over a null-config
	// value makes the post-apply state differ from the plan ("was null, but now
	// 'no-description'") — "provider produced inconsistent result after apply".
	// Only adopt a genuine, non-sentinel description; otherwise preserve the
	// configured (plan/prior-state) value.
	if wl.Description != "" && wl.Description != "no-description" {
		m.Description = types.StringValue(wl.Description)
	}
	m.WorkloadTypeID = types.StringValue(wl.WorkloadTypeID)

	// WLM does not echo these back reliably: backup_target_id is never returned as
	// a UUID (only a metadata filesystem path), policy_id is absent from some
	// workload GET shapes, and the non-detail workload GET omits instances entirely
	// (and uses a different key, "id", than the create request's "instance-id").
	// Overwriting with the empty API value would corrupt state and trigger
	// "provider produced inconsistent result after apply". Preserve the configured
	// (plan/prior-state) value whenever the API didn't return one.
	if wl.BackupTargetID != "" {
		m.BackupTargetID = types.StringValue(wl.BackupTargetID)
	}
	if wl.PolicyID != "" {
		m.PolicyID = types.StringValue(wl.PolicyID)
	}

	instIDs := make([]string, 0, len(wl.Instances))
	for _, inst := range wl.Instances {
		if id := inst.ResolvedID(); id != "" {
			instIDs = append(instIDs, id)
		}
	}
	if len(instIDs) > 0 {
		// WLM does not preserve instance ordering: the GET/create response can
		// return the same instances in a different order than the configured
		// (plan/prior-state) list. instance_ids is an ordered ListAttribute, so
		// blindly adopting the API order makes the planned value differ from the
		// applied result ("provider produced inconsistent result after apply")
		// and produces a perpetual diff. Reorder the API set to follow the
		// configured order, then append any genuinely-new IDs the API returned.
		m.InstanceIDs = reorderToConfigured(ctx, m.InstanceIDs, instIDs)
	}

	// jobschedule is mutually exclusive with policy_id: when the workload is driven
	// by a policy, the config has no inline jobschedule but WLM still returns the
	// policy's schedule. Writing that into state would create a perpetual diff
	// against the null config. Only refresh jobschedule from the API when the user
	// actually configured an inline schedule (model value already non-null).
	configuredInlineSchedule := !m.JobSchedule.IsNull() && !m.JobSchedule.IsUnknown()

	if wl.JobSchedule != nil && configuredInlineSchedule {
		s := wl.JobSchedule
		objVal, _ := types.ObjectValue(
			jobScheduleObjectType.AttrTypes,
			map[string]attr.Value{
				"enabled":             types.BoolValue(s.Enabled),
				"start_date":          types.StringValue(s.StartDate),
				"end_date":            types.StringValue(s.EndDate),
				"interval":            types.StringValue(s.Interval),
				"fullbackup_interval": types.Int64Value(int64(s.FullBackupInterval)),
				"retention_days":      types.Int64Value(int64(s.RetentionDays)),
				"snapshots_to_retain": types.Int64Value(int64(s.SnapshotsToRetain)),
			},
		)
		m.JobSchedule = objVal
	} else {
		// No inline schedule reflected into state. Covers three cases, all of which
		// want a properly-typed null:
		//   - user configured inline but the API returned none,
		//   - policy-driven workload (config jobschedule is null; writing the policy's
		//     schedule here would create a perpetual diff), and
		//   - ImportState, where the zero-value model holds a TYPELESS object that
		//     State.Set cannot serialize (Value Conversion Error on path jobschedule).
		// Always emit a null with the correct attribute types.
		m.JobSchedule = types.ObjectNull(jobScheduleObjectType.AttrTypes)
	}
}

// reorderToConfigured returns a list value whose ordering follows the configured
// (plan/prior-state) instance_ids where the members overlap with what the API
// returned, with any new API-only IDs appended in API order. This keeps the
// applied result consistent with the planned ordered list even though WLM does
// not preserve instance order, and lets genuine adds/removes still surface as a
// diff. If there is no usable configured order, the API order is used as-is.
func reorderToConfigured(ctx context.Context, configured types.List, apiIDs []string) types.List {
	apiSet := make(map[string]bool, len(apiIDs))
	for _, id := range apiIDs {
		apiSet[id] = true
	}

	ordered := make([]string, 0, len(apiIDs))
	if !configured.IsNull() && !configured.IsUnknown() {
		var prior []string
		configured.ElementsAs(ctx, &prior, false)
		seen := make(map[string]bool, len(prior))
		for _, id := range prior {
			if apiSet[id] && !seen[id] {
				ordered = append(ordered, id)
				seen[id] = true
			}
		}
		for _, id := range apiIDs {
			if !seen[id] {
				ordered = append(ordered, id)
				seen[id] = true
			}
		}
	} else {
		ordered = apiIDs
	}

	listVal, _ := types.ListValueFrom(ctx, types.StringType, ordered)
	return listVal
}

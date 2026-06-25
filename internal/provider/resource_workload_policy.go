package provider

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

var _ resource.Resource = &WorkloadPolicyResource{}
var _ resource.ResourceWithImportState = &WorkloadPolicyResource{}

func NewWorkloadPolicyResource() resource.Resource { return &WorkloadPolicyResource{} }

type WorkloadPolicyResource struct{ client *wlm.Client }

type workloadPolicyModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Description      types.String `tfsdk:"description"`
	AssignedProjects types.List   `tfsdk:"assigned_projects"`
	JobSchedule      types.Object `tfsdk:"jobschedule"`
}

func (r *WorkloadPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workload_policy"
}

func (r *WorkloadPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a reusable TrilioVault workload policy (shared backup schedule).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Policy UUID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Policy name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional description.",
			},
			"assigned_projects": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Project UUIDs to assign this policy to.",
			},
			"jobschedule": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Backup schedule for this policy.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{
						Required: true,
					},
					"start_date":          schema.StringAttribute{Optional: true},
					"end_date":            schema.StringAttribute{Optional: true},
					"interval":            schema.StringAttribute{Optional: true},
					"fullbackup_interval": schema.Int64Attribute{Optional: true},
					"retention_days":      schema.Int64Attribute{Optional: true},
					"snapshots_to_retain": schema.Int64Attribute{Optional: true},
				},
			},
		},
	}
}

func (r *WorkloadPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *WorkloadPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workloadPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pol, err := r.client.CreateWorkloadPolicy(ctx, policyModelToRequest(ctx, &plan))
	if err != nil {
		resp.Diagnostics.AddError("Create workload_policy failed", err.Error())
		return
	}

	// Only the server-assigned ID is read back. WLM returns field_values in a
	// serialized shape that does not map 1:1 to our input, so we keep the planned
	// values in state to satisfy Terraform's plan/apply consistency check.
	plan.ID = types.StringValue(pol.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WorkloadPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workloadPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pol, err := r.client.GetWorkloadPolicy(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read workload_policy failed", err.Error())
		return
	}
	if pol == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Existence confirmed. field_values is not round-tripped from the API, so we
	// leave the rest of state untouched to avoid spurious diffs.
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *WorkloadPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan workloadPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pol, err := r.client.UpdateWorkloadPolicy(ctx, plan.ID.ValueString(), policyModelToRequest(ctx, &plan))
	if err != nil {
		resp.Diagnostics.AddError("Update workload_policy failed", err.Error())
		return
	}

	plan.ID = types.StringValue(pol.ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *WorkloadPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workloadPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteWorkloadPolicy(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete workload_policy failed", err.Error())
	}
}

func (r *WorkloadPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	pol, err := r.client.GetWorkloadPolicy(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Import workload_policy failed", err.Error())
		return
	}
	if pol == nil {
		resp.Diagnostics.AddError("Import failed", fmt.Sprintf("workload_policy %q not found", req.ID))
		return
	}
	// field_values is not round-tripped, so the jobschedule block must be
	// (re)specified in configuration after import.
	state := workloadPolicyModel{
		ID:               types.StringValue(pol.ID),
		Name:             types.StringValue(pol.Name),
		Description:      types.StringValue(pol.Description),
		AssignedProjects: types.ListNull(types.StringType),
		JobSchedule:      types.ObjectNull(jobScheduleObjectType.AttrTypes),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// Defaults used when the jobschedule block (or a field within it) is omitted, so
// the advanced-scheduler field_values payload is always valid.
const (
	defaultHourlyInterval = "24" // must be one of 1,2,3,4,6,8,12,24
	defaultRetention      = "30"
	defaultStartTime      = "10:00 PM" // %I:%M %p
)

func policyModelToRequest(ctx context.Context, m *workloadPolicyModel) wlm.WorkloadPolicyRequest {
	req := wlm.WorkloadPolicyRequest{
		DisplayName:        m.Name.ValueString(),
		DisplayDescription: m.Description.ValueString(),
		AssignedProjects:   []string{},
	}

	if !m.AssignedProjects.IsNull() && !m.AssignedProjects.IsUnknown() {
		m.AssignedProjects.ElementsAs(ctx, &req.AssignedProjects, false)
	}

	interval := defaultHourlyInterval
	retention := defaultRetention
	startTime := defaultStartTime

	if !m.JobSchedule.IsNull() && !m.JobSchedule.IsUnknown() {
		var s jobScheduleModel
		m.JobSchedule.As(ctx, &s, basetypes.ObjectAsOptions{})
		if v := s.Interval.ValueString(); v != "" {
			interval = v
		}
		if v := s.SnapshotsToRetain.ValueInt64(); v > 0 {
			retention = strconv.FormatInt(v, 10)
		} else if v := s.RetentionDays.ValueInt64(); v > 0 {
			retention = strconv.FormatInt(v, 10)
		}
	}

	// WLM 6.2 advanced scheduler: field_values must carry exactly the 8 seeded
	// policy fields. We drive the hourly track and leave the rest present but
	// empty. hourly is mandatory and hourly.interval must be a valid hour count.
	req.FieldValues = map[string]interface{}{
		"hourly": map[string]interface{}{
			"interval":      interval,
			"retention":     retention,
			"snapshot_type": "incremental",
		},
		"daily":           map[string]interface{}{},
		"weekly":          map[string]interface{}{},
		"monthly":         map[string]interface{}{},
		"yearly":          map[string]interface{}{},
		"manual":          map[string]interface{}{"retention": retention},
		"retentionmanual": map[string]interface{}{"retentionmanual": retention}, // must be a digit; WLM rejects null/empty
		"start_time":      startTime,
	}

	return req
}

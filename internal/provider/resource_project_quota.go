package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

var _ resource.Resource = &ProjectQuotaResource{}
var _ resource.ResourceWithImportState = &ProjectQuotaResource{}

func NewProjectQuotaResource() resource.Resource { return &ProjectQuotaResource{} }

type ProjectQuotaResource struct{ client *wlm.Client }

type projectQuotaModel struct {
	ID            types.String `tfsdk:"id"`
	ProjectID     types.String `tfsdk:"project_id"`
	QuotaTypeID   types.String `tfsdk:"quota_type_id"`
	AllowedValue  types.Int64  `tfsdk:"allowed_value"`
	HighWatermark types.Int64  `tfsdk:"high_watermark"`
}

func (r *ProjectQuotaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_quota"
}

func (r *ProjectQuotaResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Sets a per-project T4O backup quota (an `allowed_quota` assignment). " +
			"Useful for governing tenants as code.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Allowed-quota UUID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"project_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project UUID the quota applies to. Changing forces recreation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"quota_type_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Quota-type UUID (from the project's available quota types). Changing forces recreation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"allowed_value": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "Allowed quota value for this type (e.g. number of workloads, bytes).",
			},
			"high_watermark": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Warning threshold; defaults to `allowed_value` when omitted.",
			},
		},
	}
}

func (r *ProjectQuotaResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func pqToRequest(m *projectQuotaModel) wlm.AllowedQuotaRequest {
	hw := m.AllowedValue.ValueInt64()
	if !m.HighWatermark.IsNull() && !m.HighWatermark.IsUnknown() {
		hw = m.HighWatermark.ValueInt64()
	}
	return wlm.AllowedQuotaRequest{
		QuotaTypeID:   m.QuotaTypeID.ValueString(),
		ProjectID:     m.ProjectID.ValueString(),
		AllowedValue:  m.AllowedValue.ValueInt64(),
		HighWatermark: hw,
	}
}

func pqToModel(q *wlm.AllowedQuota, m *projectQuotaModel) {
	m.ID = types.StringValue(q.ID)
	if q.ProjectID != "" {
		m.ProjectID = types.StringValue(q.ProjectID)
	}
	if q.QuotaTypeID != "" {
		m.QuotaTypeID = types.StringValue(q.QuotaTypeID)
	}
	m.AllowedValue = types.Int64Value(q.AllowedValue)
	m.HighWatermark = types.Int64Value(q.HighWatermark)
}

func (r *ProjectQuotaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan projectQuotaModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	q, err := r.client.CreateAllowedQuota(ctx, pqToRequest(&plan))
	if err != nil {
		resp.Diagnostics.AddError("Create project_quota failed", err.Error())
		return
	}
	pqToModel(q, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ProjectQuotaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state projectQuotaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	q, err := r.client.GetAllowedQuota(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read project_quota failed", err.Error())
		return
	}
	if q == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	pqToModel(q, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ProjectQuotaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan projectQuotaModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	q, err := r.client.UpdateAllowedQuota(ctx, plan.ID.ValueString(), pqToRequest(&plan))
	if err != nil {
		resp.Diagnostics.AddError("Update project_quota failed", err.Error())
		return
	}
	pqToModel(q, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ProjectQuotaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state projectQuotaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteAllowedQuota(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete project_quota failed", err.Error())
	}
}

func (r *ProjectQuotaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	q, err := r.client.GetAllowedQuota(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Import project_quota failed", err.Error())
		return
	}
	if q == nil {
		resp.Diagnostics.AddError("Import failed", fmt.Sprintf("allowed_quota %q not found", req.ID))
		return
	}
	var state projectQuotaModel
	pqToModel(q, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

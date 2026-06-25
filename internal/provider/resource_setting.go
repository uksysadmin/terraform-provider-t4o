package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

var _ resource.Resource = &SettingResource{}
var _ resource.ResourceWithImportState = &SettingResource{}

func NewSettingResource() resource.Resource { return &SettingResource{} }

type SettingResource struct{ client *wlm.Client }

type settingModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Value       types.String `tfsdk:"value"`
	Type        types.String `tfsdk:"type"`
	Description types.String `tfsdk:"description"`
	Category    types.String `tfsdk:"category"`
}

// reservedSettingNames are WLM-managed settings that Terraform must never touch —
// clobbering them (e.g. the Keystone trust) silently breaks scheduled backups.
var reservedSettingNames = map[string]bool{
	"trust_id":         true,
	"cloud_unique_id":  true,
	"backup_target_id": true,
}

func (r *SettingResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_setting"
}

func (r *SettingResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a per-project T4O setting (a WLM key/value, e.g. an email-notification address). " +
			"Settings are keyed by `name`, so the resource `id` is the setting name. " +
			"**Do not** use this to manage WLM-internal settings such as `trust_id` — those are reserved and rejected.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Setting name (settings are name-keyed).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Setting name (unique per project). Changing forces recreation.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"value": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Setting value.",
			},
			"type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Setting type hint (e.g. `email`, `string`). Defaults to what WLM stores.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Human-readable description.",
			},
			"category": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Setting category.",
			},
		},
	}
}

func (r *SettingResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// guardReserved blocks WLM-managed setting names to avoid breaking backups.
func guardReserved(name string, diags *diag.Diagnostics) bool {
	if reservedSettingNames[strings.ToLower(strings.TrimSpace(name))] {
		diags.AddAttributeError(path.Root("name"),
			"Reserved setting name",
			fmt.Sprintf("%q is a WLM-managed setting and cannot be managed by Terraform — "+
				"changing it can silently break scheduled backups.", name))
		return true
	}
	return false
}

func settingToRequest(m *settingModel) wlm.SettingItem {
	s := wlm.SettingItem{
		Name:  m.Name.ValueString(),
		Value: m.Value.ValueString(),
	}
	if !m.Type.IsNull() && !m.Type.IsUnknown() {
		s.Type = m.Type.ValueString()
	}
	if !m.Description.IsNull() {
		s.Description = m.Description.ValueString()
	}
	if !m.Category.IsNull() {
		s.Category = m.Category.ValueString()
	}
	return s
}

func settingToModel(s *wlm.SettingItem, m *settingModel) {
	m.ID = types.StringValue(s.Name)
	m.Name = types.StringValue(s.Name)
	m.Value = types.StringValue(s.Value)
	if s.Type != "" {
		m.Type = types.StringValue(s.Type)
	}
}

func (r *SettingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan settingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if guardReserved(plan.Name.ValueString(), &resp.Diagnostics) {
		return
	}
	s, err := r.client.CreateSetting(ctx, settingToRequest(&plan))
	if err != nil {
		resp.Diagnostics.AddError("Create setting failed", err.Error())
		return
	}
	settingToModel(s, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SettingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state settingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.GetSetting(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read setting failed", err.Error())
		return
	}
	if s == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	settingToModel(s, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SettingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan settingModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if guardReserved(plan.Name.ValueString(), &resp.Diagnostics) {
		return
	}
	s, err := r.client.UpdateSetting(ctx, settingToRequest(&plan))
	if err != nil {
		resp.Diagnostics.AddError("Update setting failed", err.Error())
		return
	}
	settingToModel(s, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SettingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state settingModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DeleteSetting(ctx, state.Name.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete setting failed", err.Error())
	}
}

func (r *SettingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	s, err := r.client.GetSetting(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Import setting failed", err.Error())
		return
	}
	if s == nil {
		resp.Diagnostics.AddError("Import failed", fmt.Sprintf("setting %q not found", req.ID))
		return
	}
	var state settingModel
	settingToModel(s, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

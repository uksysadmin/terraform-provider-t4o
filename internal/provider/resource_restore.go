package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

var _ resource.Resource = &RestoreResource{}
var _ resource.ResourceWithImportState = &RestoreResource{}

func NewRestoreResource() resource.Resource { return &RestoreResource{} }

type RestoreResource struct{ client *wlm.Client }

type restoreModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	SnapshotID  types.String `tfsdk:"snapshot_id"`
	Type        types.String `tfsdk:"type"`
	Status      types.String `tfsdk:"status"`
}

func (r *RestoreResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_restore"
}

func (r *RestoreResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a TrilioVault workload restore operation.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Restore UUID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Restore name.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Restore description.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"snapshot_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the snapshot to restore.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Restore type: 'oneclick', 'selective', or 'inplace'.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Restore status.",
			},
		},
	}
}

func (r *RestoreResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RestoreResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan restoreModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	restoreType := plan.Type.ValueString()
	if restoreType == "" {
		restoreType = "oneclick"
	}

	apiReq := wlm.RestoreRequest{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		SnapshotID:  plan.SnapshotID.ValueString(),
		Options: map[string]interface{}{
			"type":         "openstack",
			"restore_type": restoreType,
		},
	}
	if restoreType == "oneclick" {
		apiReq.Options["oneclickrestore"] = true
	}

	res, err := r.client.CreateRestore(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Create restore failed", err.Error())
		return
	}

	res, err = r.client.PollRestore(ctx, res.ID, 60*time.Minute)
	if err != nil {
		resp.Diagnostics.AddError("Restore failed or timed out", err.Error())
		return
	}

	plan.ID = types.StringValue(res.ID)
	plan.Status = types.StringValue(res.Status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RestoreResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state restoreModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := r.client.GetRestore(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read restore failed", err.Error())
		return
	}
	if res == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.Status = types.StringValue(res.Status)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *RestoreResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "Restores cannot be updated. They must be recreated.")
}

func (r *RestoreResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state restoreModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteRestore(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete restore failed", err.Error())
	}
}

func (r *RestoreResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

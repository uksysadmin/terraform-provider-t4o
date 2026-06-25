package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

var _ resource.Resource = &BackupTargetResource{}
var _ resource.ResourceWithImportState = &BackupTargetResource{}

func NewBackupTargetResource() resource.Resource { return &BackupTargetResource{} }

type BackupTargetResource struct{ client *wlm.Client }

type backupTargetModel struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Type             types.String `tfsdk:"type"`
	FilesystemExport types.String `tfsdk:"filesystem_export"`
	S3EndpointURL    types.String `tfsdk:"s3_endpoint_url"`
	S3Bucket         types.String `tfsdk:"s3_bucket"`
	SecretRef        types.String `tfsdk:"secret_ref"`
	IsDefault        types.Bool   `tfsdk:"is_default"`
	Immutable        types.Bool   `tfsdk:"immutable"`
}

func (r *BackupTargetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup_target"
}

func (r *BackupTargetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a TrilioVault for OpenStack backup target (NFS or S3-compatible).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Backup target UUID.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Display name for the backup target.",
			},
			"type": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Target type. The T4O 6.2 WLM API accepts `nfs` and `s3` " +
					"(the `amazon_s3` / `other_s3_compatible` labels are Horizon/migration aliases). " +
					"Use `nfs` with `filesystem_export`, or `s3` with `s3_bucket`, `s3_endpoint_url` and `secret_ref`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"filesystem_export": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "NFS export path, e.g. `10.0.0.5:/exports/tvault`. Required when `type = nfs`.",
			},
			"s3_endpoint_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "S3 endpoint URL for non-Amazon S3.",
			},
			"s3_bucket": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "S3 bucket name. Required when `type = s3`. The bucket must already exist.",
			},
			"secret_ref": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Barbican secret href holding the S3 credentials (a JSON payload with " +
					"`VAULT_S3_ACCESS_KEY_ID`, `VAULT_S3_SECRET_ACCESS_KEY`, `VAULT_S3_BUCKET`, " +
					"`VAULT_STORAGE_S3_EXPORT`, and optional `VAULT_S3_ENDPOINT_URL`/`VAULT_S3_REGION_NAME`/`VAULT_S3_SSL`). " +
					"Required when `type = s3`; WLM validates that the URL is reachable. " +
					"Create it with `openstack_keymanager_secret_v1` (see examples/admin).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"is_default": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether this is the default backup target for the project.",
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"immutable": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Enable S3 Object Lock immutability. S3 targets only. Changing forces recreation.",
				// UseStateForUnknown keeps this computed value from prior state when the
				// user does not change it. Without it, an unrelated update (e.g. a rename)
				// re-plans this computed attribute as "(known after apply)", and because it
				// also carries RequiresReplace that spuriously forces a destroy+recreate.
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown(), boolplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *BackupTargetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *BackupTargetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan backupTargetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := wlm.BackupTargetRequest{
		Name:             plan.Name.ValueString(),
		Type:             plan.Type.ValueString(),
		FilesystemExport: plan.FilesystemExport.ValueString(),
		S3EndpointURL:    plan.S3EndpointURL.ValueString(),
		S3Bucket:         plan.S3Bucket.ValueString(),
		SecretRef:        plan.SecretRef.ValueString(),
		IsDefault:        plan.IsDefault.ValueBool(),
		Immutable:        plan.Immutable.ValueBool(),
	}

	bt, err := r.client.CreateBackupTarget(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Create backup_target failed", err.Error())
		return
	}

	btToModel(bt, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BackupTargetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state backupTargetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	bt, err := r.client.GetBackupTarget(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read backup_target failed", err.Error())
		return
	}
	if bt == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	btToModel(bt, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *BackupTargetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan backupTargetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := wlm.BackupTargetRequest{
		Name:             plan.Name.ValueString(),
		Type:             plan.Type.ValueString(),
		FilesystemExport: plan.FilesystemExport.ValueString(),
		S3EndpointURL:    plan.S3EndpointURL.ValueString(),
		S3Bucket:         plan.S3Bucket.ValueString(),
		SecretRef:        plan.SecretRef.ValueString(),
		IsDefault:        plan.IsDefault.ValueBool(),
		Immutable:        plan.Immutable.ValueBool(),
	}

	bt, err := r.client.UpdateBackupTarget(ctx, plan.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Update backup_target failed", err.Error())
		return
	}

	btToModel(bt, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *BackupTargetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state backupTargetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DeleteBackupTarget(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Delete backup_target failed", err.Error())
	}
}

func (r *BackupTargetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	bt, err := r.client.GetBackupTarget(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Import backup_target failed", err.Error())
		return
	}
	if bt == nil {
		resp.Diagnostics.AddError("Import failed", fmt.Sprintf("backup_target %q not found", req.ID))
		return
	}
	var state backupTargetModel
	btToModel(bt, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// btToModel maps a WLM API response onto the Terraform state model.
// The live API does not echo `name` back in GET responses, so we preserve
// whatever `m.Name` already holds (set from plan on Create, from state on Read).
// Optional string fields use null when the API returns "" to avoid plan inconsistency.
func btToModel(bt *wlm.BackupTarget, m *backupTargetModel) {
	m.ID = types.StringValue(bt.ID)
	// Only overwrite name when WLM actually returns a real display name (from
	// name/display_name/metadata "name"); otherwise keep the existing model
	// value. ResolvedDisplayName deliberately ignores the export/bucket
	// fallbacks ResolvedName uses — adopting the export path as the `name`
	// here would corrupt the user's config and break plan consistency.
	if n := bt.ResolvedDisplayName(); n != "" {
		m.Name = types.StringValue(n)
	}
	m.Type = types.StringValue(bt.Type)
	// WLM returns the export string in `filesystem_export` for BOTH nfs and s3
	// targets and does NOT echo back s3_endpoint_url / s3_bucket. For an s3
	// target, mapping that field onto filesystem_export (and nulling the s3
	// attributes) would clobber the configured values and break plan
	// consistency after apply. So for s3 we preserve the configured
	// endpoint/bucket and leave filesystem_export untouched; only nfs targets
	// read filesystem_export from the API.
	if bt.Type == "s3" {
		if bt.S3EndpointURL != "" {
			m.S3EndpointURL = types.StringValue(bt.S3EndpointURL)
		}
		if bt.S3Bucket != "" {
			m.S3Bucket = types.StringValue(bt.S3Bucket)
		}
	} else {
		m.FilesystemExport = strOrNull(bt.FilesystemExport)
		m.S3EndpointURL = strOrNull(bt.S3EndpointURL)
		m.S3Bucket = strOrNull(bt.S3Bucket)
	}
	// secret_ref: keep the configured value unless WLM echoes one back.
	if bt.SecretRef != "" {
		m.SecretRef = types.StringValue(bt.SecretRef)
	}
	m.IsDefault = types.BoolValue(bt.IsDefault)
	m.Immutable = types.BoolValue(bt.IsImmutable())
}

func strOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

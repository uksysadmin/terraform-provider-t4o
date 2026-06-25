package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

var _ datasource.DataSource = &BackupTargetsDataSource{}

func NewBackupTargetsDataSource() datasource.DataSource { return &BackupTargetsDataSource{} }

type BackupTargetsDataSource struct{ client *wlm.Client }

type backupTargetItem struct {
	ID               types.String `tfsdk:"id"`
	Name             types.String `tfsdk:"name"`
	Type             types.String `tfsdk:"type"`
	FilesystemExport types.String `tfsdk:"filesystem_export"`
	IsDefault        types.Bool   `tfsdk:"is_default"`
	Status           types.String `tfsdk:"status"`
}

type backupTargetsDataModel struct {
	BackupTargets []backupTargetItem `tfsdk:"backup_targets"`
}

func (d *BackupTargetsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup_targets"
}

func (d *BackupTargetsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists existing TrilioVault backup targets in the project.",
		Attributes: map[string]schema.Attribute{
			"backup_targets": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of backup targets.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":                schema.StringAttribute{Computed: true},
						"name":              schema.StringAttribute{Computed: true},
						"type":              schema.StringAttribute{Computed: true},
						"filesystem_export": schema.StringAttribute{Computed: true},
						"is_default":        schema.BoolAttribute{Computed: true},
						"status":            schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *BackupTargetsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*wlm.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *wlm.Client, got %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *BackupTargetsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	targets, err := d.client.ListBackupTargets(ctx)
	if err != nil {
		resp.Diagnostics.AddError("List backup_targets failed", err.Error())
		return
	}

	var state backupTargetsDataModel
	for _, bt := range targets {
		state.BackupTargets = append(state.BackupTargets, backupTargetItem{
			ID:               types.StringValue(bt.ID),
			Name:             types.StringValue(bt.ResolvedName()),
			Type:             types.StringValue(bt.Type),
			FilesystemExport: types.StringValue(bt.FilesystemExport),
			IsDefault:        types.BoolValue(bt.IsDefault),
			Status:           types.StringValue(bt.Status),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

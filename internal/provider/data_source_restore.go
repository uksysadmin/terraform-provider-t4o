package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

var _ datasource.DataSource = &RestoreDataSource{}

func NewRestoreDataSource() datasource.DataSource { return &RestoreDataSource{} }

type RestoreDataSource struct{ client *wlm.Client }

type restoredInstanceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

type restoreDataSourceModel struct {
	ID                types.String                     `tfsdk:"id"`
	RestoreID         types.String                     `tfsdk:"restore_id"`
	Name              types.String                     `tfsdk:"name"`
	SnapshotID        types.String                     `tfsdk:"snapshot_id"`
	Status            types.String                     `tfsdk:"status"`
	RestoredInstances map[string]restoredInstanceModel `tfsdk:"restored_instances"`
}

func (d *RestoreDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_restore_details"
}

func (d *RestoreDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves details about a TrilioVault restore operation, crucially including the mapped IDs of the restored instances.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"restore_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "UUID of the restore operation.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name of the restore.",
			},
			"snapshot_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "UUID of the snapshot that was restored.",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Current status of the restore.",
			},
			"restored_instances": schema.MapNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Map of restored instances. The map key is the VM's original name, and the value contains the newly created OpenStack instance ID. Used to seamlessly adopt restored VMs into `openstack_compute_instance_v2` resources via `import` blocks.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The new OpenStack UUID of the restored instance.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The name of the restored instance.",
						},
					},
				},
			},
		},
	}
}

func (d *RestoreDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RestoreDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state restoreDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	res, err := d.client.GetRestore(ctx, state.RestoreID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read restore failed", err.Error())
		return
	}
	if res == nil {
		resp.Diagnostics.AddError("Restore not found", fmt.Sprintf("restore %s not found", state.RestoreID.ValueString()))
		return
	}

	state.ID = types.StringValue(res.ID)
	state.Name = types.StringValue(res.Name)
	state.SnapshotID = types.StringValue(res.SnapshotID)
	state.Status = types.StringValue(res.Status)

	instancesMap := make(map[string]restoredInstanceModel)
	for _, inst := range res.Instances {
		name, nameOk := inst["name"].(string)
		id, idOk := inst["id"].(string)
		if nameOk && idOk {
			instancesMap[name] = restoredInstanceModel{
				ID:   types.StringValue(id),
				Name: types.StringValue(name),
			}
		}
	}
	state.RestoredInstances = instancesMap

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

var _ datasource.DataSource = &WorkloadsDataSource{}

func NewWorkloadsDataSource() datasource.DataSource { return &WorkloadsDataSource{} }

type WorkloadsDataSource struct{ client *wlm.Client }

type workloadItem struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	WorkloadTypeID types.String `tfsdk:"workload_type_id"`
	Status         types.String `tfsdk:"status"`
}

type workloadsDataModel struct {
	Workloads []workloadItem `tfsdk:"workloads"`
}

func (d *WorkloadsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workloads"
}

func (d *WorkloadsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists existing TrilioVault workloads in the project.",
		Attributes: map[string]schema.Attribute{
			"workloads": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of workloads.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":               schema.StringAttribute{Computed: true},
						"name":             schema.StringAttribute{Computed: true},
						"description":      schema.StringAttribute{Computed: true},
						"workload_type_id": schema.StringAttribute{Computed: true},
						"status":           schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *WorkloadsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WorkloadsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	workloads, err := d.client.ListWorkloads(ctx)
	if err != nil {
		resp.Diagnostics.AddError("List workloads failed", err.Error())
		return
	}

	var state workloadsDataModel
	for _, wl := range workloads {
		state.Workloads = append(state.Workloads, workloadItem{
			ID:             types.StringValue(wl.ID),
			Name:           types.StringValue(wl.Name),
			Description:    types.StringValue(wl.Description),
			WorkloadTypeID: types.StringValue(wl.WorkloadTypeID),
			Status:         types.StringValue(wl.Status),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

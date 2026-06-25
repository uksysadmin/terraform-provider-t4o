package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

var _ datasource.DataSource = &WorkloadTypesDataSource{}

func NewWorkloadTypesDataSource() datasource.DataSource { return &WorkloadTypesDataSource{} }

type WorkloadTypesDataSource struct{ client *wlm.Client }

type workloadTypeItem struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	IsPublic    types.Bool   `tfsdk:"is_public"`
}

type workloadTypesModel struct {
	WorkloadTypes []workloadTypeItem `tfsdk:"workload_types"`
}

func (d *WorkloadTypesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workload_types"
}

func (d *WorkloadTypesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists available TrilioVault workload types (Parallel, Serial).",
		Attributes: map[string]schema.Attribute{
			"workload_types": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of workload types.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Computed: true, MarkdownDescription: "Workload type UUID."},
						"name":        schema.StringAttribute{Computed: true, MarkdownDescription: "Type name (e.g. Parallel, Serial)."},
						"description": schema.StringAttribute{Computed: true},
						"is_public":   schema.BoolAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *WorkloadTypesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *WorkloadTypesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	wts, err := d.client.ListWorkloadTypes(ctx)
	if err != nil {
		resp.Diagnostics.AddError("List workload_types failed", err.Error())
		return
	}

	var state workloadTypesModel
	for _, wt := range wts {
		state.WorkloadTypes = append(state.WorkloadTypes, workloadTypeItem{
			ID:          types.StringValue(wt.ID),
			Name:        types.StringValue(wt.Name),
			Description: types.StringValue(wt.Description),
			IsPublic:    types.BoolValue(wt.IsPublic),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

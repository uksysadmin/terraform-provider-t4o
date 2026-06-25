package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

var _ datasource.DataSource = &QuotaTypesDataSource{}

func NewQuotaTypesDataSource() datasource.DataSource { return &QuotaTypesDataSource{} }

type QuotaTypesDataSource struct{ client *wlm.Client }

type quotaTypeItem struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Status      types.String `tfsdk:"status"`
}

type quotaTypesDataModel struct {
	QuotaTypes []quotaTypeItem `tfsdk:"quota_types"`
}

func (d *QuotaTypesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quota_types"
}

func (d *QuotaTypesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the available T4O backup quota types. Use a `quota_types[*].id` " +
			"as the `quota_type_id` for `t4o_project_quota`.",
		Attributes: map[string]schema.Attribute{
			"quota_types": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of quota types.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Computed: true, MarkdownDescription: "Quota-type UUID (use as `quota_type_id`)."},
						"name":        schema.StringAttribute{Computed: true, MarkdownDescription: "Display name."},
						"description": schema.StringAttribute{Computed: true},
						"status":      schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *QuotaTypesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *QuotaTypesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	qts, err := d.client.ListQuotaTypes(ctx)
	if err != nil {
		resp.Diagnostics.AddError("List quota_types failed", err.Error())
		return
	}

	var state quotaTypesDataModel
	for _, qt := range qts {
		state.QuotaTypes = append(state.QuotaTypes, quotaTypeItem{
			ID:          types.StringValue(qt.ID),
			Name:        types.StringValue(qt.DisplayName),
			Description: types.StringValue(qt.DisplayDescription),
			Status:      types.StringValue(qt.Status),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

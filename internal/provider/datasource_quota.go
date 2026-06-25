package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

var _ datasource.DataSource = &QuotaDataSource{}

func NewQuotaDataSource() datasource.DataSource { return &QuotaDataSource{} }

type QuotaDataSource struct{ client *wlm.Client }

type quotaModel struct {
	Raw types.String `tfsdk:"raw"`
}

func (d *QuotaDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_quota"
}

func (d *QuotaDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads TrilioVault quota information. The exact API path is TBD (see API_NOTES.md); probes multiple candidates.",
		Attributes: map[string]schema.Attribute{
			"raw": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Raw JSON response from the quota endpoint.",
			},
		},
	}
}

func (d *QuotaDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *QuotaDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	quota, err := d.client.GetQuota(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Get quota failed", err.Error())
		return
	}

	raw := fmt.Sprintf("%v", quota)
	resp.Diagnostics.Append(resp.State.Set(ctx, &quotaModel{Raw: types.StringValue(raw)})...)
}

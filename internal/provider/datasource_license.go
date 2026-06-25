package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

var _ datasource.DataSource = &LicenseDataSource{}

func NewLicenseDataSource() datasource.DataSource { return &LicenseDataSource{} }

type LicenseDataSource struct{ client *wlm.Client }

type licenseModel struct {
	Raw types.String `tfsdk:"raw"`
}

func (d *LicenseDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_license"
}

func (d *LicenseDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads TrilioVault license information. The exact API path is TBD (see API_NOTES.md); probes multiple candidates.",
		Attributes: map[string]schema.Attribute{
			"raw": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Raw JSON response from the license endpoint.",
			},
		},
	}
}

func (d *LicenseDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *LicenseDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	lic, err := d.client.GetLicense(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Get license failed", err.Error())
		return
	}

	raw := fmt.Sprintf("%v", lic)
	resp.Diagnostics.Append(resp.State.Set(ctx, &licenseModel{Raw: types.StringValue(raw)})...)
}

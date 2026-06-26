package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

var _ provider.Provider = &TrilioVaultProvider{}

// TrilioVaultProvider is the root Terraform provider implementation.
type TrilioVaultProvider struct {
	version string
}

// New returns a provider factory compatible with providerserver.Serve.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &TrilioVaultProvider{version: version}
	}
}

type providerModel struct {
	AuthURL        types.String `tfsdk:"auth_url"`
	Username       types.String `tfsdk:"username"`
	Password       types.String `tfsdk:"password"`
	ProjectID      types.String `tfsdk:"project_id"`
	ProjectName    types.String `tfsdk:"project_name"`
	DomainName     types.String `tfsdk:"domain_name"`
	DomainID       types.String `tfsdk:"domain_id"`
	WLMEndpoint    types.String `tfsdk:"wlm_endpoint"`
	WLMServiceType types.String `tfsdk:"wlm_service_type"`
	Insecure       types.Bool   `tfsdk:"insecure"`
}

func (p *TrilioVaultProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "t4o"
	resp.Version = p.version
}

func (p *TrilioVaultProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Terraform provider for TrilioVault for OpenStack (T4O). Manages backup targets, workloads, and policies via the WLM API.",
		Attributes: map[string]schema.Attribute{
			"auth_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "OpenStack Keystone endpoint. Falls back to `OS_AUTH_URL` environment variable.",
			},
			"username": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "OpenStack username. Falls back to `OS_USERNAME`.",
			},
			"password": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "OpenStack password. Falls back to `OS_PASSWORD`.",
			},
			"project_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "OpenStack project (tenant) UUID. Falls back to `OS_PROJECT_ID` or `OS_TENANT_ID`.",
			},
			"project_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "OpenStack project (tenant) name. Falls back to `OS_PROJECT_NAME` or `OS_TENANT_NAME`.",
			},
			"domain_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "OpenStack domain name. Falls back to `OS_USER_DOMAIN_NAME`, `OS_PROJECT_DOMAIN_NAME`, or `OS_DOMAIN_NAME`. Defaults to `Default`.",
			},
			"domain_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "OpenStack domain ID. Falls back to `OS_USER_DOMAIN_ID`, `OS_PROJECT_DOMAIN_ID`, or `OS_DOMAIN_ID`.",
			},
			"wlm_endpoint": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Override WLM base URL (e.g. `http://10.0.0.10:8781/v1`). Normally auto-discovered from Keystone catalog.",
			},
			"wlm_service_type": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Keystone catalog service type for WLM endpoint discovery. Defaults to `workloads` (Kolla); use `workloadmgr` for DevStack.",
			},
			"insecure": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Skip TLS certificate verification. For dev/lab environments only.",
			},
		},
	}
}

func (p *TrilioVaultProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	authURL := envOrVal(cfg.AuthURL, "OS_AUTH_URL")
	username := envOrVal(cfg.Username, "OS_USERNAME")
	password := envOrVal(cfg.Password, "OS_PASSWORD")
	projectID := envOrVal(cfg.ProjectID, "OS_PROJECT_ID", "OS_TENANT_ID")
	projectName := envOrVal(cfg.ProjectName, "OS_PROJECT_NAME", "OS_TENANT_NAME")
	domainName := envOrVal(cfg.DomainName, "OS_USER_DOMAIN_NAME", "OS_PROJECT_DOMAIN_NAME", "OS_DOMAIN_NAME")
	domainID := envOrVal(cfg.DomainID, "OS_USER_DOMAIN_ID", "OS_PROJECT_DOMAIN_ID", "OS_DOMAIN_ID")
	if domainName == "" && domainID == "" {
		domainName = "Default"
	}

	wlmCfg := wlm.Config{
		AuthURL:        authURL,
		Username:       username,
		Password:       password,
		ProjectID:      projectID,
		ProjectName:    projectName,
		DomainName:     domainName,
		DomainID:       domainID,
		WLMEndpoint:    cfg.WLMEndpoint.ValueString(),
		WLMServiceType: cfg.WLMServiceType.ValueString(),
		Insecure:       cfg.Insecure.ValueBool(),
	}

	client, err := wlm.NewClient(ctx, wlmCfg)
	if err != nil {
		resp.Diagnostics.AddError("WLM client initialization failed", err.Error())
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *TrilioVaultProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewBackupTargetResource,
		NewWorkloadResource,
		NewWorkloadPolicyResource,
		NewProjectQuotaResource,
		NewSettingResource,
	}
}

func (p *TrilioVaultProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewWorkloadTypesDataSource,
		NewWorkloadsDataSource,
		NewBackupTargetsDataSource,
		NewLicenseDataSource,
		NewQuotaDataSource,
		NewQuotaTypesDataSource,
	}
}

// envOrVal returns the string value from attr if set, else the first non-empty env var.
func envOrVal(attr types.String, envVars ...string) string {
	if !attr.IsNull() && !attr.IsUnknown() && attr.ValueString() != "" {
		return attr.ValueString()
	}
	for _, env := range envVars {
		if v := os.Getenv(env); v != "" {
			return v
		}
	}
	return ""
}

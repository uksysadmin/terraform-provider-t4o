package wlm

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud/v2"
	"github.com/gophercloud/gophercloud/v2/openstack"
)

// Config holds provider-level authentication and endpoint configuration.
type Config struct {
	AuthURL        string
	Username       string
	Password       string
	ProjectID      string
	ProjectName    string
	DomainName     string
	DomainID       string
	WLMEndpoint    string // manual override: base URL up to /v1 (project_id appended)
	WLMServiceType string // override catalog service type; default "workloads"
	Insecure       bool
}

// Client is a thin HTTP client for the T4O WLM API.
type Client struct {
	// baseURL = http://VIP:8781/v1/{project_id}  (project_id already embedded)
	baseURL    string
	projectID  string
	token      string
	httpClient *http.Client
}

// NewClient authenticates with Keystone, discovers the WLM endpoint from the
// service catalog, and returns a ready-to-use Client.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.ProjectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	opts := gophercloud.AuthOptions{
		IdentityEndpoint: cfg.AuthURL,
		Username:         cfg.Username,
		Password:         cfg.Password,
		TenantID:         cfg.ProjectID,
		TenantName:       cfg.ProjectName,
		DomainName:       cfg.DomainName,
		DomainID:         cfg.DomainID,
	}

	transport := &http.Transport{}
	if cfg.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	// WLM creates are synchronous and slow: backup_target creation triggers DMS NFS
	// mount validation, and policy/target create+delete on a loaded WLM have been
	// observed to take ~110–130s. A flat 120s ceiling sits right on that boundary and
	// causes the client to abort requests that actually succeed server-side, orphaning
	// the resource and drifting Terraform state. Use a generous 600s ceiling so long
	// synchronous operations complete while still bounding a truly-hung request.
	httpClient := &http.Client{Transport: transport, Timeout: 600 * time.Second}

	providerClient, err := openstack.AuthenticatedClient(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("keystone auth failed: %w", err)
	}

	var baseURL string

	if cfg.WLMEndpoint != "" {
		// Manual override: user supplies base up to /v1; we append the project ID.
		base := strings.TrimRight(cfg.WLMEndpoint, "/")
		baseURL = base + "/" + cfg.ProjectID
	} else {
		svcType := cfg.WLMServiceType
		if svcType == "" {
			svcType = "workloads"
		}

		eo := gophercloud.EndpointOpts{
			Type:         svcType,
			Availability: gophercloud.AvailabilityPublic,
		}
		ep, err := providerClient.EndpointLocator(eo)
		if err != nil && svcType == "workloads" {
			// Fallback for DevStack / older T4O which registers as "workloadmgr".
			eo.Type = "workloadmgr"
			ep, err = providerClient.EndpointLocator(eo)
		}
		if err != nil {
			return nil, fmt.Errorf("WLM endpoint not found in Keystone catalog (tried %q and workloadmgr): %w", svcType, err)
		}

		// The catalog stores the URL with a $(tenant_id)s shell-style placeholder
		// (e.g. http://VIP:8781/v1/$(tenant_id)s). Replace it with the real project ID.
		ep = strings.ReplaceAll(ep, "$(tenant_id)s", cfg.ProjectID)
		ep = strings.ReplaceAll(ep, "%(tenant_id)s", cfg.ProjectID)
		ep = strings.ReplaceAll(ep, "%(project_id)s", cfg.ProjectID)
		baseURL = strings.TrimRight(ep, "/")
	}

	return &Client{
		baseURL:    baseURL,
		projectID:  cfg.ProjectID,
		token:      providerClient.TokenID,
		httpClient: httpClient,
	}, nil
}

// ProjectID returns the project ID this client is scoped to.
func (c *Client) ProjectID() string { return c.projectID }

// BaseURL returns the resolved WLM base URL (for diagnostics).
func (c *Client) BaseURL() string { return c.baseURL }

// -----------------------------------------------------------------------
// Low-level HTTP helpers
// -----------------------------------------------------------------------

func (c *Client) do(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("X-Auth-Token", c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

func readBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func checkStatus(resp *http.Response, body []byte, wantStatus int) error {
	if resp.StatusCode == wantStatus {
		return nil
	}
	return fmt.Errorf("WLM API returned %d (expected %d): %s", resp.StatusCode, wantStatus, string(body))
}

func is404(resp *http.Response) bool { return resp.StatusCode == http.StatusNotFound }

// -----------------------------------------------------------------------
// Backup Targets
// -----------------------------------------------------------------------

func (c *Client) CreateBackupTarget(ctx context.Context, req BackupTargetRequest) (*BackupTarget, error) {
	resp, err := c.do(ctx, http.MethodPost, "/backup_targets", createBackupTargetBody{BackupTarget: req})
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("WLM API returned %d: %s", resp.StatusCode, string(b))
	}
	var out backupTargetResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode backup_target: %w", err)
	}
	return &out.BackupTarget, nil
}

func (c *Client) GetBackupTarget(ctx context.Context, id string) (*BackupTarget, error) {
	resp, err := c.do(ctx, http.MethodGet, "/backup_targets/"+id, nil)
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if is404(resp) {
		return nil, nil
	}
	if err := checkStatus(resp, b, http.StatusOK); err != nil {
		return nil, err
	}
	var out backupTargetResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode backup_target: %w", err)
	}
	return &out.BackupTarget, nil
}

func (c *Client) UpdateBackupTarget(ctx context.Context, id string, req BackupTargetRequest) (*BackupTarget, error) {
	resp, err := c.do(ctx, http.MethodPut, "/backup_targets/"+id, createBackupTargetBody{BackupTarget: req})
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if err := checkStatus(resp, b, http.StatusOK); err != nil {
		return nil, err
	}
	var out backupTargetResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode backup_target: %w", err)
	}
	return &out.BackupTarget, nil
}

func (c *Client) DeleteBackupTarget(ctx context.Context, id string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/backup_targets/"+id, nil)
	if err != nil {
		return err
	}
	b, _ := readBody(resp)
	if is404(resp) {
		return nil // already gone
	}
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusAccepted {
		return nil
	}
	return checkStatus(resp, b, http.StatusOK)
}

func (c *Client) ListBackupTargets(ctx context.Context) ([]BackupTarget, error) {
	resp, err := c.do(ctx, http.MethodGet, "/backup_targets", nil)
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if err := checkStatus(resp, b, http.StatusOK); err != nil {
		return nil, err
	}
	var out backupTargetsResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode backup_targets: %w", err)
	}
	return out.BackupTargets, nil
}

// -----------------------------------------------------------------------
// Workload Types
// -----------------------------------------------------------------------

func (c *Client) ListWorkloadTypes(ctx context.Context) ([]WorkloadType, error) {
	resp, err := c.do(ctx, http.MethodGet, "/workload_types", nil)
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if err := checkStatus(resp, b, http.StatusOK); err != nil {
		return nil, err
	}
	var out workloadTypesResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode workload_types: %w", err)
	}
	return out.WorkloadTypes, nil
}

// -----------------------------------------------------------------------
// Workloads
// -----------------------------------------------------------------------

func (c *Client) CreateWorkload(ctx context.Context, req WorkloadRequest) (*Workload, error) {
	resp, err := c.do(ctx, http.MethodPost, "/workloads", createWorkloadBody{Workload: req})
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	// WLM accepts workload creation asynchronously and returns 202 Accepted with
	// the new workload body (status "creating"). Treat it as success like 200/201.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("WLM API returned %d: %s", resp.StatusCode, string(b))
	}
	var out workloadResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode workload: %w", err)
	}
	return &out.Workload, nil
}

func (c *Client) GetWorkload(ctx context.Context, id string) (*Workload, error) {
	resp, err := c.do(ctx, http.MethodGet, "/workloads/"+id, nil)
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if is404(resp) {
		return nil, nil
	}
	if err := checkStatus(resp, b, http.StatusOK); err != nil {
		return nil, err
	}
	var out workloadResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode workload: %w", err)
	}
	return &out.Workload, nil
}

func (c *Client) UpdateWorkload(ctx context.Context, id string, req WorkloadRequest) (*Workload, error) {
	resp, err := c.do(ctx, http.MethodPut, "/workloads/"+id, createWorkloadBody{Workload: req})
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	// WLM applies workload modifications asynchronously and returns 202 Accepted
	// (often with an empty body), like create. Treat 200/201/202 as success.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("WLM API returned %d: %s", resp.StatusCode, string(b))
	}
	var out workloadResponse
	if err := json.Unmarshal(b, &out); err == nil && out.Workload.ID != "" {
		return &out.Workload, nil
	}
	// 202 with no/empty workload body: re-read the workload to return its
	// current representation rather than failing on the empty response.
	wl, err := c.GetWorkload(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("re-read workload after update: %w", err)
	}
	if wl == nil {
		return nil, fmt.Errorf("workload %s not found after update", id)
	}
	return wl, nil
}

func (c *Client) DeleteWorkload(ctx context.Context, id string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/workloads/"+id, nil)
	if err != nil {
		return err
	}
	b, _ := readBody(resp)
	if is404(resp) {
		return nil
	}
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusAccepted {
		return nil
	}
	return checkStatus(resp, b, http.StatusOK)
}

func (c *Client) ListWorkloads(ctx context.Context) ([]Workload, error) {
	resp, err := c.do(ctx, http.MethodGet, "/workloads", nil)
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if err := checkStatus(resp, b, http.StatusOK); err != nil {
		return nil, err
	}
	var out workloadsResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode workloads: %w", err)
	}
	return out.Workloads, nil
}

// PollWorkload polls until the workload reaches a terminal state or timeout.
// Terminal states: "available", "error". Returns error on "error" state.
func (c *Client) PollWorkload(ctx context.Context, id string, timeout time.Duration) (*Workload, error) {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for workload %s to become available", id)
		}

		wl, err := c.GetWorkload(ctx, id)
		if err != nil {
			return nil, err
		}
		if wl == nil {
			return nil, fmt.Errorf("workload %s not found while polling", id)
		}

		switch wl.Status {
		case "available":
			return wl, nil
		case "error":
			return nil, fmt.Errorf("workload %s entered error state", id)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

// -----------------------------------------------------------------------
// Workload Policies
// -----------------------------------------------------------------------

func (c *Client) CreateWorkloadPolicy(ctx context.Context, req WorkloadPolicyRequest) (*WorkloadPolicy, error) {
	resp, err := c.do(ctx, http.MethodPost, "/workload_policy/", createWorkloadPolicyBody{WorkloadPolicy: req})
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("WLM API returned %d: %s", resp.StatusCode, string(b))
	}
	var out workloadPolicyResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode workload_policy: %w", err)
	}
	if len(req.AssignedProjects) > 0 {
		if err := c.AssignWorkloadPolicy(ctx, out.Policy.ID, req.AssignedProjects); err != nil {
			return nil, fmt.Errorf("assign workload_policy: %w", err)
		}
	}
	return &out.Policy, nil
}

func (c *Client) GetWorkloadPolicy(ctx context.Context, id string) (*WorkloadPolicy, error) {
	resp, err := c.do(ctx, http.MethodGet, "/workload_policy/"+id, nil)
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if is404(resp) {
		return nil, nil
	}
	if err := checkStatus(resp, b, http.StatusOK); err != nil {
		return nil, err
	}
	var out workloadPolicyResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode workload_policy: %w", err)
	}
	return &out.Policy, nil
}

func (c *Client) UpdateWorkloadPolicy(ctx context.Context, id string, req WorkloadPolicyRequest) (*WorkloadPolicy, error) {
	// WLM validates the update body under "policy" (create uses "workload_policy").
	resp, err := c.do(ctx, http.MethodPut, "/workload_policy/"+id, updateWorkloadPolicyBody{Policy: req})
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if err := checkStatus(resp, b, http.StatusOK); err != nil {
		return nil, err
	}
	var out workloadPolicyResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode workload_policy: %w", err)
	}
	if len(req.AssignedProjects) > 0 {
		if err := c.AssignWorkloadPolicy(ctx, id, req.AssignedProjects); err != nil {
			return nil, err
		}
	}
	return &out.Policy, nil
}

func (c *Client) DeleteWorkloadPolicy(ctx context.Context, id string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/workload_policy/"+id, nil)
	if err != nil {
		return err
	}
	b, _ := readBody(resp)
	if is404(resp) {
		return nil
	}
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusAccepted {
		return nil
	}
	return checkStatus(resp, b, http.StatusOK)
}

func (c *Client) AssignWorkloadPolicy(ctx context.Context, id string, projectIDs []string) error {
	resp, err := c.do(ctx, http.MethodPost, "/workload_policy/"+id+"/assign",
		assignPolicyBody{Policy: assignPolicyProjects{AddProjects: projectIDs, RemoveProjects: []string{}}})
	if err != nil {
		return err
	}
	b, _ := readBody(resp)
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return checkStatus(resp, b, http.StatusOK)
}

// -----------------------------------------------------------------------
// License + Quota (paths TBD — probe candidates; 404 → return empty map)
// -----------------------------------------------------------------------

// GetLicense probes candidate paths.
func (c *Client) GetLicense(ctx context.Context) (map[string]interface{}, error) {
	// WLM 6.2 exposes the license under /workloads/metrics/license (confirmed 200).
	// /workloads/metrics alone is 400 ("Invalid uuid for id: metrics"); the older
	// /license_check and /license paths 404 on this build.
	for _, path := range []string{"/workloads/metrics/license", "/license_check", "/license"} {
		resp, err := c.do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		b, _ := readBody(resp)
		if is404(resp) || resp.StatusCode != http.StatusOK {
			continue
		}
		var out map[string]interface{}
		if err := json.Unmarshal(b, &out); err != nil {
			return nil, fmt.Errorf("decode license: %w", err)
		}
		return out, nil
	}
	return map[string]interface{}{"note": "license endpoint not found; see API_NOTES.md"}, nil
}

// GetQuota probes candidate paths. Phase 0 showed /quota → 404.
func (c *Client) GetQuota(ctx context.Context) (map[string]interface{}, error) {
	for _, path := range []string{"/project_quota_types", "/quotas", "/project_allowed_quotas", "/quota"} {
		resp, err := c.do(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		b, _ := readBody(resp)
		if is404(resp) || resp.StatusCode != http.StatusOK {
			continue
		}
		var out map[string]interface{}
		if err := json.Unmarshal(b, &out); err != nil {
			return nil, fmt.Errorf("decode quota: %w", err)
		}
		return out, nil
	}
	return map[string]interface{}{"note": "quota endpoint not found; see API_NOTES.md"}, nil
}

// -----------------------------------------------------------------------
// Project quota (allowed_quotas)
// Request shapes from workloadmanager-client; response wrapper parsed defensively
// (parseAllowedQuota) pending a live confirmation.
// -----------------------------------------------------------------------

// -----------------------------------------------------------------------
// Settings (per-project key/value, name-keyed)
// -----------------------------------------------------------------------

func (c *Client) CreateSetting(ctx context.Context, s SettingItem) (*SettingItem, error) {
	resp, err := c.do(ctx, http.MethodPost, "/settings", settingsBody{Settings: []SettingItem{s}})
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("WLM API returned %d: %s", resp.StatusCode, string(b))
	}
	return c.GetSetting(ctx, s.Name) // create response is a list; re-read the single by name
}

func (c *Client) UpdateSetting(ctx context.Context, s SettingItem) (*SettingItem, error) {
	resp, err := c.do(ctx, http.MethodPut, "/settings", settingsBody{Settings: []SettingItem{s}})
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("WLM API returned %d: %s", resp.StatusCode, string(b))
	}
	return c.GetSetting(ctx, s.Name)
}

func (c *Client) GetSetting(ctx context.Context, name string) (*SettingItem, error) {
	resp, err := c.do(ctx, http.MethodGet, "/settings/"+name, nil)
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if is404(resp) {
		return nil, nil
	}
	if err := checkStatus(resp, b, http.StatusOK); err != nil {
		return nil, err
	}
	// show returns {"setting": {name,value,type}} — or {"setting":{"email":...}} for user_email_address_*
	var wrap struct {
		Setting map[string]interface{} `json:"setting"`
	}
	if err := json.Unmarshal(b, &wrap); err != nil || wrap.Setting == nil {
		return nil, fmt.Errorf("decode setting: %w (%s)", err, string(b))
	}
	str := func(k string) string {
		if v, ok := wrap.Setting[k]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return ""
	}
	out := &SettingItem{Name: name, Value: str("value"), Type: str("type")}
	if out.Value == "" { // email special case carries the value under "email"
		out.Value = str("email")
	}
	return out, nil
}

func (c *Client) DeleteSetting(ctx context.Context, name string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/settings/"+name, nil)
	if err != nil {
		return err
	}
	b, _ := readBody(resp)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("WLM API returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

// ListQuotaTypes returns the available backup quota types (their IDs feed t4o_project_quota).
func (c *Client) ListQuotaTypes(ctx context.Context) ([]QuotaType, error) {
	resp, err := c.do(ctx, http.MethodGet, "/project_quota_types", nil)
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if err := checkStatus(resp, b, http.StatusOK); err != nil {
		return nil, err
	}
	var out quotaTypesResponse
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("decode quota_types: %w", err)
	}
	return out.QuotaTypes, nil
}

func parseAllowedQuota(b []byte) (*AllowedQuota, error) {
	var single struct {
		Q *AllowedQuota `json:"allowed_quota"`
	}
	if json.Unmarshal(b, &single) == nil && single.Q != nil && single.Q.ID != "" {
		return single.Q, nil
	}
	var list struct {
		Q []AllowedQuota `json:"allowed_quotas"`
	}
	if json.Unmarshal(b, &list) == nil && len(list.Q) > 0 {
		return &list.Q[0], nil
	}
	var obj struct {
		Q *AllowedQuota `json:"allowed_quotas"`
	}
	if json.Unmarshal(b, &obj) == nil && obj.Q != nil && obj.Q.ID != "" {
		return obj.Q, nil
	}
	var bare AllowedQuota
	if json.Unmarshal(b, &bare) == nil && bare.ID != "" {
		return &bare, nil
	}
	return nil, fmt.Errorf("could not parse allowed_quota from response: %s", string(b))
}

func (c *Client) CreateAllowedQuota(ctx context.Context, req AllowedQuotaRequest) (*AllowedQuota, error) {
	body := map[string]interface{}{"allowed_quotas": []AllowedQuotaRequest{req}}
	resp, err := c.do(ctx, http.MethodPost, "/project_allowed_quotas/"+req.ProjectID, body)
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("WLM API returned %d: %s", resp.StatusCode, string(b))
	}
	return parseAllowedQuota(b)
}

func (c *Client) GetAllowedQuota(ctx context.Context, id string) (*AllowedQuota, error) {
	resp, err := c.do(ctx, http.MethodGet, "/project_allowed_quota/"+id, nil)
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if is404(resp) {
		return nil, nil
	}
	if err := checkStatus(resp, b, http.StatusOK); err != nil {
		return nil, err
	}
	return parseAllowedQuota(b)
}

func (c *Client) UpdateAllowedQuota(ctx context.Context, id string, req AllowedQuotaRequest) (*AllowedQuota, error) {
	body := map[string]interface{}{"allowed_quotas": map[string]interface{}{
		"project_id":     req.ProjectID,
		"allowed_value":  req.AllowedValue,
		"high_watermark": req.HighWatermark,
	}}
	resp, err := c.do(ctx, http.MethodPut, "/update_allowed_quota/"+id, body)
	if err != nil {
		return nil, err
	}
	b, _ := readBody(resp)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("WLM API returned %d: %s", resp.StatusCode, string(b))
	}
	if q, perr := parseAllowedQuota(b); perr == nil {
		return q, nil
	}
	return c.GetAllowedQuota(ctx, id) // update may return an empty body
}

func (c *Client) DeleteAllowedQuota(ctx context.Context, id string) error {
	resp, err := c.do(ctx, http.MethodDelete, "/project_allowed_quotas/"+id, nil)
	if err != nil {
		return err
	}
	b, _ := readBody(resp)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("WLM API returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

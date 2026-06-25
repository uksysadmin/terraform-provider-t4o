package wlm

// BackupTargetMetadataItem is a single key-value entry from backup_target_metadata.
// The T4O 6.2 API stores immutable as {"key":"immutable","value":"0"|"1"} here.
type BackupTargetMetadataItem struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// BackupTargetTypeItem is an entry of the backup_target_types array the live API
// returns. On WLM 6.2 this is where the only human-readable identifier lives:
// `name` carries the NFS export (or S3 bucket) — the top-level name/display_name
// are absent. Used as a name fallback in ResolvedName.
type BackupTargetTypeItem struct {
	Name string `json:"name"`
}

// BackupTarget is the T4O backup target as returned by the live API (GET response).
// Live API notes (see API_NOTES.md):
//   - `name` is NOT returned; preserve from state.
//   - `immutable` lives in BackupTargetMeta, not as a top-level boolean.
type BackupTarget struct {
	ID               string                     `json:"id"`
	Name             string                     `json:"name"`         // absent in GET on some builds; preserve from Terraform state
	DisplayName      string                     `json:"display_name"` // some WLM builds return the name here instead
	Type             string                     `json:"type"`
	FilesystemExport string                     `json:"filesystem_export,omitempty"`
	S3EndpointURL    string                     `json:"s3_endpoint_url,omitempty"`
	S3Bucket         string                     `json:"s3_bucket,omitempty"`
	SecretRef        string                     `json:"secret_ref,omitempty"`
	IsDefault        bool                       `json:"is_default"`
	Status           string                     `json:"status,omitempty"`
	BackupTargetMeta []BackupTargetMetadataItem `json:"backup_target_metadata,omitempty"`
	BackupTargetType []BackupTargetTypeItem     `json:"backup_target_types,omitempty"`
	CreatedAt        string                     `json:"created_at,omitempty"`
	UpdatedAt        string                     `json:"updated_at,omitempty"`
}

// ResolvedName returns the backup target's name, tolerating WLM builds that
// omit `name` and instead expose it as `display_name` or a
// backup_target_metadata entry keyed "name". Returns "" only if none is set.
func (bt *BackupTarget) ResolvedName() string {
	if bt.Name != "" {
		return bt.Name
	}
	if bt.DisplayName != "" {
		return bt.DisplayName
	}
	for _, m := range bt.BackupTargetMeta {
		if m.Key == "name" && m.Value != "" {
			return m.Value
		}
	}
	// WLM 6.2 omits name/display_name and carries the only human-readable
	// identifier in backup_target_types[].name (the NFS export or S3 bucket).
	for _, t := range bt.BackupTargetType {
		if t.Name != "" {
			return t.Name
		}
	}
	// Last resort: the export / bucket themselves are stable identifiers.
	if bt.FilesystemExport != "" {
		return bt.FilesystemExport
	}
	if bt.S3Bucket != "" {
		return bt.S3Bucket
	}
	return ""
}

// ResolvedDisplayName returns the backup target's human-assigned name, drawing
// ONLY from the fields WLM actually uses to carry it: top-level name /
// display_name, or a backup_target_metadata entry keyed "name". Unlike
// ResolvedName it deliberately does NOT fall back to backup_target_types[].name,
// filesystem_export, or s3_bucket — those carry the export/bucket, not the
// display name. Using them to populate the resource's `name` attribute would
// overwrite the user's configured value with the export string and break plan
// consistency after apply. Returns "" when WLM does not echo a real name, in
// which case the caller preserves the value already in plan/state.
func (bt *BackupTarget) ResolvedDisplayName() string {
	if bt.Name != "" {
		return bt.Name
	}
	if bt.DisplayName != "" {
		return bt.DisplayName
	}
	for _, m := range bt.BackupTargetMeta {
		if m.Key == "name" && m.Value != "" {
			return m.Value
		}
	}
	return ""
}

// IsImmutable reads the immutable flag from BackupTargetMeta.
func (bt *BackupTarget) IsImmutable() bool {
	for _, m := range bt.BackupTargetMeta {
		if m.Key == "immutable" {
			return m.Value == "1"
		}
	}
	return false
}

type BackupTargetRequest struct {
	Name             string            `json:"name"`
	Type             string            `json:"type"`
	FilesystemExport string            `json:"filesystem_export,omitempty"`
	S3EndpointURL    string            `json:"s3_endpoint_url,omitempty"`
	S3Bucket         string            `json:"s3_bucket,omitempty"`
	SecretRef        string            `json:"secret_ref,omitempty"`
	IsDefault        bool              `json:"is_default,omitempty"`
	Immutable        bool              `json:"immutable,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

type createBackupTargetBody struct {
	BackupTarget BackupTargetRequest `json:"backup_target"`
}

// WLM returns the singular object under "backup_targets" (plural) for POST/GET/PUT — not "backup_target".
// The list endpoint (GET /backup_targets) also returns "backup_targets" but as an array; that uses backupTargetsResponse.
type backupTargetResponse struct {
	BackupTarget BackupTarget `json:"backup_targets"`
}

type backupTargetsResponse struct {
	BackupTargets []BackupTarget `json:"backup_targets"`
}

// WorkloadType represents a T4O workload type (Parallel/Serial).
type WorkloadType struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsPublic    bool   `json:"is_public"`
}

type workloadTypesResponse struct {
	WorkloadTypes []WorkloadType `json:"workload_types"`
}

// JobSchedule represents a workload backup schedule.
type JobSchedule struct {
	Enabled            bool   `json:"enabled"`
	StartDate          string `json:"start_date,omitempty"`
	EndDate            string `json:"end_date,omitempty"`
	Interval           string `json:"interval,omitempty"`
	FullBackupInterval int    `json:"fullbackup_interval,omitempty"`
	RetentionDays      int    `json:"retention_days,omitempty"`
	SnapshotsToRetain  int    `json:"snapshots_to_retain,omitempty"`
}

// WorkloadInstance references a Nova VM in a workload.
type WorkloadInstance struct {
	// InstanceID is the key WLM expects in the create/update REQUEST ("instance-id").
	InstanceID string `json:"instance-id"`
	// ID is the key WLM uses for the instance in its RESPONSE ("id"). The request and
	// response use different keys, so we carry both; omitempty keeps the request body
	// unchanged (ID is empty when building a request).
	ID      string `json:"id,omitempty"`
	Include bool   `json:"include"`
}

// ResolvedID returns the instance UUID regardless of which key the API populated.
func (w WorkloadInstance) ResolvedID() string {
	if w.InstanceID != "" {
		return w.InstanceID
	}
	return w.ID
}

// Workload represents a T4O workload (protection plan).
type Workload struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Description    string             `json:"description,omitempty"`
	WorkloadTypeID string             `json:"workload_type_id"`
	Instances      []WorkloadInstance `json:"instances,omitempty"`
	BackupTargetID string             `json:"backup_target_id,omitempty"`
	JobSchedule    *JobSchedule       `json:"jobschedule,omitempty"`
	PolicyID       string             `json:"policy_id,omitempty"`
	Status         string             `json:"status,omitempty"`
	CreatedAt      string             `json:"created_at,omitempty"`
	UpdatedAt      string             `json:"updated_at,omitempty"`
	Metadata       map[string]string  `json:"metadata,omitempty"`
}

type WorkloadRequest struct {
	Name           string             `json:"name"`
	Description    string             `json:"description,omitempty"`
	WorkloadTypeID string             `json:"workload_type_id"`
	Instances      []WorkloadInstance `json:"instances"`
	BackupTargetID string             `json:"backup_target_id,omitempty"`
	// BackupTargetTypes is what WLM 6.2 actually reads to bind a workload to a target
	// (it ignores backup_target_id). Accepts a backup_target_type NAME or UUID; the
	// provider resolves it from the chosen backup_target's backup_target_types[].name.
	BackupTargetTypes string       `json:"backup_target_types,omitempty"`
	Encryption        bool         `json:"encryption,omitempty"`  // encrypt this workload's backups
	SecretUUID        string       `json:"secret_uuid,omitempty"` // Barbican secret UUID (encryption passphrase)
	JobSchedule       *JobSchedule `json:"jobschedule,omitempty"`
	PolicyID       string             `json:"policy_id,omitempty"`
	Metadata       map[string]string  `json:"metadata,omitempty"`
}

type createWorkloadBody struct {
	Workload WorkloadRequest `json:"workload"`
}

type workloadResponse struct {
	Workload Workload `json:"workload"`
}

type workloadsResponse struct {
	Workloads []Workload `json:"workloads"`
}

// WorkloadPolicy is the policy as returned by WLM. The view builder maps the
// DB's display_name/display_description back to plain name/description and wraps
// the object under "policy" (see workloadPolicyResponse). field_values comes back
// in a serialized/listy shape that we deliberately do NOT round-trip into
// Terraform state — the schedule attributes are treated as write-only.
type WorkloadPolicy struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Status      string      `json:"status,omitempty"`
	FieldValues interface{} `json:"field_values,omitempty"`
	// WLM returns metadata as an array ([]) when empty, not an object — so decode
	// it loosely. The provider does not map policy metadata into state anyway.
	Metadata  interface{} `json:"metadata,omitempty"`
	CreatedAt string      `json:"created_at,omitempty"`
}

// WorkloadPolicyRequest is the WLM 6.2 advanced-scheduler create/update payload.
// WLM reads display_name / display_description / email / field_values / metadata.
// field_values MUST contain exactly the seeded policy fields:
//
//	hourly, daily, weekly, monthly, yearly, manual, retentionmanual, start_time
//
// (hourly.interval is mandatory and must be one of 1,2,3,4,6,8,12,24).
// assigned_projects is NOT part of this body — assignment is a separate /assign
// call — so it is excluded from JSON via json:"-".
type WorkloadPolicyRequest struct {
	DisplayName        string                 `json:"display_name"`
	DisplayDescription string                 `json:"display_description,omitempty"`
	Email              string                 `json:"email,omitempty"`
	FieldValues        map[string]interface{} `json:"field_values"`
	Metadata           map[string]string      `json:"metadata,omitempty"`
	AssignedProjects   []string               `json:"-"` // for the separate /assign call only
}

// createWorkloadPolicyBody wraps the create payload — WLM validates body["workload_policy"].
type createWorkloadPolicyBody struct {
	WorkloadPolicy WorkloadPolicyRequest `json:"workload_policy"`
}

// updateWorkloadPolicyBody wraps the update payload — WLM validates body["policy"].
type updateWorkloadPolicyBody struct {
	Policy WorkloadPolicyRequest `json:"policy"`
}

// workloadPolicyResponse — the view builder wraps the result under "policy".
type workloadPolicyResponse struct {
	Policy WorkloadPolicy `json:"policy"`
}

type workloadPoliciesResponse struct {
	WorkloadPolicies []WorkloadPolicy `json:"policies"`
}

// assignPolicyBody — controller reads body["policy"]["add_projects"|"remove_projects"].
type assignPolicyBody struct {
	Policy assignPolicyProjects `json:"policy"`
}

type assignPolicyProjects struct {
	AddProjects    []string `json:"add_projects"`
	RemoveProjects []string `json:"remove_projects"`
}

// License — path TBD; /license returned 404 in Phase 0 (see API_NOTES.md).
type License struct {
	ExpiryDate string                 `json:"expiry_date,omitempty"`
	VMCount    int                    `json:"vm_count,omitempty"`
	IsValid    bool                   `json:"is_valid"`
	Raw        map[string]interface{} `json:"-"`
}

// Quota — path TBD; /quota returned 404 in Phase 0 (see API_NOTES.md).
type Quota struct {
	Raw map[string]interface{}
}

// AllowedQuota — a per-project backup quota assignment (WLM "allowed_quotas").
// Request shapes verified against workloadmanager-client; response shape is parsed
// defensively (parseAllowedQuota) pending a live confirmation of the wrapper key.
type AllowedQuota struct {
	ID            string `json:"id"`
	QuotaTypeID   string `json:"quota_type_id"`
	ProjectID     string `json:"project_id"`
	AllowedValue  int64  `json:"allowed_value"`
	HighWatermark int64  `json:"high_watermark"`
}

type AllowedQuotaRequest struct {
	QuotaTypeID   string `json:"quota_type_id,omitempty"`
	ProjectID     string `json:"project_id"`
	AllowedValue  int64  `json:"allowed_value"`
	HighWatermark int64  `json:"high_watermark"`
}

// QuotaType — an available T4O backup quota type (project_quota_types). Its ID is the
// quota_type_id used by t4o_project_quota.
type QuotaType struct {
	ID                 string `json:"id"`
	DisplayName        string `json:"display_name"`
	DisplayDescription string `json:"display_description"`
	Status             string `json:"status"`
}

type quotaTypesResponse struct {
	QuotaTypes []QuotaType `json:"quota_types"`
}

// Setting — a per-project T4O setting (key/value). Settings are keyed by NAME (not UUID).
// Create/update take a list under "settings"; show returns a single "setting" object.
type SettingItem struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
}

type settingsBody struct {
	Settings []SettingItem `json:"settings"`
}

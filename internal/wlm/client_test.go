package wlm_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

const (
	testProjectID = "test-project-id-1234"
	testToken     = "test-token-xyz"
)

// newTestClient creates a WLM client pointed at a mock server using WLMEndpoint override.
// Auth URL uses the /v3/ suffix so Gophercloud v2 skips version discovery and POSTs
// directly to /v3/auth/tokens — no version discovery round-trip needed.
func newTestClient(t *testing.T, wlmHandler http.HandlerFunc) *wlm.Client {
	t.Helper()

	keystoneMux := http.NewServeMux()
	keystoneSrv := httptest.NewServer(keystoneMux)
	t.Cleanup(keystoneSrv.Close)

	keystoneMux.HandleFunc("/v3/auth/tokens", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Subject-Token", testToken)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"token": map[string]interface{}{
				"project": map[string]interface{}{"id": testProjectID},
				"catalog": []interface{}{},
				"user":    map[string]interface{}{"id": "user-1", "name": "admin"},
			},
		})
	})

	wlmSrv := httptest.NewServer(wlmHandler)
	t.Cleanup(wlmSrv.Close)

	cfg := wlm.Config{
		// /v3/ suffix tells Gophercloud v2 to skip version discovery.
		AuthURL:     keystoneSrv.URL + "/v3/",
		Username:    "admin",
		Password:    "test",
		ProjectID:   testProjectID,
		DomainName:  "Default",
		WLMEndpoint: wlmSrv.URL + "/v1",
	}
	client, err := wlm.NewClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func TestListWorkloadTypes(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/"+testProjectID+"/workload_types" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"workload_types": []map[string]interface{}{
				{"id": "2ddd528d-c9b4-4d7e-8722-cc395140255a", "name": "Parallel", "is_public": true},
				{"id": "f82ce76f-17fe-438b-aa37-7a023058e50d", "name": "Serial", "is_public": true},
			},
		})
	}))

	wts, err := client.ListWorkloadTypes(context.Background())
	if err != nil {
		t.Fatalf("ListWorkloadTypes: %v", err)
	}
	if len(wts) != 2 {
		t.Errorf("expected 2 types, got %d", len(wts))
	}
	if wts[0].Name != "Parallel" {
		t.Errorf("expected Parallel, got %s", wts[0].Name)
	}
}

func TestCreateBackupTarget(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/"+testProjectID+"/backup_targets" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// Live API returns "backup_targets" (plural) even for a single object — see API_NOTES.md.
		json.NewEncoder(w).Encode(map[string]interface{}{
			"backup_targets": map[string]interface{}{
				"id":                "bt-uuid-1",
				"name":              "test-nfs",
				"type":              "nfs",
				"filesystem_export": "192.168.1.1:/exports/tvault",
				"is_default":        true,
			},
		})
	}))

	bt, err := client.CreateBackupTarget(context.Background(), wlm.BackupTargetRequest{
		Name:             "test-nfs",
		Type:             "nfs",
		FilesystemExport: "192.168.1.1:/exports/tvault",
		IsDefault:        true,
	})
	if err != nil {
		t.Fatalf("CreateBackupTarget: %v", err)
	}
	if bt.ID != "bt-uuid-1" {
		t.Errorf("expected ID bt-uuid-1, got %s", bt.ID)
	}
}

// TestCreateWorkload_202Accepted guards against a regression where the client
// rejected WLM's asynchronous 202 Accepted response (with a valid workload body,
// status "creating") as an error. 202 must be treated as success like 200/201.
func TestCreateWorkload_202Accepted(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/"+testProjectID+"/workloads" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"workload": map[string]interface{}{
				"id":               "wl-uuid-202",
				"name":             "async-workload",
				"status":           "creating",
				"workload_type_id": "2ddd528d-c9b4-4d7e-8722-cc395140255a",
			},
		})
	}))

	wl, err := client.CreateWorkload(context.Background(), wlm.WorkloadRequest{
		Name:           "async-workload",
		WorkloadTypeID: "2ddd528d-c9b4-4d7e-8722-cc395140255a",
	})
	if err != nil {
		t.Fatalf("CreateWorkload on 202 should not error: %v", err)
	}
	if wl.ID != "wl-uuid-202" {
		t.Errorf("expected ID wl-uuid-202, got %s", wl.ID)
	}
}

// TestWorkloadInstance_ResolvedID guards the request/response key asymmetry: WLM
// expects "instance-id" in the create request but returns instances under "id".
func TestWorkloadInstance_ResolvedID(t *testing.T) {
	reqShaped := wlm.WorkloadInstance{InstanceID: "vm-from-request"}
	if got := reqShaped.ResolvedID(); got != "vm-from-request" {
		t.Errorf("expected vm-from-request, got %s", got)
	}
	var respShaped wlm.WorkloadInstance
	if err := json.Unmarshal([]byte(`{"id":"vm-from-response"}`), &respShaped); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := respShaped.ResolvedID(); got != "vm-from-response" {
		t.Errorf("expected vm-from-response, got %s", got)
	}
}

func TestGetBackupTarget_404ReturnsNil(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))

	bt, err := client.GetBackupTarget(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("GetBackupTarget 404 should not error: %v", err)
	}
	if bt != nil {
		t.Errorf("expected nil on 404, got %+v", bt)
	}
}

func TestListWorkloads_Empty(t *testing.T) {
	client := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/"+testProjectID+"/workloads" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"workloads": []interface{}{}})
	}))

	wls, err := client.ListWorkloads(context.Background())
	if err != nil {
		t.Fatalf("ListWorkloads: %v", err)
	}
	if len(wls) != 0 {
		t.Errorf("expected 0 workloads, got %d", len(wls))
	}
}

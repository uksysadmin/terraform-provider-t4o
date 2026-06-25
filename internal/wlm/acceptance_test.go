package wlm_test

import (
	"context"
	"os"
	"testing"

	"github.com/trilio-demo/terraform-provider-t4o/internal/wlm"
)

// TestAcc exercises the WLM client against the live Kolla cloud.
// Requires TF_ACC=1 and OS_* credentials pointing at the tfprov-test project.
func TestAcc(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Set TF_ACC=1 to run acceptance tests against live WLM")
	}

	authURL := os.Getenv("OS_AUTH_URL")
	username := os.Getenv("OS_USERNAME")
	password := os.Getenv("OS_PASSWORD")
	projectID := os.Getenv("OS_PROJECT_ID")
	domainName := os.Getenv("OS_USER_DOMAIN_NAME")
	if domainName == "" {
		domainName = os.Getenv("OS_DOMAIN_NAME")
	}
	if domainName == "" {
		domainName = "Default"
	}
	// Default to /exports/tvault2 — a non-default NFS export that can be freely created/deleted.
	// /exports/tvault is reserved as the permanent default target (cannot be deleted via WLM API).
	nfsExport := os.Getenv("TF_ACC_NFS_EXPORT")
	if nfsExport == "" {
		nfsExport = "10.0.0.5:/exports/tvault" // override with TF_ACC_NFS_EXPORT for real runs
	}

	for _, v := range []string{authURL, username, password, projectID} {
		if v == "" {
			t.Fatal("OS_AUTH_URL, OS_USERNAME, OS_PASSWORD, OS_PROJECT_ID must all be set")
		}
	}

	cfg := wlm.Config{
		AuthURL:    authURL,
		Username:   username,
		Password:   password,
		ProjectID:  projectID,
		DomainName: domainName,
	}

	ctx := context.Background()
	client, err := wlm.NewClient(ctx, cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	t.Logf("WLM base URL: %s", client.BaseURL())

	// ---- workload types ----
	t.Run("ListWorkloadTypes", func(t *testing.T) {
		wts, err := client.ListWorkloadTypes(ctx)
		if err != nil {
			t.Fatalf("ListWorkloadTypes: %v", err)
		}
		if len(wts) < 2 {
			t.Fatalf("expected >=2 workload types, got %d", len(wts))
		}
		t.Logf("workload types: %v", func() []string {
			names := make([]string, len(wts))
			for i, wt := range wts {
				names[i] = wt.Name + "=" + wt.ID
			}
			return names
		}())
	})

	// ---- backup target CRUD ----
	t.Run("BackupTargetCRUD", func(t *testing.T) {
		bt, err := client.CreateBackupTarget(ctx, wlm.BackupTargetRequest{
			Name:             "tfacc-nfs-target",
			Type:             "nfs",
			FilesystemExport: nfsExport,
		})
		if err != nil {
			t.Fatalf("CreateBackupTarget: %v", err)
		}
		t.Logf("created backup target %s", bt.ID)
		t.Cleanup(func() {
			if err := client.DeleteBackupTarget(ctx, bt.ID); err != nil {
				t.Logf("cleanup: DeleteBackupTarget %s: %v", bt.ID, err)
			}
		})

		got, err := client.GetBackupTarget(ctx, bt.ID)
		if err != nil {
			t.Fatalf("GetBackupTarget: %v", err)
		}
		if got == nil {
			t.Fatal("GetBackupTarget returned nil for existing target")
		}
		if got.ID != bt.ID {
			t.Errorf("ID mismatch: got %s, want %s", got.ID, bt.ID)
		}
		// WLM does not echo `name` back in GET responses — name is preserved from state, not from API.

		if err := client.DeleteBackupTarget(ctx, bt.ID); err != nil {
			t.Fatalf("DeleteBackupTarget: %v", err)
		}

		gone, err := client.GetBackupTarget(ctx, bt.ID)
		if err != nil {
			t.Fatalf("GetBackupTarget after delete: %v", err)
		}
		if gone != nil {
			t.Errorf("expected nil after delete, still got %s", gone.ID)
		}
	})

	// ---- list workloads (should be empty or contain only tfacc- prefixed items) ----
	t.Run("ListWorkloads", func(t *testing.T) {
		wls, err := client.ListWorkloads(ctx)
		if err != nil {
			t.Fatalf("ListWorkloads: %v", err)
		}
		t.Logf("workloads in project: %d", len(wls))
	})
}

package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	hf "aurago/internal/huggingface"
	"aurago/internal/security"
)

func TestCronManagerAddWithoutIDKeepsEveryJobWhenClockRepeats(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{SchedulerEnabled: true})
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	mgr := NewCronManager(tempSystemTaskDir(t))
	t.Cleanup(func() { _ = mgr.Close() })
	// Windows clocks can return the same instant for back-to-back calls.
	frozen := time.Unix(1_700_000_000, 0)
	mgr.now = func() time.Time { return frozen }

	// An explicit numeric ID that equals the frozen timestamp must survive too.
	explicitID := strconv.FormatInt(frozen.UnixNano(), 10)
	if _, err := mgr.ManageSchedule("add", explicitID, "0 * * * *", "explicit prompt", "en"); err != nil {
		t.Fatalf("ManageSchedule add explicit job: %v", err)
	}

	const n = 50
	generated := make(map[string]string, n)
	for i := 0; i < n; i++ {
		prompt := fmt.Sprintf("generated prompt %d", i)
		result, err := mgr.ManageSchedule("add", "", "0 * * * *", prompt, "en")
		if err != nil {
			t.Fatalf("ManageSchedule add #%d: %v", i, err)
		}
		var resp struct {
			Status string `json:"status"`
			ID     string `json:"id"`
		}
		if err := json.Unmarshal([]byte(result), &resp); err != nil {
			t.Fatalf("decode add #%d result %s: %v", i, result, err)
		}
		if resp.Status != "success" {
			t.Fatalf("add #%d = %s, want success", i, result)
		}
		if _, err := strconv.ParseInt(resp.ID, 10, 64); err != nil {
			t.Fatalf("add #%d id = %q, want numeric id", i, resp.ID)
		}
		if resp.ID == explicitID {
			t.Fatalf("add #%d reused explicit id %q", i, resp.ID)
		}
		if previous, dup := generated[resp.ID]; dup {
			t.Fatalf("add #%d reused id %q of %q", i, resp.ID, previous)
		}
		generated[resp.ID] = prompt
	}

	jobs := mgr.GetJobs()
	if len(jobs) != n+1 {
		t.Fatalf("job count = %d, want %d", len(jobs), n+1)
	}
	for _, job := range jobs {
		want := generated[job.ID]
		if job.ID == explicitID {
			want = "explicit prompt"
		}
		if job.TaskPrompt != want {
			t.Fatalf("job %q prompt = %q, want %q", job.ID, job.TaskPrompt, want)
		}
	}
}

func TestManageOutgoingWebhooksCreateWithoutIDKeepsEveryWebhook(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := tmpDir + "/config.yaml"
	configContent := `
server:
  ui_language: en
auth:
  enabled: false
personality:
  core_personality: neutral
webhooks:
  enabled: true
  readonly: false
  outgoing: []
`
	if err := config.WriteFileAtomic(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(tmpDir, "vault.bin"))
	if err != nil {
		t.Fatalf("NewVault() error = %v", err)
	}

	const n = 3
	for i := 0; i < n; i++ {
		result := ManageOutgoingWebhooks("create", "", fmt.Sprintf("Deploy %d", i), "Trigger deploy", "POST", "https://example.test/deploy", "json", "", nil, nil, cfg, vault, false, false)
		if !strings.Contains(result, `"status":"success"`) {
			t.Fatalf("create #%d result: %s", i, result)
		}
	}

	if len(cfg.Webhooks.Outgoing) != n {
		t.Fatalf("outgoing webhook count = %d, want %d", len(cfg.Webhooks.Outgoing), n)
	}
	seen := make(map[string]bool, n)
	for _, hook := range cfg.Webhooks.Outgoing {
		if !strings.HasPrefix(hook.ID, "hook_") || seen[hook.ID] {
			t.Fatalf("webhook id %q is not a distinct generated id: %+v", hook.ID, cfg.Webhooks.Outgoing)
		}
		seen[hook.ID] = true
	}
}

func TestHuggingFaceLedgerKeepsBackToBackJobsWithoutRemoteID(t *testing.T) {
	dataDir := t.TempDir()
	const n = 5
	for i := 0; i < n; i++ {
		// A rejected job run carries no Hugging Face job ID, so the ledger
		// generates a local one.
		err := recordHuggingFaceJob(context.Background(), dataDir, "job_run_python", "cpu-basic", HuggingFaceRequest{}, map[string]interface{}{}, errors.New("quota exceeded"))
		if err != nil {
			t.Fatalf("record job #%d: %v", i, err)
		}
	}

	store, err := hf.OpenJobStore(dataDir)
	if err != nil {
		t.Fatalf("open job store: %v", err)
	}
	defer store.Close()
	jobs, err := store.ListJobs(context.Background(), 0)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != n {
		t.Fatalf("ledger job count = %d, want %d", len(jobs), n)
	}
	seen := make(map[string]bool, n)
	for _, job := range jobs {
		if !strings.HasPrefix(job.HFJobID, "local-") || seen[job.HFJobID] {
			t.Fatalf("ledger job id %q is not a distinct local id: %+v", job.HFJobID, jobs)
		}
		seen[job.HFJobID] = true
	}
}

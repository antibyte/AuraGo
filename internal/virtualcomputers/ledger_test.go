package virtualcomputers

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLedgerRecordsMachinesActionsAndExposure(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "virtual_computers.db")
	ledger, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	t.Cleanup(func() { _ = ledger.Close() })

	ctx := context.Background()
	if err := ledger.SetSetupState(ctx, "status", "ok"); err != nil {
		t.Fatalf("SetSetupState: %v", err)
	}
	if err := ledger.UpsertMachine(ctx, Machine{ID: "vm-1", Template: "python", Status: "running", TTLSeconds: 600}); err != nil {
		t.Fatalf("UpsertMachine: %v", err)
	}
	if err := ledger.RecordAction(ctx, ActionRecord{Actor: "test", Action: "launch", TargetType: "machine", TargetID: "vm-1"}); err != nil {
		t.Fatalf("RecordAction: %v", err)
	}
	if err := ledger.SetExposure(ctx, ExposureRecord{MachineID: "vm-1", Channel: "web:8080", URL: "/api/virtual-computers/machines/vm-1/web/8080/", Active: true}); err != nil {
		t.Fatalf("SetExposure: %v", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var status string
	if err := db.QueryRow(`SELECT status FROM machines WHERE id = 'vm-1'`).Scan(&status); err != nil {
		t.Fatalf("select machine: %v", err)
	}
	if status != "running" {
		t.Fatalf("machine status = %q, want running", status)
	}

	var actionCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM actions WHERE action = 'launch' AND target_id = 'vm-1'`).Scan(&actionCount); err != nil {
		t.Fatalf("select actions: %v", err)
	}
	if actionCount != 1 {
		t.Fatalf("action count = %d, want 1", actionCount)
	}

	var active int
	if err := db.QueryRow(`SELECT active FROM exposure_status WHERE machine_id = 'vm-1' AND channel = 'web:8080'`).Scan(&active); err != nil {
		t.Fatalf("select exposure: %v", err)
	}
	if active != 1 {
		t.Fatalf("exposure active = %d, want 1", active)
	}
}

func TestLedgerMigratesLegacyVolumeSchemaWithBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "virtual_computers.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE volumes (
		id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '', size_bytes INTEGER NOT NULL DEFAULT 0,
		raw_json TEXT NOT NULL DEFAULT '{}', updated_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	_ = db.Close()

	ledger, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	defer ledger.Close()
	if _, err := os.Stat(path + ".v1.bak"); err != nil {
		t.Fatalf("migration backup: %v", err)
	}
	for _, column := range []string{"created_at", "expires_at", "quota_mb", "last_verified_at", "verification_status"} {
		var count int
		if err := ledger.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('volumes') WHERE name = ?`, column).Scan(&count); err != nil || count != 1 {
			t.Fatalf("column %s count=%d err=%v", column, count, err)
		}
	}
}

func TestOpenLedgerSkipsMigrationWriteForCurrentSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "virtual_computers.db")
	ledger, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	if _, err := ledger.db.Exec(`CREATE TRIGGER reject_schema_version_update
		BEFORE UPDATE ON schema_meta BEGIN SELECT RAISE(ABORT, 'schema version rewrite'); END`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatalf("close ledger: %v", err)
	}

	reopened, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("reopen current ledger: %v", err)
	}
	defer reopened.Close()
}

func TestOpenLedgersWaitForConcurrentWriter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "virtual_computers.db")
	first, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("OpenLedger first: %v", err)
	}
	defer first.Close()
	second, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("OpenLedger second: %v", err)
	}
	defer second.Close()

	tx, err := first.db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	if _, err := tx.Exec(`INSERT INTO machines(id, updated_at) VALUES ('held-writer', '2026-07-16T00:00:00Z')`); err != nil {
		t.Fatalf("hold write lock: %v", err)
	}
	result := make(chan error, 1)
	go func() {
		result <- second.UpsertMachine(context.Background(), Machine{ID: "queued-writer"})
	}()
	select {
	case err := <-result:
		t.Fatalf("second writer returned before lock release: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := <-result; err != nil {
		t.Fatalf("queued writer: %v", err)
	}
}

func TestOpenLedgerSerializesSharedHandleWrites(t *testing.T) {
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "virtual_computers.db"))
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	defer ledger.Close()

	if got := ledger.db.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1", got)
	}
}

func TestListTrackedVolumesVerifiesKnownCapabilities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/volumes/vol-ok":
			_, _ = w.Write([]byte(`{"id":"vol-ok","created_at":"2026-07-14T08:00:00Z","expires_at":"2026-07-15T08:00:00Z","quota_mb":256}`))
		case "/v1/volumes/vol-missing":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		case "/v1/volumes/vol-stale":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"temporary storage error"}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{BaseURL: server.URL, Timeout: time.Second})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "virtual_computers.db"))
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	defer ledger.Close()
	for _, id := range []string{"vol-ok", "vol-missing", "vol-stale"} {
		if err := ledger.UpsertVolume(context.Background(), Volume{ID: id}); err != nil {
			t.Fatalf("UpsertVolume(%s): %v", id, err)
		}
	}
	volumes, err := ListTrackedVolumes(context.Background(), ledger, client)
	if err != nil {
		t.Fatalf("ListTrackedVolumes: %v", err)
	}
	byID := map[string]Volume{}
	for _, volume := range volumes {
		byID[volume.ID] = volume
	}
	if len(byID) != 2 || byID["vol-ok"].VerificationStatus != "verified" || byID["vol-stale"].VerificationStatus != "stale" {
		t.Fatalf("volumes = %+v", volumes)
	}
	if _, ok := byID["vol-missing"]; ok {
		t.Fatalf("missing volume remained tracked: %+v", volumes)
	}
}

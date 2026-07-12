package virtualcomputers

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
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

package networkshares

import (
	"context"
	"path/filepath"
	"testing"
)

func TestLedgerMigrationAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.db")
	ledger, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("OpenLedger: %v", err)
	}
	spec := ShareSpec{
		ID: "11111111-1111-4111-8111-111111111111", Protocol: ProtocolNFS,
		Name: "archive", Path: filepath.Join(t.TempDir(), "archive"), ReadOnly: true,
		Access: ShareAccess{Clients: []string{"192.0.2.0/24"}},
	}
	if err := ledger.put(context.Background(), spec, "missing"); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := ledger.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := OpenLedger(path)
	if err != nil {
		t.Fatalf("reopen migrated ledger: %v", err)
	}
	defer reopened.Close()
	record, err := reopened.get(context.Background(), spec.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if record.Spec.Name != spec.Name || record.Drift != "missing" {
		t.Fatalf("record = %+v", record)
	}
}

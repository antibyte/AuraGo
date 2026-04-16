package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/sqlconnections"
)

func newSQLDispatchTestContext(t *testing.T) (*DispatchContext, func()) {
	t.Helper()

	tempDir := t.TempDir()
	metaDB, err := sqlconnections.InitDB(filepath.Join(tempDir, "sqlconnections.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}

	vault, err := security.NewVault(strings.Repeat("a", 64), filepath.Join(tempDir, "vault.bin"))
	if err != nil {
		metaDB.Close()
		t.Fatalf("NewVault() error = %v", err)
	}

	pool := sqlconnections.NewConnectionPool(metaDB, vault, 2, 1, nil)
	cfg := &config.Config{}
	cfg.SQLConnections.Enabled = true
	cfg.SQLConnections.AllowManagement = true

	dc := &DispatchContext{
		Cfg:               cfg,
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		Vault:             vault,
		SQLConnectionsDB:  metaDB,
		SQLConnectionPool: pool,
	}

	cleanup := func() {
		pool.CloseAll()
		_ = metaDB.Close()
	}

	return dc, cleanup
}

func decodeToolOutputMap(t *testing.T, out string) map[string]interface{} {
	t.Helper()

	raw := strings.TrimPrefix(out, "Tool Output: ")
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("failed to decode tool output %q: %v", out, err)
	}
	return decoded
}

func TestDispatchServicesManageSQLConnectionsCreateUsesServiceDefaults(t *testing.T) {
	dc, cleanup := newSQLDispatchTestContext(t)
	defer cleanup()

	dataPath := filepath.Join(t.TempDir(), "agent.db")
	out, ok := dispatchServices(context.Background(), ToolCall{
		Action:         "manage_sql_connections",
		Operation:      "create",
		ConnectionName: "analytics",
		Driver:         "sqlite",
		DatabaseName:   dataPath,
		Description:    "warehouse",
	}, dc)
	if !ok {
		t.Fatal("expected dispatchServices to handle manage_sql_connections")
	}

	decoded := decodeToolOutputMap(t, out)
	if decoded["status"] != "success" {
		t.Fatalf("unexpected output: %v", decoded)
	}

	stored, err := newSQLConnectionServiceForDispatch(dc).GetByName("analytics")
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}
	if !stored.AllowRead || stored.AllowWrite || stored.AllowChange || stored.AllowDelete {
		t.Fatalf("unexpected permissions: %+v", stored)
	}
}

func TestDispatchServicesManageSQLConnectionsUpdateDeletesCredentials(t *testing.T) {
	dc, cleanup := newSQLDispatchTestContext(t)
	defer cleanup()

	service := newSQLConnectionServiceForDispatch(dc)
	_, err := service.Create(sqlconnections.CreateRequest{
		Name:         "analytics",
		Driver:       "sqlite",
		DatabaseName: filepath.Join(t.TempDir(), "analytics.db"),
		Description:  "warehouse",
		Username:     "reporter",
		Password:     "secret",
		AllowRead:    true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	stored, err := service.GetByName("analytics")
	if err != nil {
		t.Fatalf("GetByName() error = %v", err)
	}
	if stored.VaultSecretID == "" {
		t.Fatal("expected vault secret id to be present")
	}
	if _, err := dc.Vault.ReadSecret(stored.VaultSecretID); err != nil {
		t.Fatalf("ReadSecret() error = %v", err)
	}

	out, ok := dispatchServices(context.Background(), ToolCall{
		Action:           "manage_sql_connections",
		Operation:        "update",
		ConnectionName:   "analytics",
		CredentialAction: "delete",
	}, dc)
	if !ok {
		t.Fatal("expected dispatchServices to handle manage_sql_connections update")
	}

	decoded := decodeToolOutputMap(t, out)
	if decoded["status"] != "success" {
		t.Fatalf("unexpected output: %v", decoded)
	}

	updated, err := service.GetByName("analytics")
	if err != nil {
		t.Fatalf("GetByName() after update error = %v", err)
	}
	if updated.VaultSecretID != "" {
		t.Fatalf("VaultSecretID = %q, want empty", updated.VaultSecretID)
	}
	if _, err := dc.Vault.ReadSecret(stored.VaultSecretID); err == nil {
		t.Fatalf("expected old secret %q to be deleted", stored.VaultSecretID)
	}
}

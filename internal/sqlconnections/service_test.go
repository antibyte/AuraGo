package sqlconnections

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockVault implements VaultHandler for testing
type mockVault struct {
	secrets map[string]string
}

func (m *mockVault) ReadSecret(key string) (string, error) {
	if val, ok := m.secrets[key]; ok {
		return val, nil
	}
	return "", nil
}

func (m *mockVault) WriteSecret(key, value string) error {
	if m.secrets == nil {
		m.secrets = make(map[string]string)
	}
	m.secrets[key] = value
	return nil
}

func (m *mockVault) DeleteSecret(key string) error {
	if m.secrets != nil {
		delete(m.secrets, key)
	}
	return nil
}

func TestService_Create(t *testing.T) {
	// Create temp database
	tmpfile, err := os.CreateTemp("", "sql_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	db, err := InitDB(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	vault := &mockVault{}
	pool := &ConnectionPool{} // minimal pool for testing
	logger := slogDefault()

	svc := NewService(ServiceConfig{
		DB:          db,
		Vault:       vault,
		Pool:        pool,
		Logger:      logger,
		ReadOnly:    false,
		AllowManage: true,
	})

	tests := []struct {
		name    string
		req     CreateRequest
		wantErr bool
	}{
		{
			name: "valid postgres connection with credentials",
			req: CreateRequest{
				Name:         "test-pg",
				Driver:       "postgres",
				Host:         "localhost",
				Port:         5432,
				DatabaseName: "mydb",
				Description:  "Test PostgreSQL",
				Username:     "user",
				Password:     "pass",
				SSLMode:      "disable",
				AllowRead:    true,
				AllowWrite:   false,
			},
			wantErr: false,
		},
		{
			name: "valid mysql connection without credentials",
			req: CreateRequest{
				Name:         "test-mysql",
				Driver:       "mysql",
				SSLMode:      "disable",
				Host:         "localhost",
				Port:         3306,
				DatabaseName: "mydb",
			},
			wantErr: false,
		},
		{
			name: "empty name rejected",
			req: CreateRequest{
				Name:   "",
				Driver: "postgres",
			},
			wantErr: true,
		},
		{
			name: "invalid driver rejected",
			req: CreateRequest{
				Name:   "test",
				Driver: "oracle",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := svc.Create(tt.req)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if res.ID == "" {
				t.Error("expected non-empty ID")
			}
			if res.Name != tt.req.Name {
				t.Errorf("expected name %s, got %s", tt.req.Name, res.Name)
			}

			// Verify credentials were stored in vault
			if tt.req.Username != "" || tt.req.Password != "" {
				conn, _ := GetByID(db, res.ID)
				if conn.VaultSecretID == "" {
					t.Error("expected vault secret ID to be set")
				}
				if vault.secrets[conn.VaultSecretID] == "" {
					t.Error("expected vault secret to be stored")
				}
			} else {
				conn, _ := GetByID(db, res.ID)
				if conn.VaultSecretID != "" {
					t.Errorf("VaultSecretID = %q, want empty for connection without credentials", conn.VaultSecretID)
				}
			}
		})
	}
}

func TestService_CreateSQLiteWithoutCredentialsConnectsWithoutVaultSecret(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vault := &mockVault{}
	pool := NewConnectionPool(db, vault, 3, 5, nil)
	defer pool.CloseAll()
	svc := NewService(ServiceConfig{
		DB:     db,
		Vault:  vault,
		Pool:   pool,
		Logger: slogDefault(),
	})

	sqlitePath := filepath.Join(t.TempDir(), "app.db")
	res, err := svc.Create(CreateRequest{
		Name:         "sqlite-no-creds",
		Driver:       "sqlite",
		DatabaseName: sqlitePath,
		AllowRead:    true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	conn, err := GetByID(db, res.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if conn.VaultSecretID != "" {
		t.Fatalf("VaultSecretID = %q, want empty", conn.VaultSecretID)
	}

	importFixture(t, pool, res.ID)
	opened, err := pool.GetConnection(res.ID)
	if err != nil {
		t.Fatalf("GetConnection() error = %v", err)
	}
	if opened == nil {
		t.Fatal("GetConnection() returned nil db")
	}
}

func TestService_Update_CredentialRotation(t *testing.T) {
	// Create temp database
	tmpfile, err := os.CreateTemp("", "sql_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	db, err := InitDB(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	vault := &mockVault{}
	pool := &ConnectionPool{}
	logger := slogDefault()

	svc := NewService(ServiceConfig{
		DB:          db,
		Vault:       vault,
		Pool:        pool,
		Logger:      logger,
		ReadOnly:    false,
		AllowManage: true,
	})

	// Create initial connection
	res, err := svc.Create(CreateRequest{
		Name:         "test-pg",
		Driver:       "postgres",
		SSLMode:      "disable",
		Host:         "localhost",
		Port:         5432,
		DatabaseName: "mydb",
		Username:     "user",
		Password:     "pass",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Get initial vault secret ID
	conn, _ := GetByID(db, res.ID)
	initialSecretID := conn.VaultSecretID

	// Update credentials (replace)
	err = svc.Update(UpdateRequest{
		ID:               res.ID,
		Name:             "test-pg",
		CredentialAction: "replace",
		Username:         "newuser",
		Password:         "newpass",
	})
	if err != nil {
		t.Errorf("unexpected error during credential replace: %v", err)
	}

	// Verify new credentials stored
	conn, _ = GetByID(db, res.ID)
	if conn.VaultSecretID == initialSecretID {
		t.Error("expected new vault secret ID after rotation")
	}
	if vault.secrets[conn.VaultSecretID] == "" {
		t.Error("expected new vault secret to be stored")
	}

	// Verify old secret is cleaned up
	if vault.secrets[initialSecretID] != "" {
		t.Error("expected old vault secret to be cleaned up")
	}
}

func TestService_Update_CredentialDelete(t *testing.T) {
	// Create temp database
	tmpfile, err := os.CreateTemp("", "sql_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	db, err := InitDB(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	vault := &mockVault{}
	pool := &ConnectionPool{}
	logger := slogDefault()

	svc := NewService(ServiceConfig{
		DB:          db,
		Vault:       vault,
		Pool:        pool,
		Logger:      logger,
		ReadOnly:    false,
		AllowManage: true,
	})

	// Create connection with credentials
	res, err := svc.Create(CreateRequest{
		Name:     "test-pg",
		Driver:   "postgres",
		SSLMode:  "disable",
		Host:     "localhost",
		Username: "user",
		Password: "pass",
	})
	if err != nil {
		t.Fatal(err)
	}

	conn, _ := GetByID(db, res.ID)
	secretID := conn.VaultSecretID

	// Delete credentials
	err = svc.Update(UpdateRequest{
		ID:               res.ID,
		Name:             "test-pg",
		CredentialAction: "delete",
	})
	if err != nil {
		t.Errorf("unexpected error during credential delete: %v", err)
	}

	// Verify vault secret is deleted
	if vault.secrets[secretID] != "" {
		t.Error("expected vault secret to be deleted")
	}
}

func TestService_UpdateReplaceCleansNewSecretWhenMetadataUpdateFails(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vault := &mockVault{}
	svc := NewService(ServiceConfig{
		DB:     db,
		Vault:  vault,
		Logger: slogDefault(),
	})

	first, err := svc.Create(CreateRequest{
		Name:         "first",
		Driver:       "postgres",
		SSLMode:      "disable",
		DatabaseName: "app",
		Username:     "old",
		Password:     "oldpass",
	})
	if err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}
	if _, err := svc.Create(CreateRequest{
		Name:         "second",
		Driver:       "postgres",
		SSLMode:      "disable",
		DatabaseName: "app",
		Username:     "other",
		Password:     "otherpass",
	}); err != nil {
		t.Fatalf("Create(second) error = %v", err)
	}

	before := len(vault.secrets)
	if err := svc.Update(UpdateRequest{
		ID:               first.ID,
		Name:             "second",
		CredentialAction: "replace",
		Username:         "new",
		Password:         "newpass",
	}); err == nil {
		t.Fatal("Update() expected duplicate-name error")
	}
	if len(vault.secrets) != before {
		t.Fatalf("vault secret count = %d, want %d after failed metadata update", len(vault.secrets), before)
	}
}

func TestService_UpdateDeleteKeepsOldSecretWhenMetadataUpdateFails(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	vault := &mockVault{}
	svc := NewService(ServiceConfig{
		DB:     db,
		Vault:  vault,
		Logger: slogDefault(),
	})

	first, err := svc.Create(CreateRequest{
		Name:         "first",
		Driver:       "postgres",
		SSLMode:      "disable",
		DatabaseName: "app",
		Username:     "old",
		Password:     "oldpass",
	})
	if err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}
	firstRec, err := GetByID(db, first.ID)
	if err != nil {
		t.Fatalf("GetByID(first) error = %v", err)
	}
	if _, err := svc.Create(CreateRequest{
		Name:         "second",
		Driver:       "postgres",
		SSLMode:      "disable",
		DatabaseName: "app",
	}); err != nil {
		t.Fatalf("Create(second) error = %v", err)
	}

	if err := svc.Update(UpdateRequest{
		ID:               first.ID,
		Name:             "second",
		CredentialAction: "delete",
	}); err == nil {
		t.Fatal("Update() expected duplicate-name error")
	}
	if vault.secrets[firstRec.VaultSecretID] == "" {
		t.Fatalf("old secret %q was deleted even though metadata update failed", firstRec.VaultSecretID)
	}

	afterRec, err := GetByID(db, first.ID)
	if err != nil {
		t.Fatalf("GetByID(first after failed update) error = %v", err)
	}
	if afterRec.VaultSecretID != firstRec.VaultSecretID {
		t.Fatalf("VaultSecretID = %q, want %q after failed update", afterRec.VaultSecretID, firstRec.VaultSecretID)
	}
}

func TestService_Delete(t *testing.T) {
	// Create temp database
	tmpfile, err := os.CreateTemp("", "sql_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	db, err := InitDB(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	vault := &mockVault{}
	pool := &ConnectionPool{}
	logger := slogDefault()

	svc := NewService(ServiceConfig{
		DB:          db,
		Vault:       vault,
		Pool:        pool,
		Logger:      logger,
		ReadOnly:    false,
		AllowManage: true,
	})

	// Create connection with credentials
	res, err := svc.Create(CreateRequest{
		Name:     "test-pg",
		Driver:   "postgres",
		SSLMode:  "disable",
		Host:     "localhost",
		Username: "user",
		Password: "pass",
	})
	if err != nil {
		t.Fatal(err)
	}

	conn, _ := GetByID(db, res.ID)
	secretID := conn.VaultSecretID

	// Delete connection
	err = svc.Delete(DeleteRequest{ID: res.ID})
	if err != nil {
		t.Errorf("unexpected error during delete: %v", err)
	}

	// Verify connection is gone
	_, err = GetByID(db, res.ID)
	if err == nil {
		t.Error("expected error when getting deleted connection")
	}

	// Verify vault secret is cleaned up
	if vault.secrets[secretID] != "" {
		t.Error("expected vault secret to be cleaned up after delete")
	}
}

func TestService_PolicyFlags(t *testing.T) {
	// Create temp database
	tmpfile, err := os.CreateTemp("", "sql_test_*.db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	db, err := InitDB(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	vault := &mockVault{}
	pool := &ConnectionPool{}
	logger := slogDefault()

	svc := NewService(ServiceConfig{
		DB:          db,
		Vault:       vault,
		Pool:        pool,
		Logger:      logger,
		ReadOnly:    true,
		AllowManage: false,
	})

	if !svc.IsReadOnly() {
		t.Error("expected IsReadOnly to return true")
	}

	if svc.CanManage() {
		t.Error("expected CanManage to return false")
	}

	// Update flags
	svc.SetReadOnly(false)
	svc.SetAllowManage(true)

	if svc.IsReadOnly() {
		t.Error("expected IsReadOnly to return false after update")
	}

	if !svc.CanManage() {
		t.Error("expected CanManage to return true after update")
	}
}

func TestService_CreateRequiresExplicitSSLModeForNetworkDrivers(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	vault := &mockVault{}
	svc := NewService(ServiceConfig{DB: db, Vault: vault, Logger: slogDefault()})

	for _, driver := range []string{"postgres", "mysql"} {
		_, err := svc.Create(CreateRequest{
			Name:         "implicit-" + driver,
			Driver:       driver,
			Host:         "db.example.lan",
			DatabaseName: "app",
			Username:     "reader",
			Password:     "secret",
		})
		if err == nil || !strings.Contains(err.Error(), "ssl_mode is required for "+driver+" connections") {
			t.Fatalf("%s without ssl_mode: err = %v, want ssl_mode is required", driver, err)
		}
		if !errors.Is(err, ErrSSLModeRequired) {
			t.Fatalf("%s without ssl_mode: err = %v, want ErrSSLModeRequired", driver, err)
		}
		if _, err := GetByName(db, "implicit-"+driver); err == nil {
			t.Fatalf("%s without ssl_mode was stored", driver)
		}
	}
	if len(vault.secrets) != 0 {
		t.Fatalf("rejected creates left %d vault secrets behind", len(vault.secrets))
	}

	for _, mode := range []string{"disable", "require", "verify-ca", "verify-full"} {
		res, err := svc.Create(CreateRequest{Name: "pg-" + mode, Driver: "postgres", Host: "db.example.lan", DatabaseName: "app", SSLMode: mode})
		if err != nil {
			t.Fatalf("postgres with ssl_mode %q: %v", mode, err)
		}
		stored, err := GetByID(db, res.ID)
		if err != nil || stored.SSLMode != mode {
			t.Fatalf("postgres ssl_mode stored = %q (%v), want %q", stored.SSLMode, err, mode)
		}
	}
	if _, err := svc.Create(CreateRequest{Name: "mysql-require", Driver: "mysql", Host: "db.example.lan", DatabaseName: "app", SSLMode: "require"}); err != nil {
		t.Fatalf("mysql with ssl_mode require: %v", err)
	}

	res, err := svc.Create(CreateRequest{Name: "local-sqlite", Driver: "sqlite", DatabaseName: "managed-id"})
	if err != nil {
		t.Fatalf("sqlite without ssl_mode: %v", err)
	}
	stored, err := GetByID(db, res.ID)
	if err != nil || stored.SSLMode != "disable" {
		t.Fatalf("sqlite ssl_mode stored = %q (%v), want the previous default disable", stored.SSLMode, err)
	}
}

func TestService_UpdateWithoutSSLModeKeepsStoredMode(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	svc := NewService(ServiceConfig{DB: db, Vault: &mockVault{}, Logger: slogDefault()})
	res, err := svc.Create(CreateRequest{Name: "pg", Driver: "postgres", Host: "db.example.lan", DatabaseName: "app", SSLMode: "verify-full"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Update(UpdateRequest{ID: res.ID, Name: "pg-renamed", AllowRead: true}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	stored, err := GetByID(db, res.ID)
	if err != nil || stored.Name != "pg-renamed" || stored.SSLMode != "verify-full" {
		t.Fatalf("stored after update = %+v (%v), want ssl_mode verify-full", stored, err)
	}

	// A legacy row stored without a TLS mode stays untouched by unrelated edits.
	legacyID, err := Create(db, "legacy", "postgres", "db.example.lan", 5432, "app", "", true, false, false, false, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Update(UpdateRequest{ID: legacyID, Name: "legacy", Description: "edited", AllowRead: true}); err != nil {
		t.Fatalf("Update(legacy) error = %v", err)
	}
	legacy, err := GetByID(db, legacyID)
	if err != nil || legacy.SSLMode != "" || legacy.Description != "edited" {
		t.Fatalf("legacy after update = %+v (%v), want unchanged empty ssl_mode", legacy, err)
	}
}

// slogDefault returns a logger for tests
func slogDefault() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

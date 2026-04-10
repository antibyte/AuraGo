package sqlconnections

import (
	"log/slog"
	"os"
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
			}
		})
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

// slogDefault returns a logger for tests
func slogDefault() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

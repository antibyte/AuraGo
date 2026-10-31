package server

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/sqlconnections"
)

func TestResolveSQLConnectionTestTargetCredentialsFollowTarget(t *testing.T) {
	t.Parallel()

	stored := sqlconnections.ConnectionRecord{ID: "c1", Name: "Main", Driver: "postgres", Host: "db.local", Port: 5432, DatabaseName: "app", SSLMode: "disable", VaultSecretID: "v1"}

	t.Run("unchanged target without edits uses the stored secret as is", func(t *testing.T) {
		target, err := resolveSQLConnectionTestTarget(sqlConnectionRequest{}, &stored, "alice", "s3cret")
		if err != nil || !target.UseStoredVault {
			t.Fatalf("target = %+v, err = %v", target, err)
		}
	})

	t.Run("unchanged target with a new TLS mode keeps the stored password", func(t *testing.T) {
		target, err := resolveSQLConnectionTestTarget(sqlConnectionRequest{SSLMode: "require"}, &stored, "alice", "s3cret")
		if err != nil || target.UseStoredVault || target.Username != "alice" || target.Password != "s3cret" || target.Record.SSLMode != "require" {
			t.Fatalf("target = %+v, err = %v", target, err)
		}
	})

	for name, req := range map[string]sqlConnectionRequest{
		"host":     {Host: "evil.example"},
		"port":     {Port: 6543},
		"database": {DatabaseName: "other"},
		"driver":   {Driver: "mysql"},
		"user":     {Username: "bob"},
	} {
		req := req
		t.Run("changed "+name+" without a typed password is refused", func(t *testing.T) {
			_, err := resolveSQLConnectionTestTarget(req, &stored, "alice", "s3cret")
			if !errors.Is(err, errSQLTestPasswordNeeded) {
				t.Fatalf("err = %v, want errSQLTestPasswordNeeded", err)
			}
		})
	}

	t.Run("changed host with a typed password tests the edited target", func(t *testing.T) {
		target, err := resolveSQLConnectionTestTarget(sqlConnectionRequest{Host: "new.example", Password: "typed"}, &stored, "alice", "s3cret")
		if err != nil || target.Record.Host != "new.example" || target.Password != "typed" || target.Username != "alice" || target.UseStoredVault {
			t.Fatalf("target = %+v, err = %v", target, err)
		}
	})

	t.Run("unsaved record is tested as typed and SQLite needs a save", func(t *testing.T) {
		target, err := resolveSQLConnectionTestTarget(sqlConnectionRequest{Driver: "mysql", Host: "h", DatabaseName: "d", Username: "u", Password: "p"}, nil, "", "")
		if err != nil || target.Username != "u" || target.Password != "p" {
			t.Fatalf("target = %+v, err = %v", target, err)
		}
		if _, err := resolveSQLConnectionTestTarget(sqlConnectionRequest{Driver: "sqlite"}, nil, "", ""); !errors.Is(err, errSQLTestSaveFirst) {
			t.Fatalf("sqlite err = %v, want errSQLTestSaveFirst", err)
		}
		if _, err := resolveSQLConnectionTestTarget(sqlConnectionRequest{Driver: "postgres"}, nil, "", ""); !errors.Is(err, errSQLTestRequiredNetwork) {
			t.Fatalf("missing database err = %v, want errSQLTestRequiredNetwork", err)
		}
	})
}

// The test body is merged over the stored record: the typed port reaches the dialer
// (proved by a listener that sees the connection), not the stored one.
func TestHandleSQLConnectionTestUsesDraftFields(t *testing.T) {
	t.Parallel()

	metaDB, err := sqlconnections.InitDB(filepath.Join(t.TempDir(), "sqlconnections.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer metaDB.Close()
	id, err := sqlconnections.Create(metaDB, "Draft", "postgres", "127.0.0.1", 1, "app", "d", true, false, false, false, "", "disable")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("no loopback listener: %v", err)
	}
	defer listener.Close()
	accepted := make(chan struct{}, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		accepted <- struct{}{}
		conn.Close()
	}()

	s := &Server{
		SQLConnectionsDB:  metaDB,
		SQLConnectionPool: sqlconnections.NewConnectionPool(metaDB, nil, 2, 2, nil),
	}
	port := listener.Addr().(*net.TCPAddr).Port
	body := `{"port":` + strconv.Itoa(port) + `,"password":"typed"}`
	req := httptest.NewRequest(http.MethodPost, "/api/sql-connections/"+id+"/test", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handleSQLConnectionTest(s)(rec, req)

	select {
	case <-accepted:
	case <-time.After(3 * time.Second):
		t.Fatalf("the edited port was not dialled; status = %d body=%s", rec.Code, rec.Body.String())
	}
	// The fake server hangs up, so the test itself reports a (sanitized) failure.
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Connection test failed") {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleSQLConnectionTestRefusesChangedTargetWithoutPassword(t *testing.T) {
	t.Parallel()

	metaDB, err := sqlconnections.InitDB(filepath.Join(t.TempDir(), "sqlconnections.db"))
	if err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	defer metaDB.Close()
	id, err := sqlconnections.Create(metaDB, "Saved", "postgres", "db.local", 5432, "app", "d", true, false, false, false, "", "disable")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	s := &Server{
		SQLConnectionsDB:  metaDB,
		SQLConnectionPool: sqlconnections.NewConnectionPool(metaDB, nil, 2, 1, nil),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/sql-connections/"+id+"/test", strings.NewReader(`{"host":"elsewhere.example"}`))
	rec := httptest.NewRecorder()
	handleSQLConnectionTest(s)(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "enter the password") {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	// A record that is not saved yet may be tested; SQLite needs saving first.
	req = httptest.NewRequest(http.MethodPost, "/api/sql-connections/new/test", strings.NewReader(`{"driver":"sqlite","name":"x"}`))
	rec = httptest.NewRecorder()
	handleSQLConnectionTest(s)(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "save the connection first") {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

// F: a TLS mode that is not stronger than the stored one is a target change.
func TestResolveSQLConnectionTestTargetSSLModeDowngradeNeedsPassword(t *testing.T) {
	t.Parallel()

	base := sqlconnections.ConnectionRecord{ID: "c1", Name: "Main", Driver: "postgres", Host: "db.local", Port: 5432, DatabaseName: "app", VaultSecretID: "v1"}
	for _, tc := range []struct{ stored, drafted string }{
		{"require", "disable"}, {"verify-full", "verify-ca"}, {"verify-ca", "require"}, {"require", "allow"},
		{"prefer", "allow"}, {"disable", "tls-please"},
	} {
		stored := base
		stored.SSLMode = tc.stored
		_, err := resolveSQLConnectionTestTarget(sqlConnectionRequest{SSLMode: tc.drafted}, &stored, "alice", "s3cret")
		if tc.drafted == "tls-please" {
			if !errors.Is(err, errSQLTestInvalidSSLMode) {
				t.Fatalf("unknown mode: err = %v, want errSQLTestInvalidSSLMode", err)
			}
			continue
		}
		if !errors.Is(err, errSQLTestPasswordNeeded) {
			t.Fatalf("%s -> %s: err = %v, want errSQLTestPasswordNeeded", tc.stored, tc.drafted, err)
		}
	}

	// A typed password may test the weaker mode, an upgrade keeps the stored password.
	stored := base
	stored.SSLMode = "require"
	target, err := resolveSQLConnectionTestTarget(sqlConnectionRequest{SSLMode: "disable", Password: "typed"}, &stored, "alice", "s3cret")
	if err != nil || target.Password != "typed" || target.Record.SSLMode != "disable" {
		t.Fatalf("typed password: target = %+v, err = %v", target, err)
	}
	target, err = resolveSQLConnectionTestTarget(sqlConnectionRequest{SSLMode: "verify-full"}, &stored, "alice", "s3cret")
	if err != nil || target.Password != "s3cret" || target.Record.SSLMode != "verify-full" {
		t.Fatalf("upgrade: target = %+v, err = %v", target, err)
	}
	// Unchanged and unset modes keep using the stored secret untouched.
	if target, err = resolveSQLConnectionTestTarget(sqlConnectionRequest{SSLMode: "require"}, &stored, "alice", "s3cret"); err != nil || !target.UseStoredVault {
		t.Fatalf("same mode: target = %+v, err = %v", target, err)
	}
}

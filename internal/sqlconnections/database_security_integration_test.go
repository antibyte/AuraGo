package sqlconnections

import (
	"context"
	"database/sql"
	"net"
	"os"
	"strconv"
	"testing"
	"time"
)

// Only dedicated disposable databases may be supplied to this opt-in test.
func TestExternalDatabaseReadOnlyBoundary(t *testing.T) {
	for _, driver := range []string{"postgres", "mysql"} {
		t.Run(driver, func(t *testing.T) {
			key := "AURAGO_SQL_TEST_POSTGRES_ADDR"
			user := "postgres"
			if driver == "mysql" {
				key = "AURAGO_SQL_TEST_MYSQL_ADDR"
				user = "root"
			}
			addr := os.Getenv(key)
			if addr == "" {
				t.Skip("dedicated database not configured")
			}
			host, portText, err := net.SplitHostPort(addr)
			if err != nil {
				t.Fatal(err)
			}
			port, _ := strconv.Atoi(portText)
			rec := ConnectionRecord{Name: "external", Driver: driver, Host: host, Port: port, DatabaseName: "audit", SSLMode: "disable", AllowRead: true}
			dsn, dialect, err := BuildDSN(rec, user, "audit-fixture-only", 5*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			db, err := sql.Open(dialect, dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if _, err = db.ExecContext(ctx, "CREATE TABLE audit_boundary_accounts(id INTEGER)"); err != nil {
				t.Fatal(err)
			}
			defer db.Exec("DROP TABLE audit_boundary_accounts")
			if _, err = db.ExecContext(ctx, "INSERT INTO audit_boundary_accounts VALUES (1)"); err != nil {
				t.Fatal(err)
			}
			p, meta, cleanup := setupTestPool(t)
			defer cleanup()
			defer p.CloseAll()
			cred, _ := MarshalCredentials(user, "audit-fixture-only")
			p.vault = &mockVault{secrets: map[string]string{"fixture": cred}}
			_, err = Create(meta, "external", driver, host, port, "audit", "", true, false, false, false, "fixture", "disable")
			if err != nil {
				t.Fatal(err)
			}
			for _, q := range []string{"SELECT '--'; DELETE FROM audit_boundary_accounts", "SELECT '/*'; DROP TABLE audit_boundary_accounts; -- */", "WITH changed AS (DELETE FROM audit_boundary_accounts RETURNING id) SELECT * FROM changed"} {
				if _, err = ExecuteQuery(ctx, p, meta, "external", q, 20, 5*time.Second, true); err == nil {
					t.Fatalf("unsafe query accepted: %s", q)
				}
			}
			res, err := ExecuteQuery(ctx, p, meta, "external", "SELECT count(*) AS n FROM audit_boundary_accounts", 20, 5*time.Second, true)
			if err != nil || len(res.Rows) != 1 {
				t.Fatalf("read query failed: %+v %v", res, err)
			}
			var count int
			if err = db.QueryRow("SELECT count(*) FROM audit_boundary_accounts").Scan(&count); err != nil || count != 1 {
				t.Fatalf("data changed: %d %v", count, err)
			}
			tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback()
			if _, err = tx.ExecContext(ctx, "DELETE FROM audit_boundary_accounts"); err == nil {
				t.Fatal("driver did not enforce read-only transaction")
			}
		})
	}
}

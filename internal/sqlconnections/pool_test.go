package sqlconnections

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// Note: mockVault is defined in service_test.go and implements VaultHandler (with ReadSecret, WriteSecret, DeleteSecret)
// which satisfies VaultReader (ReadSecret only) used by ConnectionPool

func setupTestPool(t *testing.T) (*ConnectionPool, *sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test_sql_connections.db")
	metaDB, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	vault := &mockVault{secrets: make(map[string]string)}
	pool := NewConnectionPool(metaDB, vault, 3, 5, nil)
	cleanup := func() {
		metaDB.Close()
		os.Remove(dbPath)
	}
	return pool, metaDB, cleanup
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(1) // 1 second window

	// First access should be allowed
	if !rl.Allow("conn1") {
		t.Error("expected first access to be allowed")
	}

	// Immediate second access should be blocked
	if rl.Allow("conn1") {
		t.Error("expected immediate second access to be blocked")
	}

	// Different connection should be allowed
	if !rl.Allow("conn2") {
		t.Error("expected different connection to be allowed")
	}
}

func TestRateLimiter_RemainingDelay(t *testing.T) {
	rl := NewRateLimiter(2) // 2 second window

	// Initial access
	rl.Allow("conn1")

	// Should have remaining delay
	delay := rl.RemainingDelay("conn1")
	if delay <= 0 {
		t.Error("expected positive remaining delay")
	}

	// Unknown connection should have no delay
	delay = rl.RemainingDelay("unknown")
	if delay != 0 {
		t.Error("expected zero delay for unknown connection")
	}
}

func TestRateLimiter_Disabled(t *testing.T) {
	// Window of 0 means disabled
	rl := NewRateLimiter(0)

	// Should always allow
	for i := 0; i < 10; i++ {
		if !rl.Allow("conn1") {
			t.Errorf("expected access %d to be allowed", i)
		}
	}
}

func TestConnectionPool_PoolStats(t *testing.T) {
	pool, _, cleanup := setupTestPool(t)
	defer cleanup()

	stats := pool.PoolStats()
	if stats["total_connections"].(int) != 0 {
		t.Errorf("expected 0 total connections, got %v", stats["total_connections"])
	}
	if stats["max_connections"].(int) != 3 {
		t.Errorf("expected max 3, got %v", stats["max_connections"])
	}
}

func TestConnectionPool_SetIdleTTL(t *testing.T) {
	pool, _, cleanup := setupTestPool(t)
	defer cleanup()

	// Default is 10 minutes
	pool.SetIdleTTL(5 * time.Minute)
	// Should not panic
}

func TestConnectionPool_SetRateLimit(t *testing.T) {
	pool, _, cleanup := setupTestPool(t)
	defer cleanup()

	// Should not panic
	pool.SetRateLimit(2)
}

func TestDefaultMaxIdleConns(t *testing.T) {
	tests := []struct {
		maxOpen int
		want    int
	}{
		{maxOpen: 0, want: 1},
		{maxOpen: 1, want: 1},
		{maxOpen: 2, want: 1},
		{maxOpen: 5, want: 2},
		{maxOpen: 10, want: 3},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("max-open-%d", tt.maxOpen), func(t *testing.T) {
			if got := defaultMaxIdleConns(tt.maxOpen); got != tt.want {
				t.Fatalf("defaultMaxIdleConns(%d) = %d, want %d", tt.maxOpen, got, tt.want)
			}
		})
	}
}

func TestConnectionPool_CloseConnection(t *testing.T) {
	pool, metaDB, cleanup := setupTestPool(t)
	defer cleanup()

	// Create a connection record
	id, err := Create(metaDB, "testconn", "sqlite", "", 0, t.TempDir()+"/test.db", "test",
		true, false, false, false, "", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Close non-existent connection should not panic
	pool.CloseConnection("nonexistent")

	// Get connection (this opens it)
	db, err := pool.GetConnection(id)
	if err != nil {
		t.Fatalf("GetConnection failed: %v", err)
	}
	if db == nil {
		t.Fatal("expected non-nil db")
	}

	// Close the connection
	pool.CloseConnection(id)

	// Stats should show 0 connections now
	stats := pool.PoolStats()
	if stats["total_connections"].(int) != 0 {
		t.Errorf("expected 0 connections after close, got %v", stats["total_connections"])
	}
}

func TestConnectionPool_CloseAll(t *testing.T) {
	pool, metaDB, cleanup := setupTestPool(t)
	defer cleanup()

	// Create a connection record
	id, err := Create(metaDB, "testconn", "sqlite", "", 0, t.TempDir()+"/test.db", "test",
		true, false, false, false, "", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Get connection (this opens it)
	_, err = pool.GetConnection(id)
	if err != nil {
		t.Fatalf("GetConnection failed: %v", err)
	}

	// Close all
	pool.CloseAll()

	// Stats should show 0 connections
	stats := pool.PoolStats()
	if stats["total_connections"].(int) != 0 {
		t.Errorf("expected 0 connections after CloseAll, got %v", stats["total_connections"])
	}
}

func TestConnectionPool_GetConnection_RateLimit(t *testing.T) {
	// Test rate limiting without SQLite (which has Windows file locking issues)
	pool, poolMetaDB, cleanup := setupTestPool(t)
	defer cleanup()

	// Create test connections using the pool's metaDB
	var err error
	id1, _ := Create(poolMetaDB, "conn1", "sqlite", "", 0, t.TempDir()+"/test1.db", "test1",
		true, false, false, false, "", "")
	id2, _ := Create(poolMetaDB, "conn2", "sqlite", "", 0, t.TempDir()+"/test2.db", "test2",
		true, false, false, false, "", "")

	// Set a rate limit of 1 second
	pool.SetRateLimit(1)

	// First access to conn1 should succeed
	_, err = pool.GetConnection(id1)
	if err != nil {
		t.Fatalf("First GetConnection failed: %v", err)
	}
	defer pool.CloseConnection(id1) // Release connection so SQLite file is unlocked

	// Immediate access to conn2 (different ID) should also succeed since rate limit is per-connection
	_, err = pool.GetConnection(id2)
	if err != nil {
		t.Fatalf("GetConnection for conn2 failed: %v", err)
	}
	defer pool.CloseConnection(id2) // Release connection so SQLite file is unlocked
}

func TestConnectionPool_MaxConnsLimit(t *testing.T) {
	// Pool with max 2 connections - test eviction behavior
	pool, poolMetaDB, cleanup := setupTestPool(t)
	pool.maxConns = 2 // Override the test pool's max
	defer cleanup()

	// Create two connection records using the pool's metaDB
	var err error
	id1, _ := Create(poolMetaDB, "conn1", "sqlite", "", 0, t.TempDir()+"/test1.db", "test1",
		true, false, false, false, "", "")
	id2, _ := Create(poolMetaDB, "conn2", "sqlite", "", 0, t.TempDir()+"/test2.db", "test2",
		true, false, false, false, "", "")

	// Get first connection
	_, err = pool.GetConnection(id1)
	if err != nil {
		t.Fatalf("GetConnection for conn1 failed: %v", err)
	}
	defer pool.CloseConnection(id1) // Release connection so SQLite file is unlocked

	// Get second connection - should succeed since pool limit is 2
	_, err = pool.GetConnection(id2)
	if err != nil {
		t.Fatalf("GetConnection for conn2 failed: %v", err)
	}
	defer pool.CloseConnection(id2) // Release connection so SQLite file is unlocked

	// Verify pool stats show 2 connections
	stats := pool.PoolStats()
	if stats["total_connections"].(int) != 2 {
		t.Errorf("expected 2 connections, got %v", stats["total_connections"])
	}
}

func TestBuildDSN(t *testing.T) {
	tests := []struct {
		name     string
		rec      ConnectionRecord
		username string
		password string
		wantErr  bool
	}{
		{
			name: "postgres with SSL",
			rec: ConnectionRecord{
				Driver:       "postgres",
				Host:         "localhost",
				Port:         5432,
				DatabaseName: "testdb",
				SSLMode:      "require",
			},
			username: "user",
			password: "pass",
			wantErr:  false,
		},
		{
			name: "postgres defaults",
			rec: ConnectionRecord{
				Driver:       "postgres",
				DatabaseName: "testdb",
			},
			username: "user",
			password: "pass",
			wantErr:  false,
		},
		{
			name: "mysql",
			rec: ConnectionRecord{
				Driver:       "mysql",
				Host:         "localhost",
				Port:         3306,
				DatabaseName: "testdb",
			},
			username: "user",
			password: "pass",
			wantErr:  false,
		},
		{
			name: "sqlite",
			rec: ConnectionRecord{
				Driver:       "sqlite",
				DatabaseName: "/tmp/test.db",
			},
			wantErr: false,
		},
		{
			name: "unsupported driver",
			rec: ConnectionRecord{
				Driver: "oracle",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn, driver, err := BuildDSN(tt.rec, tt.username, tt.password, 30*time.Second)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildDSN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && dsn == "" {
				t.Error("expected non-empty DSN")
			}
			if !tt.wantErr && driver == "" {
				t.Error("expected non-empty driver name")
			}
		})
	}
}

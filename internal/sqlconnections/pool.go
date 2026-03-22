package sqlconnections

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// VaultReader is the interface for reading secrets from the vault.
type VaultReader interface {
	ReadSecret(key string) (string, error)
}

// ConnectionPool manages lazy-opened database connections keyed by connection ID.
type ConnectionPool struct {
	mu       sync.Mutex
	conns    map[string]*sql.DB
	metaDB   *sql.DB
	vault    VaultReader
	maxConns int
	timeout  time.Duration
	logger   *slog.Logger
}

// NewConnectionPool creates a ready-to-use pool.
func NewConnectionPool(metaDB *sql.DB, vault VaultReader, maxConns int, timeoutSec int, logger *slog.Logger) *ConnectionPool {
	if maxConns <= 0 {
		maxConns = 5
	}
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	return &ConnectionPool{
		conns:    make(map[string]*sql.DB),
		metaDB:   metaDB,
		vault:    vault,
		maxConns: maxConns,
		timeout:  time.Duration(timeoutSec) * time.Second,
		logger:   logger,
	}
}

// GetConnection returns a cached or newly opened *sql.DB for the given connection ID.
func (p *ConnectionPool) GetConnection(id string) (*sql.DB, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if db, ok := p.conns[id]; ok {
		if err := db.Ping(); err == nil {
			return db, nil
		}
		// stale connection — close and re-open
		db.Close()
		delete(p.conns, id)
	}

	if len(p.conns) >= p.maxConns {
		return nil, fmt.Errorf("connection pool limit reached (%d); close unused connections first", p.maxConns)
	}

	rec, err := GetByID(p.metaDB, id)
	if err != nil {
		return nil, err
	}

	db, err := p.openConnection(rec)
	if err != nil {
		return nil, err
	}

	p.conns[id] = db
	return db, nil
}

// TestConnection opens a connection, pings it, and closes immediately without caching.
func (p *ConnectionPool) TestConnection(rec ConnectionRecord) error {
	db, err := p.openConnection(rec)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	return db.PingContext(ctx)
}

// CloseConnection removes and closes a specific connection from the pool.
func (p *ConnectionPool) CloseConnection(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if db, ok := p.conns[id]; ok {
		db.Close()
		delete(p.conns, id)
	}
}

// CloseAll shuts down every pooled connection.
func (p *ConnectionPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, db := range p.conns {
		db.Close()
		delete(p.conns, id)
	}
}

// openConnection builds a DSN from metadata + vault secret and opens the database.
func (p *ConnectionPool) openConnection(rec ConnectionRecord) (*sql.DB, error) {
	var username, password string

	if rec.VaultSecretID != "" {
		raw, err := p.vault.ReadSecret(rec.VaultSecretID)
		if err != nil {
			return nil, fmt.Errorf("failed to read credentials from vault: %w", err)
		}
		username, password, err = UnmarshalCredentials(raw)
		if err != nil {
			return nil, fmt.Errorf("failed to parse credentials: %w", err)
		}
	}

	dsn, driverName, err := BuildDSN(rec, username, password, p.timeout)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s connection: %w", rec.Driver, err)
	}

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping %s database %q: %w", rec.Driver, rec.DatabaseName, err)
	}

	p.logger.Info("SQL connection opened", "id", rec.ID, "name", rec.Name, "driver", rec.Driver)
	return db, nil
}

// BuildDSN constructs a data source name from connection metadata.
// Exported for testing — never expose the result to the agent.
func BuildDSN(rec ConnectionRecord, username, password string, timeout time.Duration) (dsn string, driverName string, err error) {
	switch rec.Driver {
	case "postgres":
		driverName = "postgres"
		host := rec.Host
		if host == "" {
			host = "localhost"
		}
		port := rec.Port
		if port == 0 {
			port = 5432
		}
		sslMode := rec.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			host, port, username, password, rec.DatabaseName, sslMode)
		return

	case "mysql":
		driverName = "mysql"
		host := rec.Host
		if host == "" {
			host = "localhost"
		}
		port := rec.Port
		if port == 0 {
			port = 3306
		}
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		tls := "false"
		switch rec.SSLMode {
		case "require", "verify-ca", "verify-full":
			tls = "true"
		}
		dsn = fmt.Sprintf("%s:%s@tcp(%s)/%s?tls=%s&parseTime=true&timeout=%s",
			username, password, addr, rec.DatabaseName, tls, timeout.String())
		return

	case "sqlite":
		driverName = "sqlite"
		dsn = rec.DatabaseName // file path
		if dsn == "" {
			return "", "", fmt.Errorf("sqlite: database_name (file path) is required")
		}
		return

	default:
		return "", "", fmt.Errorf("unsupported driver: %s", rec.Driver)
	}
}

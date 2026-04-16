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

// poolEntry holds a database connection along with metadata for lifecycle management.
type poolEntry struct {
	db        *sql.DB
	createdAt time.Time
	lastUsed  time.Time
	rec       ConnectionRecord // connection metadata for reference
}

// ConnectionPool manages lazy-opened database connections keyed by connection ID.
// It provides TTL-based eviction and non-blocking connection retrieval.
type ConnectionPool struct {
	mu       sync.Mutex
	conns    map[string]*poolEntry
	metaDB   *sql.DB
	vault    VaultReader
	maxConns int
	timeout  time.Duration
	idleTTL  time.Duration // how long idle connections are kept
	logger   *slog.Logger

	// Rate limiting
	rateLimiter        *RateLimiter
	rateLimitPerSecond int
	rateLimitBurst     int
}

// RateLimiter provides per-connection rate limiting using a token bucket approach.
type RateLimiter struct {
	mu        sync.Mutex
	lastCheck map[string]time.Time // connection ID -> last access time
	windowSec int                  // minimum seconds between accesses per connection
}

// logSafe logs a message using the provided logger if it is not nil.
func logSafe(logger *slog.Logger, msg string, args ...any) {
	if logger != nil {
		logger.Info(msg, args...)
	}
}

// NewRateLimiter creates a rate limiter with the specified minimum window between accesses.
// If windowSeconds <= 0, rate limiting is effectively disabled (all accesses are allowed).
func NewRateLimiter(windowSeconds int) *RateLimiter {
	return &RateLimiter{
		lastCheck: make(map[string]time.Time),
		windowSec: windowSeconds,
	}
}

// Allow checks if an access is allowed for the given connection.
// Returns false if the connection was accessed too recently.
// If windowSec <= 0, rate limiting is disabled and all accesses are allowed.
func (rl *RateLimiter) Allow(connID string) bool {
	if rl == nil || rl.windowSec <= 0 {
		return true
	}
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if last, ok := rl.lastCheck[connID]; ok {
		elapsed := now.Sub(last).Seconds()
		if elapsed < float64(rl.windowSec) {
			return false
		}
	}
	rl.lastCheck[connID] = now
	return true
}

// RemainingDelay returns the seconds to wait before the next access is allowed.
func (rl *RateLimiter) RemainingDelay(connID string) float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if last, ok := rl.lastCheck[connID]; ok {
		elapsed := time.Since(last).Seconds()
		remaining := float64(rl.windowSec) - elapsed
		if remaining > 0 {
			return remaining
		}
	}
	return 0
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
		conns:    make(map[string]*poolEntry),
		metaDB:   metaDB,
		vault:    vault,
		maxConns: maxConns,
		timeout:  time.Duration(timeoutSec) * time.Second,
		idleTTL:  10 * time.Minute, // default idle TTL
		logger:   logger,
	}
}

// SetIdleTTL sets the maximum idle time before a connection is evicted.
func (p *ConnectionPool) SetIdleTTL(ttl time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.idleTTL = ttl
}

// SetRateLimit configures per-connection rate limiting.
// windowSeconds: minimum seconds between accesses per connection (0 = disabled)
func (p *ConnectionPool) SetRateLimit(windowSeconds int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rateLimiter = NewRateLimiter(windowSeconds)
}

// GetConnection returns a cached or newly opened *sql.DB for the given connection ID.
// It applies rate limiting and TTL-based eviction before returning a connection.
func (p *ConnectionPool) GetConnection(id string) (*sql.DB, error) {
	p.mu.Lock()

	// Check rate limit first (fast path under lock)
	if p.rateLimiter != nil && !p.rateLimiter.Allow(id) {
		delay := p.rateLimiter.RemainingDelay(id)
		p.mu.Unlock()
		return nil, fmt.Errorf("rate limit exceeded for connection %q — retry in %.1fs", id, delay)
	}

	// Evict stale entries before checking pool limit
	p.evictExpiredLocked()

	if entry, ok := p.conns[id]; ok {
		// Update last used time
		entry.lastUsed = time.Now()

		// Check connection health without blocking Ping
		// Use a quick non-blocking check via stats
		stats := entry.db.Stats()
		if stats.InUse == 0 && stats.Idle > 0 {
			// Connection appears healthy and idle
			p.mu.Unlock()
			return entry.db, nil
		}

		// Try a non-blocking PingContext with a short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		if err := entry.db.PingContext(ctx); err != nil {
			// Stale connection — close and remove
			entry.db.Close()
			delete(p.conns, id)
			logSafe(p.logger, "SQL connection stale, removed from pool", "id", id)
		} else {
			p.mu.Unlock()
			return entry.db, nil
		}
	}

	// Pool limit check
	if len(p.conns) >= p.maxConns {
		// Try to evict one idle connection to make room
		if evicted := p.evictOneIdleLocked(); evicted {
			logSafe(p.logger, "Pool limit reached, evicted idle connection to make room", "remaining", len(p.conns))
		} else {
			p.mu.Unlock()
			return nil, fmt.Errorf("connection pool limit reached (%d); close unused connections first", p.maxConns)
		}
	}
	p.mu.Unlock()

	// Fetch record outside the lock to avoid holding it during network I/O
	rec, err := GetByID(p.metaDB, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection record: %w", err)
	}

	db, err := p.openConnection(rec)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double-check we haven't exceeded limit (another goroutine might have added)
	if len(p.conns) >= p.maxConns {
		db.Close()
		return nil, fmt.Errorf("connection pool limit reached (%d); close unused connections first", p.maxConns)
	}

	p.conns[id] = &poolEntry{
		db:        db,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		rec:       rec,
	}
	return db, nil
}

// evictExpiredLocked removes connections that have been idle beyond the TTL.
// Caller must hold p.mu.
func (p *ConnectionPool) evictExpiredLocked() {
	now := time.Now()
	for id, entry := range p.conns {
		if now.Sub(entry.lastUsed) > p.idleTTL {
			entry.db.Close()
			delete(p.conns, id)
			logSafe(p.logger, "Evicted idle connection", "id", id, "idle_duration", now.Sub(entry.lastUsed).Round(time.Second))
		}
	}
}

// evictOneIdleLocked removes a single idle connection if one exists.
// Returns true if a connection was evicted, false if none were idle.
// Caller must hold p.mu.
func (p *ConnectionPool) evictOneIdleLocked() bool {
	now := time.Now()
	for id, entry := range p.conns {
		stats := entry.db.Stats()
		if stats.InUse == 0 {
			entry.db.Close()
			delete(p.conns, id)
			logSafe(p.logger, "Evicted idle connection for pool make-room", "id", id, "idle_duration", now.Sub(entry.lastUsed).Round(time.Second))
			return true
		}
	}
	return false
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
	if entry, ok := p.conns[id]; ok {
		entry.db.Close()
		delete(p.conns, id)
	}
}

// CloseAll shuts down every pooled connection.
func (p *ConnectionPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, entry := range p.conns {
		entry.db.Close()
		delete(p.conns, id)
	}
}

// PoolStats returns current pool statistics for monitoring.
func (p *ConnectionPool) PoolStats() map[string]interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	total := len(p.conns)
	var idle, inUse int
	for _, entry := range p.conns {
		stats := entry.db.Stats()
		idle += stats.Idle
		inUse += stats.InUse
	}

	return map[string]interface{}{
		"total_connections": total,
		"idle":              idle,
		"in_use":            inUse,
		"max_connections":   p.maxConns,
		"rate_limit_window": func() int {
			if p.rateLimiter != nil {
				return p.rateLimiter.windowSec
			}
			return 0
		}(),
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

	maxOpenConns := p.maxConns
	if maxOpenConns <= 0 {
		maxOpenConns = 5
	}
	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(defaultMaxIdleConns(maxOpenConns))
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping %s database %q: %w", rec.Driver, rec.DatabaseName, err)
	}

	logSafe(p.logger, "SQL connection opened", "id", rec.ID, "name", rec.Name, "driver", rec.Driver)
	return db, nil
}

func defaultMaxIdleConns(maxOpenConns int) int {
	if maxOpenConns <= 1 {
		return 1
	}
	idleConns := maxOpenConns / 2
	if idleConns < 1 {
		idleConns = 1
	}
	if idleConns > 3 {
		idleConns = 3
	}
	return idleConns
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

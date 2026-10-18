package dbutil

import "log/slog"

// Option is a functional option for configuring the database connection.
type Option func(*config)

// config holds the configuration for database connections.
type config struct {
	maxOpenConns          int
	busyTimeout           int
	synchronous           string
	corruptionRecovery    bool
	recoveryLogger        *slog.Logger
	immediateTransactions bool
}

// WithImmediateTransactions acquires SQLite's write reservation at Begin rather
// than upgrading a read snapshot after concurrent writers may have committed.
func WithImmediateTransactions() Option {
	return func(c *config) { c.immediateTransactions = true }
}

// defaultConfig returns the default configuration.
func defaultConfig() config {
	return config{
		// Allow a small pool of SQLite connections so reads and writes can
		// overlap under WAL mode. Value was 1 historically and serialized
		// almost all DB access.
		maxOpenConns:       4,
		busyTimeout:        5000,
		synchronous:        "NORMAL",
		corruptionRecovery: false,
		recoveryLogger:     nil,
	}
}

// WithMaxOpenConns sets the maximum number of open connections.
// Default is 4. Must be >= 1.
func WithMaxOpenConns(n int) Option {
	return func(c *config) {
		if n >= 1 {
			c.maxOpenConns = n
		}
	}
}

// WithBusyTimeout sets the busy timeout in milliseconds.
// Default is 5000. Must be >= 0.
func WithBusyTimeout(ms int) Option {
	return func(c *config) {
		if ms >= 0 {
			c.busyTimeout = ms
		}
	}
}

// WithSynchronous sets the synchronous mode.
// Valid values: OFF, NORMAL, FULL, EXTRA.
// Default is NORMAL.
func WithSynchronous(mode string) Option {
	return func(c *config) {
		switch mode {
		case "OFF", "NORMAL", "FULL", "EXTRA":
			c.synchronous = mode
		}
	}
}

// WithCorruptionRecovery enables automatic corruption recovery.
// If corruption is proven, originals are moved to a unique restricted quarantine
// and a fresh database is created. Requires a non-nil logger.
func WithCorruptionRecovery(logger *slog.Logger) Option {
	return func(c *config) {
		if logger != nil {
			c.corruptionRecovery = true
			c.recoveryLogger = logger
		}
	}
}

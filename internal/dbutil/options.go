package dbutil

import "log/slog"

// Option is a functional option for configuring the database connection.
type Option func(*config)

// config holds the configuration for database connections.
type config struct {
	maxOpenConns       int
	busyTimeout        int
	synchronous        string
	corruptionRecovery bool
	recoveryLogger     *slog.Logger
}

// defaultConfig returns the default configuration.
func defaultConfig() config {
	return config{
		maxOpenConns:       1,
		busyTimeout:        5000,
		synchronous:        "NORMAL",
		corruptionRecovery: false,
		recoveryLogger:     nil,
	}
}

// WithMaxOpenConns sets the maximum number of open connections.
// Default is 1. Must be >= 1.
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
// If the database fails integrity check, corrupted files are renamed to .bak
// and a fresh database is created. Requires a non-nil logger.
func WithCorruptionRecovery(logger *slog.Logger) Option {
	return func(c *config) {
		if logger != nil {
			c.corruptionRecovery = true
			c.recoveryLogger = logger
		}
	}
}

package warnings

import (
	"log/slog"
	"time"

	"aurago/internal/config"
)

const vectorDBValidationWatchTimeout = 3 * time.Minute

// VectorDBHealth exposes the runtime readiness state needed for warning producers.
type VectorDBHealth interface {
	IsReady() bool
	IsDisabled() bool
}

// VectorDBMonitor emits a warning when embeddings are configured but the startup
// validation goroutine marks the VectorDB as disabled.
type VectorDBMonitor struct {
	reg    *Registry
	cfg    *config.Config
	vdb    VectorDBHealth
	logger *slog.Logger
}

// NewVectorDBMonitor creates a monitor for runtime VectorDB health warnings.
func NewVectorDBMonitor(reg *Registry, cfg *config.Config, vdb VectorDBHealth, logger *slog.Logger) *VectorDBMonitor {
	return &VectorDBMonitor{
		reg:    reg,
		cfg:    cfg,
		vdb:    vdb,
		logger: logger,
	}
}

// Start watches VectorDB readiness in the background. It is a no-op when embeddings
// are intentionally disabled in config (handled synchronously by checkVectorDBDisabled).
func (m *VectorDBMonitor) Start() {
	if m == nil || m.reg == nil || m.vdb == nil || !embeddingsConfigured(m.cfg) {
		return
	}
	go m.watch()
}

func (m *VectorDBMonitor) watch() {
	deadline := time.Now().Add(vectorDBValidationWatchTimeout)
	for time.Now().Before(deadline) {
		if m.vdb.IsReady() {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !m.vdb.IsReady() || !m.vdb.IsDisabled() {
		return
	}

	m.reg.Add(Warning{
		ID:       "vectordb_validation_failed",
		Severity: SeverityWarning,
		Title:    "Long-Term Memory Unavailable",
		Description: "Embedding validation failed at startup, so long-term memory search and storage are disabled until AuraGo is restarted with a working embedding provider. " +
			"Check the application log for provider errors and verify your embeddings configuration in the Web UI.",
		Category: CategorySystem,
	})
	if m.logger != nil {
		m.logger.Warn("Registered warning: embedding pipeline validation failed at startup")
	}
}

func embeddingsConfigured(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	provider := cfg.Embeddings.Provider
	return provider != "" && provider != "disabled"
}

// WatchVectorDBRecovery monitors a replacement VectorDB after hot-reload (e.g. setup
// wizard). It starts the standard validation monitor and clears
// vectordb_validation_failed only after validation completes successfully.
func WatchVectorDBRecovery(reg *Registry, cfg *config.Config, vdb VectorDBHealth, logger *slog.Logger) {
	if reg == nil || vdb == nil || !embeddingsConfigured(cfg) {
		return
	}
	NewVectorDBMonitor(reg, cfg, vdb, logger).Start()
	go watchVectorDBRecoveryClear(reg, vdb, logger)
}

func watchVectorDBRecoveryClear(reg *Registry, vdb VectorDBHealth, logger *slog.Logger) {
	deadline := time.Now().Add(vectorDBValidationWatchTimeout)
	for time.Now().Before(deadline) {
		if vdb.IsReady() {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !vdb.IsReady() {
		return
	}
	if vdb.IsDisabled() {
		return
	}
	reg.Remove("vectordb_validation_failed")
	if logger != nil {
		logger.Info("Cleared vectordb_validation_failed after successful VectorDB recovery")
	}
}
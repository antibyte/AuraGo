package sqlconnections

import (
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"aurago/internal/uid"
)

// VaultHandler defines the interface for vault operations used by the service layer.
// This abstracts the concrete vault implementation to keep the service testable.
type VaultHandler interface {
	ReadSecret(key string) (string, error)
	WriteSecret(key, value string) error
	DeleteSecret(key string) error
}

// Service provides a centralized SQL connection lifecycle management.
// It encapsulates metadata CRUD, vault credential handling, and secret rotation.
// This avoids duplication across server handlers and agent dispatch.
type Service struct {
	db          *sql.DB
	vault       VaultHandler
	pool        *ConnectionPool
	logger      *slog.Logger
	readOnly    bool // global read-only flag
	allowManage bool // agent management policy
}

// ServiceConfig holds the configuration for a new Service.
type ServiceConfig struct {
	DB          *sql.DB
	Vault       VaultHandler
	Pool        *ConnectionPool
	Logger      *slog.Logger
	ReadOnly    bool // global SQL read-only (blocks all mutating queries)
	AllowManage bool // agent can create/update/delete connections
}

// NewService creates a new SQL connection service.
func NewService(cfg ServiceConfig) *Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		db:          cfg.DB,
		vault:       cfg.Vault,
		pool:        cfg.Pool,
		logger:      logger,
		readOnly:    cfg.ReadOnly,
		allowManage: cfg.AllowManage,
	}
}

// CreateRequest encapsulates the data needed to create a new connection.
type CreateRequest struct {
	Name         string
	Driver       string
	Host         string
	Port         int
	DatabaseName string
	Description  string
	Username     string
	Password     string
	SSLMode      string
	AllowRead    bool
	AllowWrite   bool
	AllowChange  bool
	AllowDelete  bool
}

// CreateResult returns the ID of the newly created connection.
type CreateResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Create adds a new SQL connection with secure credential storage.
// Credentials are stored in the vault with an opaque, non-deterministic key.
func (s *Service) Create(req CreateRequest) (*CreateResult, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("connection name is required")
	}
	if req.Driver != "postgres" && req.Driver != "mysql" && req.Driver != "sqlite" {
		return nil, fmt.Errorf("unsupported driver: %s (must be postgres, mysql, or sqlite)", req.Driver)
	}

	// Generate opaque vault key - not derived from connection name
	vaultSecretID := "sql_" + uid.New()

	// Store credentials in vault if provided
	if req.Username != "" || req.Password != "" {
		credJSON, err := MarshalCredentials(req.Username, req.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal credentials: %w", err)
		}
		if s.vault != nil {
			if err := s.vault.WriteSecret(vaultSecretID, credJSON); err != nil {
				return nil, fmt.Errorf("failed to store credentials: %w", err)
			}
			s.logger.Info("SQL connection credentials stored in vault", "secret_id", vaultSecretID, "connection", req.Name)
		}
	}

	if req.SSLMode == "" {
		req.SSLMode = "disable"
	}

	id, err := Create(s.db, req.Name, req.Driver, req.Host, req.Port, req.DatabaseName, req.Description,
		req.AllowRead, req.AllowWrite, req.AllowChange, req.AllowDelete, vaultSecretID, req.SSLMode)
	if err != nil {
		// Best-effort cleanup of vault secret on failure
		if vaultSecretID != "" && s.vault != nil {
			_ = s.vault.DeleteSecret(vaultSecretID)
		}
		return nil, err
	}

	return &CreateResult{ID: id, Name: req.Name}, nil
}

// UpdateRequest encapsulates the data needed to update a connection.
// CredentialAction specifies how to handle credentials: "keep", "replace", or "delete".
type UpdateRequest struct {
	ID               string
	Name             string
	Driver           string
	Host             string
	Port             int
	DatabaseName     string
	Description      string
	SSLMode          string
	AllowRead        bool
	AllowWrite       bool
	AllowChange      bool
	AllowDelete      bool
	CredentialAction string // "keep" (default), "replace", "delete"
	Username         string
	Password         string
}

// Update modifies an existing connection. It handles credential rotation with
// old secret cleanup on success. Rename without credential change is supported.
func (s *Service) Update(req UpdateRequest) error {
	if req.ID == "" {
		return fmt.Errorf("connection ID is required")
	}
	if req.Name == "" {
		return fmt.Errorf("connection name is required")
	}
	if req.Driver != "" && req.Driver != "postgres" && req.Driver != "mysql" && req.Driver != "sqlite" {
		return fmt.Errorf("unsupported driver: %s", req.Driver)
	}

	// Fetch existing record
	existing, err := GetByID(s.db, req.ID)
	if err != nil {
		return err
	}

	// Track old vault secret for cleanup after successful rotation
	oldVaultSecretID := existing.VaultSecretID
	var newVaultSecretID string

	// Apply field updates
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Driver != "" {
		existing.Driver = req.Driver
	}
	if req.Host != "" {
		existing.Host = req.Host
	}
	if req.Port > 0 {
		existing.Port = req.Port
	}
	if req.DatabaseName != "" {
		existing.DatabaseName = req.DatabaseName
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if req.SSLMode != "" {
		existing.SSLMode = req.SSLMode
	}
	// Permission flags - only update if explicitly set (allow false to be intentional)
	existing.AllowRead = req.AllowRead
	existing.AllowWrite = req.AllowWrite
	existing.AllowChange = req.AllowChange
	existing.AllowDelete = req.AllowDelete

	// Handle credential changes based on action
	switch req.CredentialAction {
	case "replace":
		// Generate new opaque vault key
		newVaultSecretID = "sql_" + uid.New()
		credJSON, err := MarshalCredentials(req.Username, req.Password)
		if err != nil {
			return fmt.Errorf("failed to marshal credentials: %w", err)
		}
		if s.vault != nil {
			if err := s.vault.WriteSecret(newVaultSecretID, credJSON); err != nil {
				return fmt.Errorf("failed to store new credentials: %w", err)
			}
			s.logger.Info("SQL connection credentials rotated", "old_secret_id", oldVaultSecretID, "new_secret_id", newVaultSecretID, "connection", existing.Name)
		}
		existing.VaultSecretID = newVaultSecretID

	case "delete":
		newVaultSecretID = "" // mark for deletion
		existing.VaultSecretID = ""
		if oldVaultSecretID != "" && s.vault != nil {
			_ = s.vault.DeleteSecret(oldVaultSecretID)
			s.logger.Info("SQL connection credentials deleted from vault", "secret_id", oldVaultSecretID, "connection", existing.Name)
		}

	case "keep", "":
		// No change to credentials
	default:
		return fmt.Errorf("invalid credential_action: %s (use: keep, replace, delete)", req.CredentialAction)
	}

	// Close pooled connection to force reconnect with new settings
	if s.pool != nil {
		s.pool.CloseConnection(existing.ID)
	}

	if err := Update(s.db, existing.ID, existing.Name, existing.Driver, existing.Host, existing.Port,
		existing.DatabaseName, existing.Description,
		existing.AllowRead, existing.AllowWrite, existing.AllowChange, existing.AllowDelete,
		existing.VaultSecretID, existing.SSLMode); err != nil {
		return err
	}

	// Best-effort cleanup of old secret after successful update
	if newVaultSecretID != "" && oldVaultSecretID != "" && oldVaultSecretID != newVaultSecretID {
		if s.vault != nil {
			_ = s.vault.DeleteSecret(oldVaultSecretID)
		}
	}

	return nil
}

// DeleteRequest encapsulates the data needed to delete a connection.
type DeleteRequest struct {
	ID string
}

// Delete removes a connection and its vault credentials.
func (s *Service) Delete(req DeleteRequest) error {
	if req.ID == "" {
		return fmt.Errorf("connection ID is required")
	}

	// Fetch existing to get vault secret ID
	existing, err := GetByID(s.db, req.ID)
	if err != nil {
		return err
	}

	// Close pooled connection
	if s.pool != nil {
		s.pool.CloseConnection(existing.ID)
	}

	if err := Delete(s.db, req.ID); err != nil {
		return err
	}

	// Clean up vault secret
	if existing.VaultSecretID != "" && s.vault != nil {
		_ = s.vault.DeleteSecret(existing.VaultSecretID)
		s.logger.Info("SQL connection deleted", "id", req.ID, "name", existing.Name)
	}

	return nil
}

// IsReadOnly returns whether global read-only mode is enabled.
func (s *Service) IsReadOnly() bool {
	return s.readOnly
}

// CanManage returns whether agent management operations are allowed.
func (s *Service) CanManage() bool {
	return s.allowManage
}

// TestConnection tests connectivity for a connection by ID.
func (s *Service) TestConnection(id string) error {
	rec, err := GetByID(s.db, id)
	if err != nil {
		return err
	}
	if s.pool == nil {
		return fmt.Errorf("connection pool not available")
	}
	return s.pool.TestConnection(rec)
}

// GetByID returns a connection by its ID.
func (s *Service) GetByID(id string) (ConnectionRecord, error) {
	return GetByID(s.db, id)
}

// GetByName returns a connection by its unique name.
func (s *Service) GetByName(name string) (ConnectionRecord, error) {
	return GetByName(s.db, name)
}

// List returns all connection records.
func (s *Service) List() ([]ConnectionRecord, error) {
	return List(s.db)
}

// SetPool updates the connection pool reference.
func (s *Service) SetPool(pool *ConnectionPool) {
	s.pool = pool
}

// SetReadOnly updates the read-only flag.
func (s *Service) SetReadOnly(readOnly bool) {
	s.readOnly = readOnly
}

// SetAllowManage updates the agent management permission.
func (s *Service) SetAllowManage(allowManage bool) {
	s.allowManage = allowManage
}

// Helper to get current timestamp in RFC3339
func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}

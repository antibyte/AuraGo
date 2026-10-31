package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"aurago/internal/sqlconnections"
)

type sqlConnectionRequest struct {
	Name         string `json:"name"`
	Driver       string `json:"driver"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	DatabaseName string `json:"database_name"`
	Description  string `json:"description"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	SSLMode      string `json:"ssl_mode"`
	AllowRead    *bool  `json:"allow_read"`
	AllowWrite   *bool  `json:"allow_write"`
	AllowChange  *bool  `json:"allow_change"`
	AllowDelete  *bool  `json:"allow_delete"`
}

func boolValueOrDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func resolveSQLConnectionCreatePermissions(req sqlConnectionRequest) (bool, bool, bool, bool) {
	return boolValueOrDefault(req.AllowRead, true),
		boolValueOrDefault(req.AllowWrite, false),
		boolValueOrDefault(req.AllowChange, false),
		boolValueOrDefault(req.AllowDelete, false)
}

func resolveSQLConnectionUpdatePermissions(req sqlConnectionRequest, existing sqlconnections.ConnectionRecord) (bool, bool, bool, bool) {
	return boolValueOrDefault(req.AllowRead, existing.AllowRead),
		boolValueOrDefault(req.AllowWrite, existing.AllowWrite),
		boolValueOrDefault(req.AllowChange, existing.AllowChange),
		boolValueOrDefault(req.AllowDelete, existing.AllowDelete)
}

func newSQLConnectionService(s *Server) *sqlconnections.Service {
	return sqlconnections.NewService(sqlconnections.ServiceConfig{
		DB:     s.SQLConnectionsDB,
		Vault:  s.Vault,
		Pool:   s.SQLConnectionPool,
		Logger: s.Logger,
	})
}

// sqlConnectionClientMessage shows TLS-mode validation errors verbatim (they
// only echo the administrator's input) and keeps every other error generic.
func sqlConnectionClientMessage(err error, fallback string) string {
	if errors.Is(err, sqlconnections.ErrSSLModeRequired) || errors.Is(err, sqlconnections.ErrSSLModeInvalid) {
		return err.Error()
	}
	return fallback
}

func buildSQLConnectionCreateRequest(req sqlConnectionRequest) sqlconnections.CreateRequest {
	allowRead, allowWrite, allowChange, allowDelete := resolveSQLConnectionCreatePermissions(req)
	// No default TLS mode here: the service rejects PostgreSQL/MySQL connections
	// without an explicit ssl_mode and applies the SQLite default itself.
	return sqlconnections.CreateRequest{
		Name:         req.Name,
		Driver:       req.Driver,
		Host:         req.Host,
		Port:         req.Port,
		DatabaseName: req.DatabaseName,
		Description:  req.Description,
		Username:     req.Username,
		Password:     req.Password,
		SSLMode:      req.SSLMode,
		AllowRead:    allowRead,
		AllowWrite:   allowWrite,
		AllowChange:  allowChange,
		AllowDelete:  allowDelete,
	}
}

func buildSQLConnectionUpdateRequest(req sqlConnectionRequest, existing sqlconnections.ConnectionRecord) sqlconnections.UpdateRequest {
	allowRead, allowWrite, allowChange, allowDelete := resolveSQLConnectionUpdatePermissions(req, existing)
	updateReq := sqlconnections.UpdateRequest{
		ID:               existing.ID,
		Name:             existing.Name,
		Driver:           existing.Driver,
		Host:             existing.Host,
		Port:             existing.Port,
		DatabaseName:     existing.DatabaseName,
		Description:      existing.Description,
		SSLMode:          existing.SSLMode,
		AllowRead:        allowRead,
		AllowWrite:       allowWrite,
		AllowChange:      allowChange,
		AllowDelete:      allowDelete,
		CredentialAction: "keep",
	}

	if req.Name != "" {
		updateReq.Name = req.Name
	}
	if req.Driver != "" {
		updateReq.Driver = req.Driver
	}
	if req.Host != "" {
		updateReq.Host = req.Host
	}
	if req.Port > 0 {
		updateReq.Port = req.Port
	}
	if req.DatabaseName != "" {
		updateReq.DatabaseName = req.DatabaseName
	}
	if req.Description != "" {
		updateReq.Description = req.Description
	}
	if req.SSLMode != "" {
		updateReq.SSLMode = req.SSLMode
	}
	if req.Username != "" || req.Password != "" {
		updateReq.CredentialAction = "replace"
		updateReq.Username = req.Username
		updateReq.Password = req.Password
	}

	return updateReq
}

// handleSQLConnections handles GET (list) and POST (create) on /api/sql-connections.
func handleSQLConnections(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.SQLConnectionsDB == nil {
			jsonError(w, "SQL connections not initialized", http.StatusServiceUnavailable)
			return
		}
		service := newSQLConnectionService(s)
		switch r.Method {
		case http.MethodGet:
			list, err := service.List()
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to list SQL connections", "Failed to list SQL connections", err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(list)

		case http.MethodPost:
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				jsonError(w, "failed to read body", http.StatusBadRequest)
				return
			}
			var req sqlConnectionRequest
			if err := json.Unmarshal(body, &req); err != nil {
				jsonError(w, "invalid JSON", http.StatusBadRequest)
				return
			}

			result, err := service.Create(buildSQLConnectionCreateRequest(req))
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, sqlConnectionClientMessage(err, "Failed to create SQL connection"), "Failed to create SQL connection", err, "connection_name", req.Name)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"id": result.ID})

		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleSQLConnectionByID handles GET, PUT, DELETE on /api/sql-connections/{id}.
func handleSQLConnectionByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.SQLConnectionsDB == nil {
			jsonError(w, "SQL connections not initialized", http.StatusServiceUnavailable)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/sql-connections/")
		if id == "" {
			jsonError(w, "missing connection id", http.StatusBadRequest)
			return
		}
		service := newSQLConnectionService(s)

		switch r.Method {
		case http.MethodGet:
			rec, err := service.GetByID(id)
			if err != nil {
				jsonError(w, "SQL connection not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(rec)

		case http.MethodPut:
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				jsonError(w, "failed to read body", http.StatusBadRequest)
				return
			}
			var req sqlConnectionRequest
			if err := json.Unmarshal(body, &req); err != nil {
				jsonError(w, "invalid JSON", http.StatusBadRequest)
				return
			}

			existing, err := service.GetByID(id)
			if err != nil {
				jsonError(w, "SQL connection not found", http.StatusNotFound)
				return
			}

			if err := service.Update(buildSQLConnectionUpdateRequest(req, existing)); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, sqlConnectionClientMessage(err, "Failed to update SQL connection"), "Failed to update SQL connection", err, "connection_id", id)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

		case http.MethodDelete:
			if _, err := service.GetByID(id); err != nil {
				jsonError(w, "SQL connection not found", http.StatusNotFound)
				return
			}

			if err := service.Delete(sqlconnections.DeleteRequest{ID: id}); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to delete SQL connection", "Failed to delete SQL connection", err, "connection_id", id)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

		default:
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// sqlConnectionTestTarget is what a connection test connects to: the stored
// record with the edited fields laid over it, plus the credentials to use.
type sqlConnectionTestTarget struct {
	Record   sqlconnections.ConnectionRecord
	Username string
	Password string
	// UseStoredVault reports that the stored vault secret is used as is (no edited fields).
	UseStoredVault bool
}

var (
	errSQLTestSaveFirst       = errors.New("save the connection first to test a SQLite database")
	errSQLTestPasswordNeeded  = errors.New("the connection target changed - enter the password to test it")
	errSQLTestRequiredNetwork = errors.New("driver, host and database are required to test a connection")
	errSQLTestInvalidSSLMode  = errors.New("ssl_mode must be one of disable, allow, prefer, require, verify-ca, verify-full")
)

// sqlSSLModeRank orders the TLS modes by strength. Reports false for an unknown mode.
func sqlSSLModeRank(mode string) (int, bool) {
	switch mode {
	case "disable":
		return 0, true
	case "allow", "prefer":
		return 1, true
	case "require":
		return 2, true
	case "verify-ca":
		return 3, true
	case "verify-full":
		return 4, true
	}
	return 0, false
}

// sqlSSLModeWeakened reports whether testing with the drafted mode instead of the
// stored one lowers the transport protection. Only a stronger mode keeps the
// stored password; a weaker, different or unknown stored mode counts as a change.
func sqlSSLModeWeakened(stored, drafted string) bool {
	if stored == drafted {
		return false
	}
	if stored == "" {
		stored = "disable" // the pool's default for an unset mode
	}
	storedRank, storedKnown := sqlSSLModeRank(stored)
	draftedRank, draftedKnown := sqlSSLModeRank(drafted)
	return !storedKnown || !draftedKnown || draftedRank <= storedRank
}

// resolveSQLConnectionTestTarget lays the edited record fields over the stored
// record (nil for a connection that is not saved yet). Stored credentials follow
// the target: they are only reused while driver, host, port, database and user
// are unchanged, the TLS mode is not weakened (disable < allow/prefer < require <
// verify-ca < verify-full; an unknown mode is rejected) and no new password is typed; a changed target needs the typed
// password, so a stored secret is never sent to an edited address.
func resolveSQLConnectionTestTarget(req sqlConnectionRequest, stored *sqlconnections.ConnectionRecord, storedUser, storedPassword string) (sqlConnectionTestTarget, error) {
	var rec sqlconnections.ConnectionRecord
	if stored != nil {
		rec = *stored
	}
	if v := strings.TrimSpace(req.Driver); v != "" {
		rec.Driver = v
	}
	if v := strings.TrimSpace(req.Host); v != "" {
		rec.Host = v
	}
	if req.Port > 0 {
		rec.Port = req.Port
	}
	if v := strings.TrimSpace(req.DatabaseName); v != "" {
		rec.DatabaseName = v
	}
	if v := strings.TrimSpace(req.SSLMode); v != "" {
		if _, known := sqlSSLModeRank(v); !known {
			return sqlConnectionTestTarget{}, errSQLTestInvalidSSLMode
		}
		rec.SSLMode = v
	}
	if v := strings.TrimSpace(req.Name); v != "" {
		rec.Name = v
	}

	username := strings.TrimSpace(req.Username)
	password := req.Password

	if stored == nil {
		if rec.Driver == "sqlite" {
			return sqlConnectionTestTarget{}, errSQLTestSaveFirst
		}
		if rec.Driver == "" || strings.TrimSpace(rec.DatabaseName) == "" {
			return sqlConnectionTestTarget{}, errSQLTestRequiredNetwork
		}
		return sqlConnectionTestTarget{Record: rec, Username: username, Password: password}, nil
	}

	if rec.Driver == "sqlite" || stored.Driver == "sqlite" {
		// SQLite databases are managed uploads: only the stored generation can be tested.
		return sqlConnectionTestTarget{Record: *stored, UseStoredVault: true}, nil
	}

	targetChanged := !strings.EqualFold(rec.Driver, stored.Driver) ||
		!strings.EqualFold(rec.Host, stored.Host) ||
		rec.Port != stored.Port ||
		rec.DatabaseName != stored.DatabaseName ||
		(username != "" && username != storedUser) ||
		sqlSSLModeWeakened(stored.SSLMode, rec.SSLMode)

	if password == "" {
		if targetChanged {
			return sqlConnectionTestTarget{}, errSQLTestPasswordNeeded
		}
		if username == "" && rec.SSLMode == stored.SSLMode {
			return sqlConnectionTestTarget{Record: rec, UseStoredVault: true}, nil
		}
		password = storedPassword
	}
	if username == "" {
		username = storedUser
	}
	return sqlConnectionTestTarget{Record: rec, Username: username, Password: password}, nil
}

// handleSQLConnectionTest handles POST on /api/sql-connections/{id}/test.
// The optional JSON body carries the edited record fields (same shape as the
// save request); {id} "new" tests a record that is not saved yet.
func handleSQLConnectionTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.SQLConnectionsDB == nil {
			jsonError(w, "SQL connections database not initialized", http.StatusServiceUnavailable)
			return
		}
		if s.SQLConnectionPool == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "SQL connections are not enabled. Enable them in Configuration → SQL Connections and save.",
			})
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Extract ID from /api/sql-connections/{id}/test
		path := strings.TrimPrefix(r.URL.Path, "/api/sql-connections/")
		id := strings.TrimSuffix(path, "/test")
		if id == "" {
			jsonError(w, "missing connection id", http.StatusBadRequest)
			return
		}

		var req sqlConnectionRequest
		if r.Body != nil {
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				jsonError(w, "failed to read body", http.StatusBadRequest)
				return
			}
			if len(strings.TrimSpace(string(body))) > 0 {
				if err := json.Unmarshal(body, &req); err != nil {
					jsonError(w, "invalid JSON", http.StatusBadRequest)
					return
				}
			}
		}

		service := newSQLConnectionService(s)
		var stored *sqlconnections.ConnectionRecord
		var storedUser, storedPassword string
		if id != "new" {
			rec, err := service.GetByID(id)
			if err != nil {
				jsonError(w, "SQL connection not found", http.StatusNotFound)
				return
			}
			stored = &rec
			if rec.VaultSecretID != "" && rec.Driver != "sqlite" && s.Vault != nil {
				if raw, err := s.Vault.ReadSecret(rec.VaultSecretID); err == nil {
					storedUser, storedPassword, _ = sqlconnections.UnmarshalCredentials(raw)
				}
			}
		}

		target, err := resolveSQLConnectionTestTarget(req, stored, storedUser, storedPassword)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error", "message": err.Error()})
			return
		}

		if target.UseStoredVault {
			err = s.SQLConnectionPool.TestConnection(target.Record)
		} else {
			err = s.SQLConnectionPool.TestConnectionWithCredentials(target.Record, target.Username, target.Password)
		}
		if err != nil {
			// Sanitize error message to prevent leaking driver-specific details
			if s.Logger != nil {
				s.Logger.Warn("SQL connection test failed", "connection_id", id, "connection_name", target.Record.Name, "error", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Connection test failed. Check host, port, credentials, and database availability.",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "Connection successful",
		})
	}
}

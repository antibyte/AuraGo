package server

import (
	"encoding/json"
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

func buildSQLConnectionCreateRequest(req sqlConnectionRequest) sqlconnections.CreateRequest {
	allowRead, allowWrite, allowChange, allowDelete := resolveSQLConnectionCreatePermissions(req)
	sslMode := req.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	return sqlconnections.CreateRequest{
		Name:         req.Name,
		Driver:       req.Driver,
		Host:         req.Host,
		Port:         req.Port,
		DatabaseName: req.DatabaseName,
		Description:  req.Description,
		Username:     req.Username,
		Password:     req.Password,
		SSLMode:      sslMode,
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
			jsonError(w, `{"error":"SQL connections not initialized"}`, http.StatusServiceUnavailable)
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
				jsonError(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
				return
			}
			var req sqlConnectionRequest
			if err := json.Unmarshal(body, &req); err != nil {
				jsonError(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}

			result, err := service.Create(buildSQLConnectionCreateRequest(req))
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to create SQL connection", "Failed to create SQL connection", err, "connection_name", req.Name)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"id": result.ID})

		default:
			jsonError(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

// handleSQLConnectionByID handles GET, PUT, DELETE on /api/sql-connections/{id}.
func handleSQLConnectionByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.SQLConnectionsDB == nil {
			jsonError(w, `{"error":"SQL connections not initialized"}`, http.StatusServiceUnavailable)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/sql-connections/")
		if id == "" {
			jsonError(w, `{"error":"missing connection id"}`, http.StatusBadRequest)
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
				jsonError(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
				return
			}
			var req sqlConnectionRequest
			if err := json.Unmarshal(body, &req); err != nil {
				jsonError(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}

			existing, err := service.GetByID(id)
			if err != nil {
				jsonError(w, "SQL connection not found", http.StatusNotFound)
				return
			}

			if err := service.Update(buildSQLConnectionUpdateRequest(req, existing)); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to update SQL connection", "Failed to update SQL connection", err, "connection_id", id)
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
			jsonError(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

// handleSQLConnectionTest handles POST on /api/sql-connections/{id}/test.
func handleSQLConnectionTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.SQLConnectionsDB == nil {
			jsonError(w, `{"error":"SQL connections database not initialized"}`, http.StatusServiceUnavailable)
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
			jsonError(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// Extract ID from /api/sql-connections/{id}/test
		path := strings.TrimPrefix(r.URL.Path, "/api/sql-connections/")
		id := strings.TrimSuffix(path, "/test")
		if id == "" {
			jsonError(w, `{"error":"missing connection id"}`, http.StatusBadRequest)
			return
		}

		service := newSQLConnectionService(s)
		rec, err := service.GetByID(id)
		if err != nil {
			jsonError(w, "SQL connection not found", http.StatusNotFound)
			return
		}

		if err := service.TestConnection(rec.ID); err != nil {
			// Sanitize error message to prevent leaking driver-specific details
			if s.Logger != nil {
				s.Logger.Warn("SQL connection test failed", "connection_id", id, "connection_name", rec.Name, "error", err)
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

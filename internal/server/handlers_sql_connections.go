package server

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"aurago/internal/sqlconnections"
)

// handleSQLConnections handles GET (list) and POST (create) on /api/sql-connections.
func handleSQLConnections(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.SQLConnectionsDB == nil {
			http.Error(w, `{"error":"SQL connections not initialized"}`, http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			list, err := sqlconnections.List(s.SQLConnectionsDB)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to list SQL connections", "Failed to list SQL connections", err)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(list)

		case http.MethodPost:
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
				return
			}
			var req struct {
				Name         string `json:"name"`
				Driver       string `json:"driver"`
				Host         string `json:"host"`
				Port         int    `json:"port"`
				DatabaseName string `json:"database_name"`
				Description  string `json:"description"`
				Username     string `json:"username"`
				Password     string `json:"password"`
				SSLMode      string `json:"ssl_mode"`
				AllowRead    bool   `json:"allow_read"`
				AllowWrite   bool   `json:"allow_write"`
				AllowChange  bool   `json:"allow_change"`
				AllowDelete  bool   `json:"allow_delete"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}
			if req.SSLMode == "" {
				req.SSLMode = "disable"
			}

			// Store credentials in vault
			vaultKey := ""
			if req.Username != "" || req.Password != "" {
				credJSON, err := sqlconnections.MarshalCredentials(req.Username, req.Password)
				if err != nil {
					http.Error(w, `{"error":"failed to marshal credentials"}`, http.StatusInternalServerError)
					return
				}
				vaultKey = "sqlconn_" + req.Name
				if s.Vault != nil {
					if err := s.Vault.WriteSecret(vaultKey, credJSON); err != nil {
						http.Error(w, `{"error":"failed to store credentials"}`, http.StatusInternalServerError)
						return
					}
				}
			}

			id, err := sqlconnections.Create(s.SQLConnectionsDB,
				req.Name, req.Driver, req.Host, req.Port, req.DatabaseName, req.Description,
				req.AllowRead, req.AllowWrite, req.AllowChange, req.AllowDelete, vaultKey, req.SSLMode)
			if err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to create SQL connection", "Failed to create SQL connection", err, "connection_name", req.Name)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"id": id})

		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

// handleSQLConnectionByID handles GET, PUT, DELETE on /api/sql-connections/{id}.
func handleSQLConnectionByID(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.SQLConnectionsDB == nil {
			http.Error(w, `{"error":"SQL connections not initialized"}`, http.StatusServiceUnavailable)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/sql-connections/")
		if id == "" {
			http.Error(w, `{"error":"missing connection id"}`, http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			rec, err := sqlconnections.GetByID(s.SQLConnectionsDB, id)
			if err != nil {
				jsonError(w, "SQL connection not found", http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(rec)

		case http.MethodPut:
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if err != nil {
				http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
				return
			}
			var req struct {
				Name         string `json:"name"`
				Driver       string `json:"driver"`
				Host         string `json:"host"`
				Port         int    `json:"port"`
				DatabaseName string `json:"database_name"`
				Description  string `json:"description"`
				Username     string `json:"username"`
				Password     string `json:"password"`
				SSLMode      string `json:"ssl_mode"`
				AllowRead    bool   `json:"allow_read"`
				AllowWrite   bool   `json:"allow_write"`
				AllowChange  bool   `json:"allow_change"`
				AllowDelete  bool   `json:"allow_delete"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
				return
			}

			// Update credentials if provided
			existing, err := sqlconnections.GetByID(s.SQLConnectionsDB, id)
			if err != nil {
				jsonError(w, "SQL connection not found", http.StatusNotFound)
				return
			}
			vaultKey := existing.VaultSecretID
			if req.Username != "" || req.Password != "" {
				credJSON, err := sqlconnections.MarshalCredentials(req.Username, req.Password)
				if err != nil {
					http.Error(w, `{"error":"failed to marshal credentials"}`, http.StatusInternalServerError)
					return
				}
				if vaultKey == "" {
					vaultKey = "sqlconn_" + req.Name
				}
				if s.Vault != nil {
					if err := s.Vault.WriteSecret(vaultKey, credJSON); err != nil {
						http.Error(w, `{"error":"failed to store credentials"}`, http.StatusInternalServerError)
						return
					}
				}
			}

			if err := sqlconnections.Update(s.SQLConnectionsDB,
				id, req.Name, req.Driver, req.Host, req.Port, req.DatabaseName, req.Description,
				req.AllowRead, req.AllowWrite, req.AllowChange, req.AllowDelete, vaultKey, req.SSLMode); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusBadRequest, "Failed to update SQL connection", "Failed to update SQL connection", err, "connection_id", id)
				return
			}

			// Close cached connection to force reconnect
			if s.SQLConnectionPool != nil {
				s.SQLConnectionPool.CloseConnection(id)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "updated"})

		case http.MethodDelete:
			existing, err := sqlconnections.GetByID(s.SQLConnectionsDB, id)
			if err != nil {
				jsonError(w, "SQL connection not found", http.StatusNotFound)
				return
			}

			// Close cached connection
			if s.SQLConnectionPool != nil {
				s.SQLConnectionPool.CloseConnection(id)
			}

			if err := sqlconnections.Delete(s.SQLConnectionsDB, id); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to delete SQL connection", "Failed to delete SQL connection", err, "connection_id", id)
				return
			}

			// Clean up vault secret
			if existing.VaultSecretID != "" && s.Vault != nil {
				_ = s.Vault.DeleteSecret(existing.VaultSecretID)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

		default:
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		}
	}
}

// handleSQLConnectionTest handles POST on /api/sql-connections/{id}/test.
func handleSQLConnectionTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.SQLConnectionsDB == nil || s.SQLConnectionPool == nil {
			http.Error(w, `{"error":"SQL connections not initialized"}`, http.StatusServiceUnavailable)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// Extract ID from /api/sql-connections/{id}/test
		path := strings.TrimPrefix(r.URL.Path, "/api/sql-connections/")
		id := strings.TrimSuffix(path, "/test")
		if id == "" {
			http.Error(w, `{"error":"missing connection id"}`, http.StatusBadRequest)
			return
		}

		rec, err := sqlconnections.GetByID(s.SQLConnectionsDB, id)
		if err != nil {
			jsonError(w, "SQL connection not found", http.StatusNotFound)
			return
		}

		if err := s.SQLConnectionPool.TestConnection(rec); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Connection test failed",
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

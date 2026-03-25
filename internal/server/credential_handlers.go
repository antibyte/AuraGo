package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"aurago/internal/credentials"
	"aurago/internal/security"

	"github.com/google/uuid"
)

type credentialRequest struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	Host            string `json:"host"`
	Username        string `json:"username"`
	Description     string `json:"description"`
	CertificateMode string `json:"certificate_mode"`
	Password        string `json:"password"`
	CertificateText string `json:"certificate_text"`
	Token           string `json:"token"`
	AllowPython     bool   `json:"allow_python"`
}

func handleListCredentials(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			http.Error(w, `{"error":"inventory database not configured"}`, http.StatusServiceUnavailable)
			return
		}

		items, err := credentials.List(s.InventoryDB)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		if items == nil {
			items = []credentials.Record{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	}
}

func handleGetCredential(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			http.Error(w, `{"error":"inventory database not configured"}`, http.StatusServiceUnavailable)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/credentials/")
		if id == "" {
			http.Error(w, `{"error":"credential id required"}`, http.StatusBadRequest)
			return
		}

		item, err := credentials.GetByID(s.InventoryDB, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, `{"error":"credential not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(item)
	}
}

func handleCreateCredential(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			http.Error(w, `{"error":"inventory database not configured"}`, http.StatusServiceUnavailable)
			return
		}
		if s.Vault == nil {
			http.Error(w, `{"error":"vault not configured"}`, http.StatusServiceUnavailable)
			return
		}

		var req credentialRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}

		rec := credentials.Record{
			Name:            strings.TrimSpace(req.Name),
			Type:            strings.TrimSpace(req.Type),
			Host:            strings.TrimSpace(req.Host),
			Username:        strings.TrimSpace(req.Username),
			Description:     strings.TrimSpace(req.Description),
			CertificateMode: strings.TrimSpace(req.CertificateMode),
			AllowPython:     req.AllowPython,
		}
		if err := validateCredentialRequest(rec); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}

		password := strings.TrimSpace(req.Password)
		certificate := strings.TrimSpace(req.CertificateText)
		token := strings.TrimSpace(req.Token)
		if password != "" {
			rec.PasswordVaultID = "credential_password_" + uuid.NewString()
			security.RegisterSensitive(password)
			if err := s.Vault.WriteSecret(rec.PasswordVaultID, password); err != nil {
				http.Error(w, `{"error":"failed to store password in vault"}`, http.StatusInternalServerError)
				return
			}
		}
		if certificate != "" {
			rec.CertificateVaultID = "credential_certificate_" + uuid.NewString()
			security.RegisterSensitive(certificate)
			if err := s.Vault.WriteSecret(rec.CertificateVaultID, certificate); err != nil {
				if rec.PasswordVaultID != "" {
					_ = s.Vault.DeleteSecret(rec.PasswordVaultID)
				}
				http.Error(w, `{"error":"failed to store certificate in vault"}`, http.StatusInternalServerError)
				return
			}
		}
		if token != "" {
			rec.TokenVaultID = "credential_token_" + uuid.NewString()
			security.RegisterSensitive(token)
			if err := s.Vault.WriteSecret(rec.TokenVaultID, token); err != nil {
				if rec.PasswordVaultID != "" {
					_ = s.Vault.DeleteSecret(rec.PasswordVaultID)
				}
				if rec.CertificateVaultID != "" {
					_ = s.Vault.DeleteSecret(rec.CertificateVaultID)
				}
				http.Error(w, `{"error":"failed to store token in vault"}`, http.StatusInternalServerError)
				return
			}
		}

		id, err := credentials.Create(s.InventoryDB, rec)
		if err != nil {
			if rec.PasswordVaultID != "" {
				_ = s.Vault.DeleteSecret(rec.PasswordVaultID)
			}
			if rec.CertificateVaultID != "" {
				_ = s.Vault.DeleteSecret(rec.CertificateVaultID)
			}
			if rec.TokenVaultID != "" {
				_ = s.Vault.DeleteSecret(rec.TokenVaultID)
			}
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": id})
	}
}

func handleUpdateCredential(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			http.Error(w, `{"error":"inventory database not configured"}`, http.StatusServiceUnavailable)
			return
		}
		if s.Vault == nil {
			http.Error(w, `{"error":"vault not configured"}`, http.StatusServiceUnavailable)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/credentials/")
		if id == "" {
			http.Error(w, `{"error":"credential id required"}`, http.StatusBadRequest)
			return
		}

		existing, err := credentials.GetByID(s.InventoryDB, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, `{"error":"credential not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			}
			return
		}

		var req credentialRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}

		existing.Name = strings.TrimSpace(req.Name)
		existing.Type = strings.TrimSpace(req.Type)
		existing.Host = strings.TrimSpace(req.Host)
		existing.Username = strings.TrimSpace(req.Username)
		existing.Description = strings.TrimSpace(req.Description)
		existing.CertificateMode = strings.TrimSpace(req.CertificateMode)
		existing.AllowPython = req.AllowPython
		if err := validateCredentialRequest(existing); err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
			return
		}

		if password := strings.TrimSpace(req.Password); password != "" {
			if existing.PasswordVaultID == "" {
				existing.PasswordVaultID = "credential_password_" + uuid.NewString()
			}
			security.RegisterSensitive(password)
			if err := s.Vault.WriteSecret(existing.PasswordVaultID, password); err != nil {
				http.Error(w, `{"error":"failed to store password in vault"}`, http.StatusInternalServerError)
				return
			}
		}

		if certificate := strings.TrimSpace(req.CertificateText); certificate != "" {
			if existing.CertificateVaultID == "" {
				existing.CertificateVaultID = "credential_certificate_" + uuid.NewString()
			}
			security.RegisterSensitive(certificate)
			if err := s.Vault.WriteSecret(existing.CertificateVaultID, certificate); err != nil {
				http.Error(w, `{"error":"failed to store certificate in vault"}`, http.StatusInternalServerError)
				return
			}
		}

		if token := strings.TrimSpace(req.Token); token != "" {
			if existing.TokenVaultID == "" {
				existing.TokenVaultID = "credential_token_" + uuid.NewString()
			}
			security.RegisterSensitive(token)
			if err := s.Vault.WriteSecret(existing.TokenVaultID, token); err != nil {
				http.Error(w, `{"error":"failed to store token in vault"}`, http.StatusInternalServerError)
				return
			}
		}

		if err := credentials.Update(s.InventoryDB, existing); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, `{"error":"credential not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleDeleteCredential(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			http.Error(w, `{"error":"inventory database not configured"}`, http.StatusServiceUnavailable)
			return
		}

		id := strings.TrimPrefix(r.URL.Path, "/api/credentials/")
		if id == "" {
			http.Error(w, `{"error":"credential id required"}`, http.StatusBadRequest)
			return
		}

		item, err := credentials.GetByID(s.InventoryDB, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, `{"error":"credential not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			}
			return
		}

		if err := credentials.Delete(s.InventoryDB, id); err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, `{"error":"credential not found"}`, http.StatusNotFound)
			} else {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			}
			return
		}

		if s.Vault != nil {
			if item.PasswordVaultID != "" {
				_ = s.Vault.DeleteSecret(item.PasswordVaultID)
			}
			if item.CertificateVaultID != "" {
				_ = s.Vault.DeleteSecret(item.CertificateVaultID)
			}
			if item.TokenVaultID != "" {
				_ = s.Vault.DeleteSecret(item.TokenVaultID)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func validateCredentialRequest(rec credentials.Record) error {
	if strings.TrimSpace(rec.Name) == "" {
		return fmt.Errorf("name is required")
	}
	typ := strings.ToLower(strings.TrimSpace(rec.Type))
	if typ == "" {
		typ = "ssh"
	}
	if !credentials.ValidCredentialTypes[typ] {
		return fmt.Errorf("unsupported credential type")
	}
	if typ == "ssh" && strings.TrimSpace(rec.Host) == "" {
		return fmt.Errorf("host is required for SSH credentials")
	}
	if strings.TrimSpace(rec.Username) == "" {
		return fmt.Errorf("username is required")
	}
	return nil
}

func handleListPythonAccessibleCredentials(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.InventoryDB == nil {
			http.Error(w, `{"error":"inventory database not configured"}`, http.StatusServiceUnavailable)
			return
		}

		items, err := credentials.ListPythonAccessible(s.InventoryDB)
		if err != nil {
			http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusInternalServerError)
			return
		}
		if items == nil {
			items = []credentials.Record{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	}
}

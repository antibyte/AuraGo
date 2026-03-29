package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// handleCertStatus returns info about the current TLS certificate configuration.
func handleCertStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		httpsCfg := s.Cfg.Server.HTTPS
		result := map[string]interface{}{
			"enabled":   httpsCfg.Enabled,
			"cert_mode": httpsCfg.CertMode,
			"domain":    httpsCfg.Domain,
		}

		// Check for certificate files
		certDir := filepath.Join(s.Cfg.Directories.DataDir, "certs")

		switch httpsCfg.CertMode {
		case "custom":
			if httpsCfg.CertFile != "" {
				info, err := GetCertInfo(httpsCfg.CertFile)
				if err == nil {
					result["cert_info"] = info
				} else {
					result["cert_error"] = "Failed to read certificate info"
				}
			}
		case "selfsigned":
			certFile := filepath.Join(certDir, "selfsigned.crt")
			info, err := GetCertInfo(certFile)
			if err == nil {
				result["cert_info"] = info
			} else {
				result["cert_info"] = map[string]string{"status": "not_generated"}
			}
		case "auto":
			result["cert_dir"] = certDir
			result["email"] = httpsCfg.Email
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// handleCertRegenerate regenerates the self-signed certificate.
func handleCertRegenerate(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		domain := s.Cfg.Server.HTTPS.Domain
		if err := RegenerateSelfSignedCert(s.Cfg.Directories.DataDir, domain, s.Logger); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to regenerate certificate"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"message": "Self-signed certificate regenerated. Restart AuraGo to use the new certificate.",
		})
	}
}

// handleCertUpload handles PEM certificate file uploads (cert + key).
func handleCertUpload(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Limit upload size to 1MB
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, "File too large (max 1MB)", http.StatusRequestEntityTooLarge)
			return
		}

		certDir := filepath.Join(s.Cfg.Directories.DataDir, "certs")
		if err := os.MkdirAll(certDir, 0o750); err != nil {
			http.Error(w, "Failed to create cert directory", http.StatusInternalServerError)
			return
		}

		uploaded := map[string]string{}

		// Process cert file
		if certFile, certHeader, err := r.FormFile("cert"); err == nil {
			defer certFile.Close()
			if err := saveUploadedCert(certFile, certHeader.Filename, filepath.Join(certDir, "custom.crt")); err != nil {
				http.Error(w, "Failed to save certificate", http.StatusBadRequest)
				return
			}
			uploaded["cert"] = filepath.Join(certDir, "custom.crt")
		}

		// Process key file
		if keyFile, keyHeader, err := r.FormFile("key"); err == nil {
			defer keyFile.Close()
			if err := saveUploadedCert(keyFile, keyHeader.Filename, filepath.Join(certDir, "custom.key")); err != nil {
				http.Error(w, "Failed to save key", http.StatusBadRequest)
				return
			}
			// Restrict key file permissions
			os.Chmod(filepath.Join(certDir, "custom.key"), 0o600)
			uploaded["key"] = filepath.Join(certDir, "custom.key")
		}

		if len(uploaded) == 0 {
			http.Error(w, "No files uploaded. Send 'cert' and/or 'key' form fields.", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "ok",
			"uploaded": uploaded,
			"message":  "Certificate files uploaded. Set cert_mode to 'custom' and update cert_file/key_file paths in config, then restart.",
		})
	}
}

// saveUploadedCert validates and saves an uploaded PEM file.
func saveUploadedCert(src io.Reader, filename, destPath string) error {
	// Basic filename validation
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".pem" && ext != ".crt" && ext != ".key" && ext != ".cer" {
		return fmt.Errorf("unsupported file extension: %s (expected .pem, .crt, .key, or .cer)", ext)
	}

	data, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Basic PEM validation
	content := string(data)
	if !strings.Contains(content, "-----BEGIN") {
		return fmt.Errorf("file does not appear to be a valid PEM file")
	}

	return os.WriteFile(destPath, data, 0o644)
}

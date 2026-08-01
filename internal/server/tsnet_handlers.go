package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/security"
	"aurago/internal/tsnetnode"
)

// handleTsNetStatus returns the current status of the tsnet embedded node.
func handleTsNetStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if s.TsNetManager == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled": false,
				"running": false,
			})
			return
		}

		status := s.TsNetManager.GetStatus()
		host := ""
		if len(status.CertDNS) > 0 {
			host = status.CertDNS[0]
		} else if status.DNS != "" {
			host = status.DNS
		}
		host = strings.TrimSuffix(host, ".")
		webUIURL := ""
		homepageURL := ""
		manifestURL := ""
		spaceAgentURL := ""
		publicURL := ""
		if host != "" && status.ServingHTTP {
			scheme := "https"
			if status.HTTPFallback {
				scheme = "http"
			}
			webUIURL = fmt.Sprintf("%s://%s", scheme, host)
			if status.FunnelActive {
				publicURL = "https://" + host
			}
		}
		if host != "" && status.HomepageServing {
			homepageURL = fmt.Sprintf("https://%s:8443", host)
		}
		manifestHost := strings.TrimSuffix(status.ManifestDNS, ".")
		if manifestHost == "" {
			manifestHost = host
		}
		if manifestHost != "" && status.ManifestServing {
			port := tsnetCfgManifestPort(s)
			manifestURL = formatManifestTailscaleURL(manifestHost, port)
		}
		spaceAgentHost := strings.TrimSuffix(status.SpaceAgentDNS, ".")
		if spaceAgentHost != "" && status.SpaceAgentServing {
			spaceAgentURL = fmt.Sprintf("https://%s", spaceAgentHost)
		}
		s.CfgMu.RLock()
		tsnetCfg := s.Cfg.Tailscale.TsNet
		s.CfgMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":             tsnetCfg.Enabled,
			"serve_http":          tsnetCfg.ServeHTTP,
			"expose_homepage":     tsnetCfg.ExposeHomepage,
			"expose_space_agent":  tsnetCfg.ExposeSpaceAgent,
			"expose_manifest":     tsnetCfg.ExposeManifest,
			"funnel":              tsnetCfg.Funnel,
			"ready":               status.Ready,
			"running":             status.Running,
			"starting":            status.Starting,
			"serving_http":        status.ServingHTTP,
			"homepage_serving":    status.HomepageServing,
			"manifest_serving":    status.ManifestServing,
			"space_agent_serving": status.SpaceAgentServing,
			"http_fallback":       status.HTTPFallback,
			"funnel_active":       status.FunnelActive,
			"hostname":            status.Hostname,
			"dns":                 status.DNS,
			"ips":                 status.IPs,
			"cert_dns":            status.CertDNS,
			"web_ui_url":          webUIURL,
			"homepage_url":        homepageURL,
			"manifest_url":        manifestURL,
			"manifest_dns":        status.ManifestDNS,
			"space_agent_dns":     status.SpaceAgentDNS,
			"space_agent_url":     spaceAgentURL,
			"public_url":          publicURL,
			"error":               status.Error,
			"login_url":           status.LoginURL,
			"nodes":               status.Nodes,
			"operation":           status.Operation,
		})
	}
}

func tsnetCfgManifestPort(s *Server) int {
	if s == nil || s.Cfg == nil {
		return defaultManifestTailscalePort
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	return effectiveManifestTailscalePort(s.Cfg.Tailscale.TsNet.ManifestPort)
}

// handleTsNetStart (re)starts the tsnet node.
func handleTsNetStart(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := decodeOptionalEmptyTsNetJSON(w, r); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		if s.TsNetManager == nil {
			jsonError(w, "tsnet not initialized", http.StatusServiceUnavailable)
			return
		}

		s.CfgMu.RLock()
		enabled := s.Cfg != nil && s.Cfg.Tailscale.TsNet.Enabled
		s.CfgMu.RUnlock()
		if !enabled {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "Enable tsnet in config first",
			})
			return
		}

		handler := s.tsNetHandler
		if handler == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "error",
				"message": "tsnet handler not ready — restart AuraGo to initialize",
			})
			return
		}

		var (
			operationID string
			err         error
		)
		if st := s.TsNetManager.GetStatus(); st.Running {
			operationID, err = s.TsNetManager.BeginReconfigure(handler)
		} else {
			operationID, err = s.TsNetManager.BeginStart(handler)
		}
		if err != nil {
			code := http.StatusInternalServerError
			if strings.Contains(err.Error(), tsnetnode.ErrorOperationConflict) {
				code = http.StatusConflict
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(map[string]string{
				"status":     "error",
				"error_code": tsnetPublicErrorCode(err),
				"message":    tsnetPublicErrorMessage(err),
			})
			return
		}

		go reconcileDesktopStoreAfterTsNetOperation(s, operationID)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "starting", "operation_id": operationID})
	}
}

func reconcileDesktopStoreAfterTsNetOperation(s *Server, operationID string) {
	if s == nil || s.TsNetManager == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := s.TsNetManager.WaitOperation(ctx, operationID); err != nil {
		s.Logger.Error("[tsnet] operation failed",
			"operation_id", operationID,
			"error_code", tsnetPublicErrorCode(err),
			"error", tsnetPublicErrorMessage(err))
		return
	}
	if err := s.reconcileDesktopStoreTailscale(ctx); err != nil {
		s.Logger.Warn("[tsnet] desktop store proxy reconcile failed",
			"error_code", tsnetPublicErrorCode(err),
			"error", tsnetPublicErrorMessage(err))
	}
}

type tsnetCredentialRequest struct {
	Node    tsnetnode.NodeID `json:"node"`
	AuthKey string           `json:"auth_key"`
}

type tsnetReauthRequest struct {
	Node               tsnetnode.NodeID `json:"node"`
	Mode               string           `json:"mode"`
	ConfirmNewIdentity bool             `json:"confirm_new_identity"`
}

func handleTsNetCredentials(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		behindProxy := false
		if s.Cfg != nil {
			s.CfgMu.RLock()
			behindProxy = s.Cfg.Server.HTTPS.BehindProxy
			s.CfgMu.RUnlock()
		}
		if !vaultAllowRequest(r, behindProxy) {
			jsonError(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		if s.Vault == nil {
			jsonError(w, "Vault not initialized (master key missing)", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodPost:
			var req tsnetCredentialRequest
			if err := decodeStrictTsNetJSON(w, r, &req); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			req.AuthKey = strings.TrimSpace(req.AuthKey)
			security.RegisterSensitive(req.AuthKey)
			if !tsnetnode.IsValidNodeID(req.Node) {
				jsonError(w, "Invalid tsnet node", http.StatusBadRequest)
				return
			}
			if err := validateTsNetAuthKey(req.AuthKey); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			vaultKey := tsnetNodeVaultKey(req.Node)
			if err := s.Vault.WriteSecret(vaultKey, req.AuthKey); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to store tsnet credential", "[tsnet] Failed to store node credential", err, "node", req.Node)
				return
			}
			setTsNetNodeCredential(s, req.Node, req.AuthKey)
			if s.TsNetManager != nil {
				s.TsNetManager.MarkCredentialChanged(req.Node)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "saved", "node": req.Node, "credential_changed": true})
		case http.MethodDelete:
			if err := decodeOptionalEmptyTsNetJSON(w, r); err != nil {
				jsonError(w, err.Error(), http.StatusBadRequest)
				return
			}
			node := tsnetnode.NodeID(strings.TrimSpace(r.URL.Query().Get("node")))
			if !tsnetnode.IsValidNodeID(node) {
				jsonError(w, "Invalid tsnet node", http.StatusBadRequest)
				return
			}
			if err := s.Vault.DeleteSecret(tsnetNodeVaultKey(node)); err != nil {
				jsonLoggedError(w, s.Logger, http.StatusInternalServerError, "Failed to delete tsnet credential", "[tsnet] Failed to delete node credential", err, "node", node)
				return
			}
			setTsNetNodeCredential(s, node, "")
			if s.TsNetManager != nil {
				s.TsNetManager.MarkCredentialChanged(node)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "deleted", "node": node, "credential_changed": true})
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleTsNetReauth(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.TsNetManager == nil {
			jsonError(w, "tsnet not initialized", http.StatusServiceUnavailable)
			return
		}
		var req tsnetReauthRequest
		if err := decodeStrictTsNetJSON(w, r, &req); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !tsnetnode.IsValidNodeID(req.Node) {
			jsonError(w, "Invalid tsnet node", http.StatusBadRequest)
			return
		}
		if req.Mode == "" {
			req.Mode = "normal"
		}
		operationID, err := s.TsNetManager.BeginReauthenticate(req.Node, req.Mode, req.ConfirmNewIdentity, s.tsNetHandler)
		if err != nil {
			code := http.StatusBadRequest
			if strings.Contains(err.Error(), tsnetnode.ErrorOperationConflict) ||
				strings.Contains(err.Error(), tsnetnode.ErrorNodeNotConfigured) {
				code = http.StatusConflict
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(map[string]string{
				"status":     "error",
				"error_code": tsnetPublicErrorCode(err),
				"message":    tsnetPublicErrorMessage(err),
			})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted", "operation_id": operationID})
	}
}

func decodeStrictTsNetJSON(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("Invalid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("Invalid JSON: exactly one object is required")
	}
	return nil
}

func decodeOptionalEmptyTsNetJSON(w http.ResponseWriter, r *http.Request) error {
	if r.Body == nil || r.Body == http.NoBody {
		return nil
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var empty struct{}
	if err := decoder.Decode(&empty); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("Invalid JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("Invalid JSON: exactly one empty object is allowed")
	}
	return nil
}

func validateTsNetAuthKey(key string) error {
	if len(key) < 24 || len(key) > 512 {
		return fmt.Errorf("Auth key has an invalid length")
	}
	if !strings.HasPrefix(key, "tskey-auth-") {
		return fmt.Errorf("Auth key must start with tskey-auth-")
	}
	for _, char := range key {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return fmt.Errorf("Auth key contains invalid characters")
	}
	return nil
}

func tsnetNodeVaultKey(node tsnetnode.NodeID) string {
	switch node {
	case tsnetnode.NodeManifest:
		return "tailscale_tsnet_authkey_manifest"
	case tsnetnode.NodeSpaceAgent:
		return "tailscale_tsnet_authkey_space_agent"
	default:
		return "tailscale_tsnet_authkey_main"
	}
}

func isTsNetAuthVaultKey(key string) bool {
	switch canonicalVaultSecretKey(strings.TrimSpace(key)) {
	case "tailscale_tsnet_authkey",
		"tailscale_tsnet_authkey_main",
		"tailscale_tsnet_authkey_manifest",
		"tailscale_tsnet_authkey_space_agent":
		return true
	default:
		return false
	}
}

func setTsNetNodeCredential(s *Server, node tsnetnode.NodeID, value string) {
	if s == nil || s.Cfg == nil {
		return
	}
	s.CfgMu.Lock()
	switch node {
	case tsnetnode.NodeMain:
		s.Cfg.Tailscale.TsNet.AuthKeyMain = value
	case tsnetnode.NodeManifest:
		s.Cfg.Tailscale.TsNet.AuthKeyManifest = value
	case tsnetnode.NodeSpaceAgent:
		s.Cfg.Tailscale.TsNet.AuthKeySpaceAgent = value
	}
	if s.TsNetManager != nil {
		s.TsNetManager.UpdateConfig(s.Cfg)
	}
	s.CfgMu.Unlock()
}

func applyTsNetCredentialMutation(s *Server, vaultKey, value string) {
	if s == nil || s.Cfg == nil {
		return
	}
	vaultKey = canonicalVaultSecretKey(strings.TrimSpace(vaultKey))
	var changedNodes []tsnetnode.NodeID
	s.CfgMu.Lock()
	switch vaultKey {
	case "tailscale_tsnet_authkey":
		s.Cfg.Tailscale.TsNet.AuthKey = value
		changedNodes = []tsnetnode.NodeID{tsnetnode.NodeMain, tsnetnode.NodeManifest, tsnetnode.NodeSpaceAgent}
	case "tailscale_tsnet_authkey_main":
		s.Cfg.Tailscale.TsNet.AuthKeyMain = value
		changedNodes = []tsnetnode.NodeID{tsnetnode.NodeMain}
	case "tailscale_tsnet_authkey_manifest":
		s.Cfg.Tailscale.TsNet.AuthKeyManifest = value
		changedNodes = []tsnetnode.NodeID{tsnetnode.NodeManifest}
	case "tailscale_tsnet_authkey_space_agent":
		s.Cfg.Tailscale.TsNet.AuthKeySpaceAgent = value
		changedNodes = []tsnetnode.NodeID{tsnetnode.NodeSpaceAgent}
	}
	if len(changedNodes) > 0 && s.TsNetManager != nil {
		s.TsNetManager.UpdateConfig(s.Cfg)
	}
	s.CfgMu.Unlock()
	if len(changedNodes) == 0 || s.TsNetManager == nil {
		return
	}
	for _, node := range changedNodes {
		s.TsNetManager.MarkCredentialChanged(node)
	}
}

func tsnetPublicErrorCode(err error) string {
	for _, code := range []string{
		tsnetnode.ErrorOperationConflict,
		tsnetnode.ErrorNodeNotConfigured,
		tsnetnode.ErrorAuthKeyMissing,
		tsnetnode.ErrorAuthKeyRejected,
		tsnetnode.ErrorLoginRequired,
		tsnetnode.ErrorNodeKeyExpired,
		tsnetnode.ErrorStateCorrupt,
		tsnetnode.ErrorCertUnavailable,
		tsnetnode.ErrorFunnelUnavailable,
		tsnetnode.ErrorTimeout,
	} {
		if strings.Contains(err.Error(), code) {
			return code
		}
	}
	return tsnetnode.ErrorBackendUnavailable
}

func tsnetPublicErrorMessage(err error) string {
	switch tsnetPublicErrorCode(err) {
	case tsnetnode.ErrorOperationConflict:
		return "Another tsnet operation is already running"
	case tsnetnode.ErrorNodeNotConfigured:
		return "The selected tsnet node is not enabled or is not fully configured"
	case tsnetnode.ErrorAuthKeyMissing:
		return "A Tailscale auth key is required for this node"
	case tsnetnode.ErrorAuthKeyRejected:
		return "Tailscale rejected the stored auth key"
	case tsnetnode.ErrorNodeKeyExpired:
		return "The Tailscale node key has expired"
	case tsnetnode.ErrorStateCorrupt:
		return "State recovery is not available or the state could not be recovered safely"
	case tsnetnode.ErrorTimeout:
		return "The tsnet operation timed out"
	default:
		return "The tsnet operation could not be completed"
	}
}

// handleTsNetStop stops the tsnet node.
func handleTsNetStop(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := decodeOptionalEmptyTsNetJSON(w, r); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		if s.TsNetManager == nil {
			jsonError(w, "tsnet not initialized", http.StatusServiceUnavailable)
			return
		}

		if err := s.TsNetManager.Stop(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			s.Logger.Error("Failed to stop tsnet node",
				"error_code", tsnetPublicErrorCode(err),
				"error", tsnetPublicErrorMessage(err))
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to stop tsnet"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
	}
}

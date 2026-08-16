package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/virtualcomputers"
)

// In-memory single-use storage switch authorizations bound to a target identity hash.
var virtualComputersStorageSwitchTokens = struct {
	sync.Mutex
	byToken map[string]virtualComputersStorageSwitchToken
}{byToken: map[string]virtualComputersStorageSwitchToken{}}

type virtualComputersStorageSwitchToken struct {
	Token      string
	TargetHash string
	SessionKey string
	ExpiresAt  time.Time
	Migrate    bool
	Used       bool
}

type virtualComputersStorageSwitchRequest struct {
	// ConfirmWithoutMigration authorizes switching identity while retaining source objects
	// and marking ledger volumes as previous_store. Full object migration is not implemented
	// in this path; clients that need migration must copy externally first.
	ConfirmWithoutMigration bool   `json:"confirm_without_migration"`
	TargetHash              string `json:"target_hash"`
	// Optional proposed target fields when the client has not saved yet.
	Mode             string `json:"mode,omitempty"`
	Endpoint         string `json:"endpoint,omitempty"`
	Bucket           string `json:"bucket,omitempty"`
	Region           string `json:"region,omitempty"`
	UseSSL           *bool  `json:"use_ssl,omitempty"`
	ControlPlaneMode string `json:"control_plane_mode,omitempty"`
	ControlPlaneHost string `json:"control_plane_host,omitempty"`
	InstallDir       string `json:"install_dir,omitempty"`
}

func registerVirtualComputersStorageSwitchRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/virtual-computers/storage/switch/preview", handleVirtualComputersStorageSwitchPreview(s))
	mux.HandleFunc("/api/virtual-computers/storage/switch/authorize", handleVirtualComputersStorageSwitchAuthorize(s))
}

func handleVirtualComputersStorageSwitchPreview(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req virtualComputersStorageSwitchRequest
		if err := decodeOptionalJSON(r, &req); err != nil {
			jsonError(w, "invalid request", http.StatusBadRequest)
			return
		}
		current, target, volumeCount, err := virtualComputersStorageSwitchContext(s, req)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]interface{}{
			"status":              "ok",
			"identity_changed":    current.Hash() != target.Hash(),
			"current_hash":        current.Hash(),
			"target_hash":         target.Hash(),
			"available_volumes":   volumeCount,
			"requires_token":      current.Hash() != target.Hash() && volumeCount > 0,
			"migration_supported": false,
			"message":             "Object migration is not automated yet. Authorize switch-without-migration to keep source objects and mark volumes previous_store.",
		})
	}
}

func handleVirtualComputersStorageSwitchAuthorize(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req virtualComputersStorageSwitchRequest
		if err := decodeOptionalJSON(r, &req); err != nil {
			jsonError(w, "invalid request", http.StatusBadRequest)
			return
		}
		if !req.ConfirmWithoutMigration {
			jsonError(w, "confirm_without_migration is required (automated migration is not available yet)", http.StatusBadRequest)
			return
		}
		current, target, volumeCount, err := virtualComputersStorageSwitchContext(s, req)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		if current.Hash() == target.Hash() {
			writeJSON(w, map[string]interface{}{"status": "ok", "token": "", "required": false})
			return
		}
		if volumeCount == 0 {
			writeJSON(w, map[string]interface{}{"status": "ok", "token": "", "required": false, "available_volumes": 0})
			return
		}
		token, err := virtualComputersIssueStorageSwitchToken(target.Hash(), virtualComputersSessionKey(r), false)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{
			"status":            "ok",
			"token":             token,
			"required":          true,
			"target_hash":       target.Hash(),
			"available_volumes": volumeCount,
			"expires_in_sec":    600,
			"message":           "Include X-AuraGo-Storage-Switch-Token on the next config save that applies this storage identity.",
		})
	}
}

func virtualComputersStorageSwitchContext(s *Server, req virtualComputersStorageSwitchRequest) (current, target virtualcomputers.StorageIdentity, volumeCount int, err error) {
	if s == nil || s.Cfg == nil {
		return current, target, 0, fmt.Errorf("server config unavailable")
	}
	s.CfgMu.RLock()
	vc := s.Cfg.VirtualComputers
	s.CfgMu.RUnlock()
	current = virtualcomputers.StorageIdentityFromConfig(vc)
	target = current
	if strings.TrimSpace(req.Mode) != "" || strings.TrimSpace(req.Endpoint) != "" || strings.TrimSpace(req.TargetHash) != "" {
		proposed := vc
		if m := strings.TrimSpace(req.Mode); m != "" {
			proposed.Storage.Mode = m
		}
		if ep := strings.TrimSpace(req.Endpoint); ep != "" {
			proposed.Storage.Endpoint = ep
		}
		if b := strings.TrimSpace(req.Bucket); b != "" {
			proposed.Storage.Bucket = b
		}
		if r := strings.TrimSpace(req.Region); r != "" {
			proposed.Storage.Region = r
		}
		if req.UseSSL != nil {
			proposed.Storage.UseSSL = *req.UseSSL
		}
		if m := strings.TrimSpace(req.ControlPlaneMode); m != "" {
			proposed.ControlPlane.Mode = m
		}
		if h := strings.TrimSpace(req.ControlPlaneHost); h != "" {
			proposed.ControlPlane.Host = h
		}
		if d := strings.TrimSpace(req.InstallDir); d != "" {
			proposed.ControlPlane.InstallDir = d
		}
		target = virtualcomputers.StorageIdentityFromConfig(proposed)
	}
	if th := strings.TrimSpace(req.TargetHash); th != "" && th != target.Hash() {
		return current, target, 0, fmt.Errorf("target_hash does not match the proposed storage identity")
	}
	volumeCount, err = virtualComputersCountAvailableVolumes(s)
	if err != nil {
		return current, target, 0, err
	}
	return current, target, volumeCount, nil
}

func virtualComputersCountAvailableVolumes(s *Server) (int, error) {
	if s == nil || s.Cfg == nil {
		return 0, nil
	}
	path := strings.TrimSpace(s.Cfg.SQLite.VirtualComputersPath)
	if path == "" {
		return 0, nil
	}
	ledger, err := virtualcomputers.OpenLedger(path)
	if err != nil {
		return 0, err
	}
	defer ledger.Close()
	return ledger.CountAvailableVolumes(context.Background())
}

func virtualComputersIssueStorageSwitchToken(targetHash, sessionKey string, migrate bool) (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate storage switch token: %w", err)
	}
	token := "vcs_" + hex.EncodeToString(raw)
	virtualComputersStorageSwitchTokens.Lock()
	defer virtualComputersStorageSwitchTokens.Unlock()
	// Drop expired entries opportunistically.
	now := time.Now()
	for k, v := range virtualComputersStorageSwitchTokens.byToken {
		if now.After(v.ExpiresAt) || v.Used {
			delete(virtualComputersStorageSwitchTokens.byToken, k)
		}
	}
	virtualComputersStorageSwitchTokens.byToken[token] = virtualComputersStorageSwitchToken{
		Token:      token,
		TargetHash: strings.TrimSpace(targetHash),
		SessionKey: sessionKey,
		ExpiresAt:  now.Add(10 * time.Minute),
		Migrate:    migrate,
	}
	return token, nil
}

func virtualComputersSessionKey(r *http.Request) string {
	if r == nil {
		return ""
	}
	if c, err := r.Cookie("aurago_session"); err == nil && c != nil {
		return strings.TrimSpace(c.Value)
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return "bearer:" + strings.TrimSpace(auth[7:])
	}
	return strings.TrimSpace(r.RemoteAddr)
}

// virtualComputersConsumeStorageSwitchToken validates and burns a single-use token for targetHash.
func virtualComputersConsumeStorageSwitchToken(token, targetHash, sessionKey string) error {
	token = strings.TrimSpace(token)
	targetHash = strings.TrimSpace(targetHash)
	if token == "" {
		return fmt.Errorf("storage switch token is required")
	}
	virtualComputersStorageSwitchTokens.Lock()
	defer virtualComputersStorageSwitchTokens.Unlock()
	entry, ok := virtualComputersStorageSwitchTokens.byToken[token]
	if !ok {
		return fmt.Errorf("storage switch token is invalid or expired")
	}
	delete(virtualComputersStorageSwitchTokens.byToken, token)
	if entry.Used || time.Now().After(entry.ExpiresAt) {
		return fmt.Errorf("storage switch token is invalid or expired")
	}
	if subtle.ConstantTimeCompare([]byte(entry.TargetHash), []byte(targetHash)) != 1 {
		return fmt.Errorf("storage switch token does not match the target storage identity")
	}
	if entry.SessionKey != "" && sessionKey != "" &&
		subtle.ConstantTimeCompare([]byte(entry.SessionKey), []byte(sessionKey)) != 1 {
		return fmt.Errorf("storage switch token does not match this session")
	}
	if entry.Migrate {
		return fmt.Errorf("migration tokens are not supported yet")
	}
	return nil
}

// virtualComputersEnforceStorageSwitchGate blocks config saves that change storage identity
// while available volumes exist, unless a valid single-use token is presented.
// Returns (blocked message, http status, ok).
func virtualComputersEnforceStorageSwitchGate(s *Server, r *http.Request, oldCfg, newCfg config.Config) (string, int, bool) {
	oldID := virtualcomputers.StorageIdentityFromConfig(oldCfg.VirtualComputers)
	newID := virtualcomputers.StorageIdentityFromConfig(newCfg.VirtualComputers)
	if oldID.Hash() == newID.Hash() {
		return "", 0, true
	}
	count, err := virtualComputersCountAvailableVolumes(s)
	if err != nil {
		return "could not inspect virtual computer volumes for storage switch: " + err.Error(), http.StatusInternalServerError, false
	}
	if count == 0 {
		return "", 0, true
	}
	token := strings.TrimSpace(r.Header.Get("X-AuraGo-Storage-Switch-Token"))
	if token == "" {
		// Also accept body field injected by clients that cannot set headers easily.
		token = strings.TrimSpace(r.Header.Get("X-Storage-Switch-Token"))
	}
	if token == "" {
		return fmt.Sprintf(
			"storage identity change blocked: %d available volume(s) still reference the previous store. "+
				"POST /api/virtual-computers/storage/switch/authorize with confirm_without_migration=true, "+
				"then retry config save with header X-AuraGo-Storage-Switch-Token.",
			count,
		), http.StatusConflict, false
	}
	if err := virtualComputersConsumeStorageSwitchToken(token, newID.Hash(), virtualComputersSessionKey(r)); err != nil {
		return security.Scrub(err.Error()), http.StatusConflict, false
	}
	// Apply previous_store marking before the new config is published.
	if markErr := virtualComputersMarkVolumesPreviousStore(s); markErr != nil && s != nil && s.Logger != nil {
		s.Logger.Warn("[VirtualComputers] mark previous_store volumes failed", "error", markErr)
	}
	// Stop managed Garage when leaving managed mode or changing host identity.
	if oldID.Mode == virtualcomputers.StorageModeManagedGarage {
		go virtualComputersStopManagedGarageIfPresent(s, virtualcomputers.FromAuraConfig(&oldCfg))
	}
	return "", 0, true
}

func virtualComputersMarkVolumesPreviousStore(s *Server) error {
	if s == nil || s.Cfg == nil {
		return nil
	}
	path := strings.TrimSpace(s.Cfg.SQLite.VirtualComputersPath)
	if path == "" {
		return nil
	}
	ledger, err := virtualcomputers.OpenLedger(path)
	if err != nil {
		return err
	}
	defer ledger.Close()
	_, err = ledger.MarkAvailableVolumesPreviousStore(context.Background())
	return err
}

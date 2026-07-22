package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/sipphone"
)

const sipRequestBodyLimit = 128 << 10

type sipConfigPayload struct {
	config.SIPConfig
	Password      string `json:"password,omitempty"`
	PasswordSet   bool   `json:"password_set"`
	ClearPassword bool   `json:"clear_password,omitempty"`
}

type sipRequestLimiter struct {
	mu      sync.Mutex
	windows map[string]*sipRequestWindow
}

type sipRequestWindow struct {
	started time.Time
	count   int
}

func registerSIPHandlers(mux *http.ServeMux, s *Server) {
	limiter := &sipRequestLimiter{windows: make(map[string]*sipRequestWindow)}
	admin := func(handler http.HandlerFunc) http.HandlerFunc {
		guarded := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.allow(r, 60, time.Minute) {
				w.Header().Set("Retry-After", "60")
				jsonError(w, "SIP API rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			handler(w, r)
		})
		return requireAdmin(s, guarded).ServeHTTP
	}
	mux.HandleFunc("/api/sip/config", admin(handleSIPConfig(s)))
	mux.HandleFunc("/api/sip/test", admin(handleSIPTest(s)))
	mux.HandleFunc("/api/sip/status", admin(handleSIPStatus(s)))
	mux.HandleFunc("/api/sip/calls", admin(handleSIPCalls(s)))
	mux.HandleFunc("/api/sip/calls/", admin(handleSIPCallAction(s)))
	mux.HandleFunc("/api/sip/events", admin(handleSIPEvents(s)))
}

func handleSIPConfig(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeSIPConfig(w, sipConfigSnapshot(s))
		case http.MethodPut:
			handlePutSIPConfig(s, w, r)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func sipConfigSnapshot(s *Server) config.SIPConfig {
	if s == nil || s.Cfg == nil {
		var fallback config.SIPConfig
		config.ApplySIPDefaults(&fallback)
		return fallback
	}
	s.CfgMu.RLock()
	snapshot := s.Cfg.SIP
	snapshot.Media.Codecs = append([]string(nil), snapshot.Media.Codecs...)
	snapshot.Inbound.TrustedPeerCIDRs = append([]string(nil), snapshot.Inbound.TrustedPeerCIDRs...)
	snapshot.Inbound.AllowedCallers = append([]string(nil), snapshot.Inbound.AllowedCallers...)
	snapshot.Outbound.AllowedDomains = append([]string(nil), snapshot.Outbound.AllowedDomains...)
	snapshot.Outbound.AllowedUsers = append([]string(nil), snapshot.Outbound.AllowedUsers...)
	snapshot.Outbound.AllowedE164Prefixes = append([]string(nil), snapshot.Outbound.AllowedE164Prefixes...)
	snapshot.Voice.AllowedTools = append([]string(nil), snapshot.Voice.AllowedTools...)
	s.CfgMu.RUnlock()
	return snapshot
}

func writeSIPConfig(w http.ResponseWriter, cfg config.SIPConfig) {
	payload := sipConfigPayload{SIPConfig: cfg, PasswordSet: strings.TrimSpace(cfg.Password) != ""}
	payload.SIPConfig.Password = ""
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(payload)
}

func handlePutSIPConfig(s *Server, w http.ResponseWriter, r *http.Request) {
	if !sameOriginOrNoOrigin(r) {
		jsonError(w, "Request origin does not match server host", http.StatusForbidden)
		return
	}
	var incoming sipConfigPayload
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, sipRequestBodyLimit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&incoming); err != nil {
		jsonError(w, "Invalid SIP configuration JSON", http.StatusBadRequest)
		return
	}
	if incoming.ClearPassword && strings.TrimSpace(incoming.Password) != "" && incoming.Password != maskedKey {
		jsonError(w, "password and clear_password cannot be used together", http.StatusBadRequest)
		return
	}
	old := sipConfigSnapshot(s)
	next := incoming.SIPConfig
	next.Password = old.Password
	mutations := make([]vaultMutation, 0, 1)
	switch {
	case incoming.ClearPassword:
		next.Password = ""
		mutations = append(mutations, vaultMutation{key: config.SIPPasswordVaultKey, delete: true})
	case strings.TrimSpace(incoming.Password) != "" && incoming.Password != maskedKey:
		if s == nil || s.Vault == nil {
			jsonError(w, "Vault is required to store the SIP password", http.StatusServiceUnavailable)
			return
		}
		next.Password = strings.TrimSpace(incoming.Password)
		security.RegisterSensitive(next.Password)
		mutations = append(mutations, vaultMutation{key: config.SIPPasswordVaultKey, value: next.Password})
	}
	config.NormalizeSIPConfig(&next)
	if err := config.ValidateSIPRuntimeConfig(next); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if old.Enabled && next.Enabled && (old.Media.RTPPortStart != next.Media.RTPPortStart || old.Media.RTPPortEnd != next.Media.RTPPortEnd) {
		jsonError(w, "RTP port range changes require an AuraGo restart", http.StatusConflict)
		return
	}
	if err := persistSIPConfig(s, next, mutations); err != nil {
		if s != nil && s.Logger != nil {
			s.Logger.Error("Failed to update SIP configuration", "error", err)
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if s.SIPPhone != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		err := s.SIPPhone.Reconfigure(ctx, next)
		cancel()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "pending", "message": "Configuration saved; SIP runtime reconciliation will require a restart"})
			return
		}
	}
	writeSIPConfig(w, sipConfigSnapshot(s))
}

func persistSIPConfig(s *Server, next config.SIPConfig, mutations []vaultMutation) error {
	if s == nil || s.Cfg == nil {
		return fmt.Errorf("server config is not available")
	}
	s.CfgMu.RLock()
	configPath := s.Cfg.ConfigPath
	s.CfgMu.RUnlock()
	if strings.TrimSpace(configPath) == "" {
		return fmt.Errorf("config path not set")
	}
	s.CfgSaveMu.Lock()
	defer s.CfgSaveMu.Unlock()
	original, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	output, err := marshalConfigWithSIP(original, next)
	if err != nil {
		return err
	}
	if err := config.WriteFileAtomic(configPath, output, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	vaultSnapshots, err := applyVaultMutations(s.Vault, mutations)
	if err != nil {
		_ = config.WriteFileAtomic(configPath, original, 0o600)
		return fmt.Errorf("update SIP secret: %w", err)
	}
	newConfig, err := config.Load(configPath)
	if err == nil {
		newConfig.ConfigPath = configPath
		newConfig.ApplyVaultSecrets(s.Vault)
		err = config.ValidateSIPRuntimeConfig(newConfig.SIP)
	}
	if err != nil {
		_ = config.WriteFileAtomic(configPath, original, 0o600)
		_ = restoreVaultSecrets(s.Vault, vaultSnapshots)
		return fmt.Errorf("reload SIP config: %w", err)
	}
	newConfig.ResolveProviders()
	newConfig.ApplyOAuthTokens(s.Vault)
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(newConfig)
	s.CfgMu.Unlock()
	return nil
}

func handleSIPTest(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		manager := sipManager(s, w)
		if manager == nil {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		if err := manager.TestConnection(ctx); err != nil {
			jsonError(w, "SIP connection test failed", http.StatusBadGateway)
			return
		}
		writeSIPJSON(w, map[string]string{"status": "ok"})
	}
}

func handleSIPStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		manager := sipManager(s, w)
		if manager != nil {
			writeSIPJSON(w, manager.Status())
		}
	}
}

func handleSIPCalls(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		manager := sipManager(s, w)
		if manager == nil {
			return
		}
		switch r.Method {
		case http.MethodGet:
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			calls, err := manager.ListCalls(r.Context(), limit)
			if err != nil {
				jsonError(w, "Failed to list SIP calls", http.StatusInternalServerError)
				return
			}
			writeSIPJSON(w, map[string]interface{}{"calls": calls})
		case http.MethodPost:
			if !sameOriginOrNoOrigin(r) {
				jsonError(w, "Request origin does not match server host", http.StatusForbidden)
				return
			}
			var body struct {
				Target string `json:"target"`
			}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, sipRequestBodyLimit)).Decode(&body); err != nil {
				jsonError(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			call, err := manager.Dial(r.Context(), body.Target)
			if err != nil {
				writeSIPManagerError(w, err)
				return
			}
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(call)
		default:
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleSIPCallAction(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !sameOriginOrNoOrigin(r) {
			jsonError(w, "Request origin does not match server host", http.StatusForbidden)
			return
		}
		manager := sipManager(s, w)
		if manager == nil {
			return
		}
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/sip/calls/"), "/"), "/")
		if len(parts) != 2 || parts[0] == "" {
			jsonError(w, "Invalid SIP call action path", http.StatusNotFound)
			return
		}
		callID, action := parts[0], parts[1]
		var err error
		switch action {
		case "answer":
			err = manager.Answer(callID)
		case "reject":
			err = manager.Reject(callID)
		case "hangup":
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			err = manager.Hangup(ctx, callID)
			cancel()
		case "dtmf":
			var body struct {
				Digit string `json:"digit"`
			}
			if decodeErr := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); decodeErr != nil || len([]rune(body.Digit)) != 1 {
				jsonError(w, "A single DTMF digit is required", http.StatusBadRequest)
				return
			}
			err = manager.SendDTMF(callID, []rune(body.Digit)[0])
		default:
			jsonError(w, "Unknown SIP call action", http.StatusNotFound)
			return
		}
		if err != nil {
			writeSIPManagerError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleSIPEvents(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		manager := sipManager(s, w)
		if manager == nil {
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			jsonError(w, "SSE is not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store")
		w.Header().Set("Connection", "keep-alive")
		events, unsubscribe := manager.Subscribe()
		defer unsubscribe()
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case event := <-events:
				data, _ := json.Marshal(event)
				_, _ = fmt.Fprintf(w, "id: %d\nevent: sip\ndata: %s\n\n", event.Sequence, data)
				flusher.Flush()
			case <-heartbeat.C:
				_, _ = fmt.Fprint(w, ": keepalive\n\n")
				flusher.Flush()
			}
		}
	}
}

func sipManager(s *Server, w http.ResponseWriter) *sipphone.Manager {
	if s == nil || s.SIPPhone == nil {
		jsonError(w, "SIP endpoint is unavailable", http.StatusServiceUnavailable)
		return nil
	}
	return s.SIPPhone
}

func writeSIPJSON(w http.ResponseWriter, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(value)
}

func writeSIPManagerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sipphone.ErrDisabled):
		jsonError(w, err.Error(), http.StatusServiceUnavailable)
	case errors.Is(err, sipphone.ErrReadOnly), errors.Is(err, sipphone.ErrPermissionDenied):
		jsonError(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, sipphone.ErrBusy):
		jsonError(w, err.Error(), http.StatusConflict)
	case errors.Is(err, sipphone.ErrCallNotFound):
		jsonError(w, err.Error(), http.StatusNotFound)
	default:
		jsonError(w, "SIP operation failed", http.StatusBadGateway)
	}
}

func (l *sipRequestLimiter) allow(r *http.Request, maximum int, window time.Duration) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host == "" {
		host = "unknown"
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	record := l.windows[host]
	if record == nil || now.Sub(record.started) >= window {
		l.windows[host] = &sipRequestWindow{started: now, count: 1}
		return true
	}
	if record.count >= maximum {
		return false
	}
	record.count++
	return true
}

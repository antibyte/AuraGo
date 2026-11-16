package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"aurago/internal/config"
	"aurago/internal/desktopstore"
	"aurago/internal/security"
	"aurago/internal/tools"
	"gopkg.in/yaml.v3"
)

type storeIntegrationBinding struct {
	settings desktopstore.IntegrationSettings
	ctx      context.Context
	cancel   context.CancelFunc
}

type storeIntegrationRuntime struct {
	sync.Mutex
	started  bool
	closed   bool
	bindings map[string]storeIntegrationBinding
	failures map[string]string
}

type storeIntegrationUpdate struct {
	Revision               string   `json:"revision"`
	URL                    string   `json:"url"`
	Username               string   `json:"username"`
	Credential             string   `json:"credential"`
	ClearCredential        bool     `json:"clear_credential"`
	SelectStoreInstance    bool     `json:"select_store_instance"`
	Read                   bool     `json:"read"`
	Execute                bool     `json:"execute"`
	Create                 bool     `json:"create"`
	Process                bool     `json:"process"`
	Sign                   bool     `json:"sign"`
	Notify                 *bool    `json:"notify"`
	AllowedIDs             []string `json:"allowed_ids"`
	Source                 string   `json:"source"`
	CertificateFingerprint string   `json:"certificate_fingerprint"`
	AdminPort              string   `json:"admin_port"`
	DNSAddress             string   `json:"dns_address"`
	DockerNetwork          *string  `json:"docker_network"`
	RelayPort              *int     `json:"relay_port"`
}

func (s *Server) revokeStoreIntegration(id string) {
	s.revokeStoreRelay(id, false)
	s.storeIntegrations.Lock()
	defer s.storeIntegrations.Unlock()
	delete(s.storeIntegrations.failures, id)
	if binding, ok := s.storeIntegrations.bindings[id]; ok {
		binding.cancel()
		delete(s.storeIntegrations.bindings, id)
	}
	if id == "uptime-kuma" && s.ConfigSnapshot().UptimeKuma.StoreManaged {
		if poller := s.uptimeKuma.Load(); poller != nil {
			poller.cancel()
		}
	}
}

func (s *Server) revokeStoreIntegrations() bool {
	s.revokeStoreRelay("", false)
	s.storeIntegrations.Lock()
	defer s.storeIntegrations.Unlock()
	for id, binding := range s.storeIntegrations.bindings {
		binding.cancel()
		delete(s.storeIntegrations.bindings, id)
	}
	return s.storeIntegrations.started
}

func (s *Server) acquireStoreAccess(ctx context.Context, id, base string, mutation bool) (context.Context, func(), error) {
	deny := func() (context.Context, func(), error) {
		return ctx, func() {}, errors.New("Store integration is disabled, unverified or has changed")
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled || !cfg.VirtualDesktop.AllowAgentControl || !cfg.Tools.VirtualDesktop.Enabled || !cfg.Agent.AllowNetworkRequests {
		return deny()
	}
	if mutation && cfg.VirtualDesktop.ReadOnly {
		return deny()
	}
	s.storeIntegrations.Lock()
	b, exists := s.storeIntegrations.bindings[id]
	s.storeIntegrations.Unlock()
	if !exists || b.ctx.Err() != nil || b.settings.URL != base || !b.settings.Read || (mutation && !b.settings.Execute) {
		return deny()
	}
	if id == "adguard-home" && (!cfg.AdGuard.Enabled || !cfg.AdGuard.StoreManaged || cfg.AdGuard.URL != base || (mutation && cfg.AdGuard.ReadOnly)) {
		return deny()
	}
	if id == "uptime-kuma" && (!cfg.UptimeKuma.Enabled || !cfg.UptimeKuma.StoreManaged || cfg.UptimeKuma.BaseURL != base) {
		return deny()
	}
	s.DesktopMu.Lock()
	store := s.DesktopStore
	s.DesktopMu.Unlock()
	if store == nil || store.IntegrationIdle(ctx, id) != nil {
		return deny()
	}
	current, err := desktopstore.ReadIntegration(s.Vault, id)
	if err != nil || current.Revision != b.settings.Revision {
		return deny()
	}
	callCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(b.ctx, cancel)
	if b.ctx.Err() != nil {
		cancel()
	}
	return callCtx, func() { stop(); cancel() }, nil
}

func (s *Server) startStoreIntegrations() {
	s.storeIntegrations.Lock()
	s.storeIntegrations.started = true
	s.storeIntegrations.Unlock()
	tools.SetStoreAccess(s.acquireStoreAccess)
	s.refreshStoreIntegrations()
	if s.ShutdownCh != nil {
		go func() { <-s.ShutdownCh; s.stopStoreIntegrations() }()
	}
}

func (s *Server) refreshStoreIntegrations() {
	s.restoreStoreRelay()
	for _, id := range desktopstore.IntegrationApps {
		settings, err := desktopstore.ReadIntegration(s.Vault, id)
		if err != nil || settings.Revision == "" || !settings.Read {
			continue
		}
		ctx, cancel := desktopStoreOperationContext(s.ShutdownCh, 20*time.Second)
		store, err := s.getDesktopStoreService(ctx)
		if err == nil {
			s.refreshStoreIntegration(store, id)
		}
		cancel()
	}
}

func (s *Server) refreshStoreIntegration(store *desktopstore.Service, id string) {
	if !desktopstore.HasIntegration(id) {
		return
	}
	ctx, cancel := desktopStoreOperationContext(s.ShutdownCh, 20*time.Second)
	defer cancel()
	s.CfgSaveMu.Lock()
	defer s.CfgSaveMu.Unlock()
	settings, err := desktopstore.ReadIntegration(s.Vault, id)
	if err != nil || settings.Revision == "" || !settings.Read {
		return
	}
	finish, err := store.IntegrationOperation(ctx, id)
	if err != nil {
		return
	}
	err = s.testStoreIntegration(ctx, store, id, settings)
	finish(err)
	if err == nil && id == "uptime-kuma" {
		s.restartUptimeKumaPoller()
	}
}

func storeLegacyConflict(cfg *config.Config, id string, settings desktopstore.IntegrationSettings) bool {
	switch id {
	case "adguard-home":
		return cfg.AdGuard.URL != "" && (!cfg.AdGuard.StoreManaged || cfg.AdGuard.URL != settings.URL)
	case "uptime-kuma":
		return cfg.UptimeKuma.BaseURL != "" && (!cfg.UptimeKuma.StoreManaged || cfg.UptimeKuma.BaseURL != settings.URL)
	}
	return false
}

func (s *Server) storeIntegrationURL(ctx context.Context, app desktopstore.InstalledApp, settings desktopstore.IntegrationSettings) (string, error) {
	cfg := s.ConfigSnapshot()
	if settings.URL != "" {
		return settings.URL, security.ValidateHTTPBaseURL(settings.URL)
	}
	if !printGuardEngineIsLocal(cfg.Docker.Host, cfg.Runtime.IsDocker) {
		return "", errors.New("Configure an explicit address reachable from AuraGo for this remote Docker host")
	}
	if cfg.Runtime.IsDocker {
		info, err := s.printGuardHost(ctx)
		if err != nil {
			return "", err
		}
		found := false
		for _, network := range info.Networks {
			if network == desktopstore.IntegrationNetwork(app) {
				found = true
			}
		}
		if !found {
			return "", errors.New("Store update required: select a Docker network shared with AuraGo")
		}
	}
	return desktopstore.IntegrationURL(app, settings.AdminPort, cfg.Runtime.IsDocker)
}

func (s *Server) storeIntegrationStatus(ctx context.Context, store *desktopstore.Service, id string) (map[string]any, error) {
	settings, err := desktopstore.ReadIntegration(s.Vault, id)
	if err != nil {
		return nil, err
	}
	app, exists, err := store.GetInstalled(ctx, id)
	if err != nil || !exists {
		return nil, errors.New("Store app is not installed")
	}
	address, addressErr := s.storeIntegrationURL(ctx, app, settings)
	cfg := s.ConfigSnapshot()
	credentialStored := settings.Credential != ""
	if id == "adguard-home" {
		credentialStored = cfg.AdGuard.StoreManaged && cfg.AdGuard.Password != ""
	}
	if id == "uptime-kuma" {
		credentialStored = cfg.UptimeKuma.StoreManaged && cfg.UptimeKuma.APIKey != ""
	}
	state := "setup_required"
	if settings.Revision != "" && !settings.Read {
		state = "disabled"
	}
	s.storeIntegrations.Lock()
	b, bound := s.storeIntegrations.bindings[id]
	failure := s.storeIntegrations.failures[id]
	s.storeIntegrations.Unlock()
	if bound && settings.Read && b.ctx.Err() == nil && b.settings.Revision == settings.Revision {
		state = "connected"
	}
	conflict := storeLegacyConflict(cfg, id, settings)
	if addressErr != nil || conflict {
		state = "error"
	}
	if failure != "" {
		state = "error"
	}
	if settings.Read && (!cfg.VirtualDesktop.Enabled || !cfg.VirtualDesktop.AllowAgentControl || !cfg.Tools.VirtualDesktop.Enabled || !cfg.Agent.AllowNetworkRequests) {
		state = "disabled"
	}
	result := map[string]any{"revision": settings.Revision, "state": state, "url": address, "username": settings.Username, "credential_stored": credentialStored, "conflict": conflict, "read": settings.Read, "execute": settings.Execute, "create": settings.Create, "process": settings.Process, "sign": settings.Sign, "notify": settings.Notify, "allowed_ids": settings.AllowedIDs, "source": settings.Source, "certificate_fingerprint": settings.CertificateFingerprint, "admin_port": settings.AdminPort, "dns_address": settings.DNSAddress}
	result["relay_port"] = cfg.StoreLLMRelay.Port
	if cfg.StoreLLMRelay.Port == 0 {
		result["relay_port"] = storeRelayDefaultPort
	}
	result["docker_network"] = settings.DockerNetwork
	result["containerized"] = cfg.Runtime.IsDocker
	if cfg.Runtime.IsDocker {
		if host, e := s.printGuardHost(ctx); e == nil {
			result["networks"] = host.Networks
		}
	}
	if addressErr != nil {
		result["message"] = addressErr.Error()
	}
	if failure != "" {
		result["message"] = failure
	}
	return result, nil
}

func handleDesktopStoreIntegration(s *Server, id string, suffix []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setDesktopStoreNoCacheHeaders(w)
		if !desktopstore.HasIntegration(id) || len(suffix) > 1 || (len(suffix) == 1 && suffix[0] != "test") {
			jsonError(w, "Unknown integration", 404)
			return
		}
		testing := len(suffix) == 1
		if (testing && r.Method != http.MethodPost) || (!testing && r.Method != http.MethodGet && r.Method != http.MethodPut && r.Method != http.MethodDelete) {
			jsonError(w, "Method not allowed", 405)
			return
		}
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if r.Method != http.MethodGet && rejectDesktopStoreMutationIfDisabled(s, w) {
			return
		}
		store, err := s.getDesktopStoreService(r.Context())
		if err != nil {
			jsonError(w, "Store unavailable", 503)
			return
		}
		if r.Method != http.MethodGet {
			s.CfgSaveMu.Lock()
			defer s.CfgSaveMu.Unlock()
			finish, err := store.IntegrationOperation(r.Context(), id)
			if err != nil {
				writeDesktopStoreStartError(w, err)
				return
			}
			var opErr error
			defer func() { finish(opErr) }()
			s.revokeStoreIntegration(id)
			if testing {
				settings, readErr := desktopstore.ReadIntegration(s.Vault, id)
				opErr = readErr
				if opErr == nil {
					opErr = s.testStoreIntegration(r.Context(), store, id, settings)
				}
			} else {
				var req storeIntegrationUpdate
				if r.Method == http.MethodPut {
					decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
					decoder.DisallowUnknownFields()
					if decoder.Decode(&req) != nil {
						opErr = errors.New("Invalid integration settings")
					}
				}
				if opErr == nil {
					opErr = s.saveStoreIntegration(r.Context(), store, id, req, r.Method == http.MethodDelete)
				}
			}
			if opErr != nil {
				jsonError(w, security.Scrub(opErr.Error()), http.StatusConflict)
				return
			}
		}
		status, err := s.storeIntegrationStatus(r.Context(), store, id)
		if err != nil {
			jsonError(w, err.Error(), 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	}
}

func (s *Server) saveStoreIntegration(ctx context.Context, store *desktopstore.Service, id string, req storeIntegrationUpdate, disconnect bool) error {
	previous, err := desktopstore.ReadIntegration(s.Vault, id)
	if err != nil {
		return err
	}
	if !disconnect && req.Revision != previous.Revision {
		return errors.New("Integration changed; reload the dialog")
	}
	cfg := s.ConfigSnapshot()
	if storeLegacyConflict(cfg, id, previous) && !req.SelectStoreInstance {
		return errors.New("Existing manual integration: explicitly select the Store instance")
	}
	if len(req.Credential) > 4096 || strings.IndexFunc(req.Credential, unicode.IsControl) >= 0 || (req.ClearCredential && req.Credential != "") {
		return errors.New("Invalid credential")
	}
	security.RegisterSensitive(req.Credential)
	next := desktopstore.IntegrationSettings{Revision: fmt.Sprintf("%d", time.Now().UnixNano()), URL: strings.TrimRight(strings.TrimSpace(req.URL), "/"), Username: strings.TrimSpace(req.Username), Credential: previous.Credential, Read: req.Read, Execute: req.Execute, Create: req.Create, Process: req.Process, Sign: req.Sign, Notify: previous.Notify, AllowedIDs: req.AllowedIDs, Source: req.Source, CertificateFingerprint: req.CertificateFingerprint, AdminPort: req.AdminPort, DNSAddress: req.DNSAddress}
	next.DockerNetwork = previous.DockerNetwork
	if req.DockerNetwork != nil {
		next.DockerNetwork = *req.DockerNetwork
		if cfg.Runtime.IsDocker {
			host, err := s.printGuardHost(ctx)
			if err != nil {
				return err
			}
			found := false
			for _, network := range host.Networks {
				if network == next.DockerNetwork {
					found = true
				}
			}
			if !found {
				return errors.New("Select a Docker network shared with AuraGo")
			}
		}
	}
	if !disconnect && req.AdminPort != previous.AdminPort && next.URL == previous.URL {
		next.URL = ""
	}
	if req.Notify != nil {
		next.Notify = *req.Notify
	} else if previous.Revision == "" && id == "uptime-kuma" {
		next.Notify = cfg.UptimeKuma.RelayToAgent
	}
	if disconnect {
		next = previous
		next.Revision = fmt.Sprintf("%d", time.Now().UnixNano())
		next.Read = false
		next.Execute = false
		next.Create = false
		next.Process = false
		next.Sign = false
	}
	if req.Credential != "" {
		next.Credential = req.Credential
	}
	if req.ClearCredential {
		next.Credential = ""
		next.Read = false
		next.Execute, next.Create, next.Process, next.Sign = false, false, false, false
	}
	app, exists, err := store.GetInstalled(ctx, id)
	if err != nil || !exists {
		return errors.New("Store app is not installed")
	}
	if next.URL == "" {
		if cfg.Runtime.IsDocker && next.DockerNetwork != "" {
			next.URL, err = desktopstore.IntegrationURL(app, next.AdminPort, true)
		} else {
			next.URL, err = s.storeIntegrationURL(ctx, app, next)
		}
	}
	if err != nil {
		return err
	}
	if err = security.ValidateHTTPBaseURL(next.URL); err != nil {
		return err
	}
	if err = validateStoreIntegrationGrants(id, next); err != nil {
		return err
	}
	mutations := []vaultMutation{}
	patch := map[string]interface{}{}
	if req.RelayPort != nil {
		if id != desktopstore.OpenWebUIAppID || *req.RelayPort < 1 || *req.RelayPort > 65535 {
			return errors.New("Invalid internal relay port")
		}
		patch["store_llm_relay"] = map[string]interface{}{"port": *req.RelayPort, "docker_network": next.DockerNetwork}
	}
	switch id {
	case "adguard-home":
		patch["adguard"] = map[string]interface{}{"store_managed": true, "enabled": next.Read, "readonly": !next.Execute, "url": next.URL, "username": next.Username}
		if req.Credential != "" || req.ClearCredential {
			mutations = append(mutations, vaultMutation{key: "adguard_password", value: req.Credential, delete: req.ClearCredential})
		}
		next.Credential = ""
	case "uptime-kuma":
		patch["uptime_kuma"] = map[string]interface{}{"store_managed": true, "enabled": next.Read, "base_url": next.URL, "relay_to_agent": next.Notify}
		if req.Credential != "" || req.ClearCredential {
			mutations = append(mutations, vaultMutation{key: "uptime_kuma_api_key", value: req.Credential, delete: req.ClearCredential})
		}
		next.Credential = ""
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return err
	}
	mutations = append(mutations, vaultMutation{key: desktopstore.IntegrationKey(id), value: string(raw)})
	if err := s.persistStoreIntegrationPatch(cfg, patch, mutations); err != nil {
		return err
	}
	if id == desktopstore.OpenWebUIAppID && next.Read && !disconnect {
		return s.connectOpenWebUI(ctx, next, true)
	}
	return nil
}

func (s *Server) persistStoreIntegrationPatch(cfg *config.Config, patch map[string]interface{}, mutations []vaultMutation) error {
	before, err := os.ReadFile(cfg.ConfigPath)
	if err != nil {
		return err
	}
	var document map[string]interface{}
	if err = yaml.Unmarshal(before, &document); err != nil {
		return err
	}
	deepMerge(document, patch, "")
	out, err := yaml.Marshal(document)
	if err != nil {
		return err
	}
	reloaded, err := saveProviderConfig(s, cfg.ConfigPath, before, out, mutations)
	if err != nil {
		return err
	}
	reloaded.Runtime = cfg.Runtime
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(reloaded)
	s.CfgMu.Unlock()
	s.restartUptimeKumaPoller()
	return nil
}

func validateStoreIntegrationGrants(id string, p desktopstore.IntegrationSettings) error {
	if !p.Read && (p.Execute || p.Create || p.Process || p.Sign) {
		return errors.New("Enable the connection before granting additional permissions")
	}
	if len(p.AllowedIDs) > 200 || len(p.Username) > 256 || len(p.Source) > 256 {
		return errors.New("Integration settings exceed their limits")
	}
	for _, id := range p.AllowedIDs {
		if id == "" || len(id) > 128 || strings.ContainsAny(id, "/\\?#%\r\n") {
			return errors.New("Invalid allowed action ID")
		}
	}
	if p.DNSAddress != "" {
		ip := net.ParseIP(p.DNSAddress)
		if ip == nil || !ip.IsGlobalUnicast() || ip.IsLoopback() {
			return errors.New("Choose a specific LAN address for DNS port 53")
		}
	}
	if p.Execute && id != "adguard-home" && id != "node-red" && id != "olivetin" && id != "n8n" {
		return errors.New("Execution is not supported by this integration")
	}
	if p.Create && id != "bytestash" {
		return errors.New("Creation is not supported by this integration")
	}
	if (p.Process || p.Sign) && id != "stirling-pdf" {
		return errors.New("PDF permissions are not supported by this integration")
	}
	if p.Notify && id != "uptime-kuma" {
		return errors.New("Notifications are not supported by this integration")
	}
	if p.AdminPort != "" && (id != "adguard-home" || (p.AdminPort != "main" && p.AdminPort != "admin")) {
		return errors.New("Invalid admin port")
	}
	if p.DNSAddress != "" && id != "adguard-home" {
		return errors.New("DNS publishing is only supported for AdGuard")
	}
	return nil
}

func (s *Server) testStoreIntegration(ctx context.Context, store *desktopstore.Service, id string, settings desktopstore.IntegrationSettings) (testErr error) {
	defer func() {
		s.storeIntegrations.Lock()
		defer s.storeIntegrations.Unlock()
		if testErr == nil {
			delete(s.storeIntegrations.failures, id)
			return
		}
		if s.storeIntegrations.failures == nil {
			s.storeIntegrations.failures = map[string]string{}
		}
		s.storeIntegrations.failures[id] = security.Scrub(testErr.Error())
	}()
	app, exists, err := store.GetInstalled(ctx, id)
	if err != nil || !exists || app.Status != desktopstore.AppStatusRunning {
		return errors.New("Start the Store app before testing its connection")
	}
	if settings.Revision == "" {
		return errors.New("Save the connection settings first")
	}
	cfg := s.ConfigSnapshot()
	if storeLegacyConflict(cfg, id, settings) {
		return errors.New("Integration configuration has changed")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	switch id {
	case desktopstore.OpenWebUIAppID:
		if err := s.connectOpenWebUI(ctx, settings, false); err != nil {
			return err
		}
	case "adguard-home":
		var result struct {
			Status string `json:"status"`
			Data   struct {
				Version string `json:"version"`
			} `json:"data"`
		}
		raw := tools.AdGuardStatus(tools.AdGuardConfig{Context: ctx, URL: settings.URL, Username: cfg.AdGuard.Username, Password: cfg.AdGuard.Password})
		if cfg.AdGuard.Password == "" || json.Unmarshal([]byte(raw), &result) != nil || result.Status != "ok" || result.Data.Version == "" {
			return errors.New("AdGuard connection failed; complete setup using admin port 3000, or update the Store app to publish existing port 80")
		}
	case "uptime-kuma":
		uk := uptimeKumaToolConfigFrom(cfg.UptimeKuma)
		uk.StoreManaged = false
		if _, err := tools.FetchUptimeKumaSnapshot(ctx, uk, s.Logger); err != nil {
			return errors.New("Kuma connection failed; check the address and metrics API key")
		}
	default:
		return errors.New("Store update required: integration adapter unavailable")
	}
	bindingCtx, bindingCancel := context.WithCancel(context.Background())
	s.storeIntegrations.Lock()
	if s.storeIntegrations.closed || ctx.Err() != nil {
		s.storeIntegrations.Unlock()
		bindingCancel()
		return errors.New("Store connection test cancelled")
	}
	if s.storeIntegrations.bindings == nil {
		s.storeIntegrations.bindings = map[string]storeIntegrationBinding{}
	}
	if old, ok := s.storeIntegrations.bindings[id]; ok {
		old.cancel()
	}
	s.storeIntegrations.bindings[id] = storeIntegrationBinding{settings: settings, ctx: bindingCtx, cancel: bindingCancel}
	s.storeIntegrations.Unlock()
	return nil
}

func (s *Server) storeIntegrationAdminPort(id string) string {
	if id != "adguard-home" {
		return ""
	}
	s.storeIntegrations.Lock()
	defer s.storeIntegrations.Unlock()
	b, ok := s.storeIntegrations.bindings[id]
	if ok && b.ctx.Err() == nil {
		return b.settings.AdminPort
	}
	return ""
}

func (s *Server) stopStoreIntegrations() {
	s.revokeStoreRelay("", true)
	s.storeIntegrations.Lock()
	s.storeIntegrations.closed = true
	for id, b := range s.storeIntegrations.bindings {
		b.cancel()
		delete(s.storeIntegrations.bindings, id)
	}
	s.storeIntegrations.Unlock()
}

func (s *Server) deleteStoreIntegrationData(ctx context.Context, id string) error {
	s.revokeStoreIntegration(id)
	s.CfgSaveMu.Lock()
	defer s.CfgSaveMu.Unlock()
	settings, err := desktopstore.ReadIntegration(s.Vault, id)
	if err != nil {
		return err
	}
	cfg := s.ConfigSnapshot()
	patch := map[string]interface{}{}
	mutations := []vaultMutation{{key: desktopstore.IntegrationKey(id), delete: true}}
	if id == "adguard-home" && cfg.AdGuard.StoreManaged && cfg.AdGuard.URL == settings.URL {
		patch["adguard"] = map[string]interface{}{"store_managed": false, "enabled": false, "url": "", "username": ""}
		mutations = append(mutations, vaultMutation{key: "adguard_password", delete: true})
	}
	if id == "uptime-kuma" && cfg.UptimeKuma.StoreManaged && cfg.UptimeKuma.BaseURL == settings.URL {
		patch["uptime_kuma"] = map[string]interface{}{"store_managed": false, "enabled": false, "base_url": "", "relay_to_agent": false}
		mutations = append(mutations, vaultMutation{key: "uptime_kuma_api_key", delete: true})
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return s.persistStoreIntegrationPatch(cfg, patch, mutations)
}

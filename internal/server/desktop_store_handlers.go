package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/desktopstore"
	"aurago/internal/tools"
	"aurago/internal/tsnetnode"
)

func registerDesktopStoreRoutes(mux *http.ServeMux, s *Server) {
	mux.HandleFunc("/api/desktop/store/catalog", handleDesktopStoreCatalog(s))
	mux.HandleFunc("/api/desktop/store/apps", handleDesktopStoreApps(s))
	mux.HandleFunc("/api/desktop/store/install", handleDesktopStoreInstall(s))
	mux.HandleFunc("/api/desktop/store/operations/", handleDesktopStoreOperation(s))
	mux.HandleFunc("/api/desktop/store/apps/", handleDesktopStoreAppRoute(s))
}

func (s *Server) getDesktopStoreService(ctx context.Context) (*desktopstore.Service, error) {
	s.DesktopMu.Lock()
	if s.DesktopStore != nil {
		store := s.DesktopStore
		s.DesktopMu.Unlock()
		return store, nil
	}
	s.DesktopMu.Unlock()

	desktopSvc, _, err := s.getDesktopService(ctx)
	if err != nil {
		return nil, err
	}
	desktopCfg := desktopSvc.Config()

	s.DesktopMu.Lock()
	defer s.DesktopMu.Unlock()
	if s.DesktopStore != nil {
		return s.DesktopStore, nil
	}
	var launchpadAdapter desktopstore.LaunchpadAdapter
	if s.LaunchpadDB != nil {
		launchpadAdapter = desktopstore.SQLiteLaunchpadAdapter{
			DB:      s.LaunchpadDB,
			DataDir: desktopCfg.DataDir,
		}
	}
	store, err := desktopstore.NewService(desktopstore.Config{
		DBPath:       filepath.Join(desktopCfg.DataDir, "desktop_store.db"),
		DockerHost:   desktopCfg.DockerHost,
		DataDir:      desktopCfg.DataDir,
		WorkspaceDir: desktopCfg.WorkspaceDir,
		Docker:       desktopstore.NewToolsDockerAdapter(desktopCfg.DockerHost, desktopCfg.WorkspaceDir, s.Logger),
		Desktop:      desktopSvc,
		Launchpad:    launchpadAdapter,
		Secrets:      s.Vault,
	})
	if err != nil {
		return nil, err
	}
	if err := store.Init(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}
	s.DesktopStore = store
	return store, nil
}

func handleDesktopStoreCatalog(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		setDesktopStoreNoCacheHeaders(w)
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		store, err := s.getDesktopStoreService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		apps, err := store.ListApps(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		dockerAvailable := false
		if desktopSvc, _, err := s.getDesktopService(r.Context()); err == nil {
			dockerAvailable = tools.DockerPing(desktopSvc.Config().DockerHost) == nil
		}
		mutationDisabledReason := s.desktopStoreMutationDisabledReason()
		if !dockerAvailable && mutationDisabledReason == "" {
			mutationDisabledReason = "docker_unavailable"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":                   "ok",
			"catalog":                  store.Catalog(),
			"installed":                apps,
			"docker_available":         dockerAvailable,
			"mutations_allowed":        dockerAvailable && mutationDisabledReason == "",
			"mutation_disabled_reason": mutationDisabledReason,
		})
	}
}

func setDesktopStoreNoCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

func handleDesktopStoreApps(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		setDesktopStoreNoCacheHeaders(w)
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		store, err := s.getDesktopStoreService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		apps, err := store.ListApps(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "apps": apps})
	}
}

func handleDesktopStoreInstall(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if rejectDesktopStoreMutationIfDisabled(s, w) {
			return
		}
		store, err := s.getDesktopStoreService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		var req desktopstore.InstallRequest
		if err := decodeDesktopJSON(w, r, &req, desktopSmallJSONBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		op, err := store.StartInstall(r.Context(), req)
		if err != nil {
			writeDesktopStoreStartError(w, err)
			return
		}
		s.runDesktopStoreOperation(op.ID)
		writeDesktopStoreOperationAccepted(w, op)
	}
}

func handleDesktopStoreOperation(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		setDesktopStoreNoCacheHeaders(w)
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/desktop/store/operations/"), "/")
		if id == "" {
			jsonError(w, "operation id is required", http.StatusBadRequest)
			return
		}
		store, err := s.getDesktopStoreService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		op, err := store.Operation(r.Context(), id)
		if err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "operation": op})
	}
}

func handleDesktopStoreAppRoute(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/desktop/store/apps/"), "/")
		parts := strings.Split(rest, "/")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			jsonError(w, "app id is required", http.StatusBadRequest)
			return
		}
		appID := parts[0]
		action := ""
		if len(parts) > 1 {
			action = parts[1]
		}
		if action == "open-url" {
			handleDesktopStoreOpenURL(s, appID)(w, r)
			return
		}
		if action == "terminal" {
			handleDesktopStoreTerminal(s, appID)(w, r)
			return
		}
		if action == "credentials" {
			handleDesktopStoreCredentials(s, appID)(w, r)
			return
		}
		if len(parts) == 4 && parts[1] == "companions" && parts[2] == "agent" && parts[3] == "config" {
			handleDesktopStoreBeszelAgentConfig(s, appID)(w, r)
			return
		}
		if r.Method == http.MethodDelete && action == "" {
			handleDesktopStoreDelete(s, appID)(w, r)
			return
		}
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if rejectDesktopStoreMutationIfDisabled(s, w) {
			return
		}
		opType := action
		switch opType {
		case desktopstore.OperationStart, desktopstore.OperationStop, desktopstore.OperationRestart, desktopstore.OperationUpdate:
		default:
			jsonError(w, "unsupported store action", http.StatusBadRequest)
			return
		}
		store, err := s.getDesktopStoreService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		op, err := store.StartAppOperation(r.Context(), appID, opType, desktopstore.OperationRequest{})
		if err != nil {
			writeDesktopStoreStartError(w, err)
			return
		}
		s.runDesktopStoreOperation(op.ID)
		writeDesktopStoreOperationAccepted(w, op)
	}
}

func handleDesktopStoreOpenURL(s *Server, appID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		setDesktopStoreNoCacheHeaders(w)
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		store, err := s.getDesktopStoreService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		fromTailnet, tailnetDNS := s.storeTailnetRequestInfo(r)
		openURL, app, err := store.OpenURL(r.Context(), appID, r.Host, fromTailnet, tailnetDNS, r.URL.Query().Get("port_id"))
		if err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"url":    openURL,
			"app":    app,
		})
	}
}

func handleDesktopStoreTerminal(s *Server, appID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireDesktopPermission(s, w, r, desktopScopeWrite) {
			return
		}
		cfg, dockerEnabled, dockerReadOnly := containerDockerConfig(s)
		if !dockerEnabled {
			containerJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "error", "message": "Docker is not enabled"})
			return
		}
		if dockerReadOnly {
			containerJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "Docker is in read-only mode"})
			return
		}
		if !sameOriginOrNoOrigin(r) {
			containerJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "forbidden websocket origin"})
			return
		}
		store, err := s.getDesktopStoreService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		app, ok, err := store.GetInstalled(r.Context(), appID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !ok {
			jsonError(w, "store app is not installed", http.StatusNotFound)
			return
		}
		if app.Status != desktopstore.AppStatusRunning {
			containerJSON(w, http.StatusConflict, map[string]string{"status": "error", "message": "Store app is not running"})
			return
		}
		entry, ok := desktopStoreCatalogEntry(store.Catalog(), app.AppID)
		if !ok || entry.Metadata["terminal_enabled"] != "true" {
			containerJSON(w, http.StatusForbidden, map[string]string{"status": "error", "message": "Store app does not allow terminal access"})
			return
		}
		running, err := activeContainerTerminalBackend.ContainerRunning(r.Context(), cfg, app.ContainerName)
		if err != nil {
			containerJSON(w, http.StatusBadGateway, map[string]string{"status": "error", "message": err.Error()})
			return
		}
		if !running {
			containerJSON(w, http.StatusConflict, map[string]string{"status": "error", "message": "Container is not running"})
			return
		}
		bootstrap := strings.TrimSpace(r.URL.Query().Get("bootstrap")) == "1"
		execCmd := storeTerminalExecCommand(entry.Metadata, bootstrap)
		session, err := activeContainerTerminalBackend.CreateSession(r.Context(), cfg, app.ContainerName, 120, 30, execCmd)
		if err != nil {
			containerJSON(w, http.StatusBadGateway, map[string]string{"status": "error", "message": err.Error()})
			return
		}
		conn, err := containerTerminalUpgrader.Upgrade(w, r, nil)
		if err != nil {
			_ = session.Close()
			return
		}
		defer conn.Close()
		serveContainerTerminalSession(r.Context(), conn, session)
	}
}

func desktopStoreCatalogEntry(catalog []desktopstore.CatalogEntry, appID string) (desktopstore.CatalogEntry, bool) {
	for _, entry := range catalog {
		if strings.EqualFold(entry.ID, appID) {
			return entry, true
		}
	}
	return desktopstore.CatalogEntry{}, false
}

func handleDesktopStoreCredentials(s *Server, appID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		setDesktopStoreNoCacheHeaders(w)
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		store, err := s.getDesktopStoreService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		credentials, err := store.ExposedCredentials(r.Context(), appID)
		if err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":      "ok",
			"credentials": credentials,
		})
	}
}

func handleDesktopStoreBeszelAgentConfig(s *Server, appID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if rejectDesktopStoreMutationIfDisabled(s, w) {
			return
		}
		if strings.ToLower(strings.TrimSpace(appID)) != "beszel" {
			jsonError(w, "unsupported store companion", http.StatusBadRequest)
			return
		}
		var req struct {
			Key   string `json:"key"`
			Token string `json:"token"`
		}
		if err := decodeDesktopJSON(w, r, &req, desktopSmallJSONBodyLimit); err != nil {
			jsonError(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		store, err := s.getDesktopStoreService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		app, err := store.ConfigureBeszelAgent(r.Context(), req.Key, req.Token)
		if err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.broadcastDesktopStoreChanged("")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ok",
			"app":    app,
		})
	}
}

func handleDesktopStoreDelete(s *Server, appID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		if rejectDesktopStoreMutationIfDisabled(s, w) {
			return
		}
		store, err := s.getDesktopStoreService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		deleteData := strings.EqualFold(r.URL.Query().Get("delete_data"), "true")
		op, err := store.StartAppOperation(r.Context(), appID, desktopstore.OperationUninstall, desktopstore.OperationRequest{DeleteData: deleteData})
		if err != nil {
			writeDesktopStoreStartError(w, err)
			return
		}
		s.runDesktopStoreOperation(op.ID)
		writeDesktopStoreOperationAccepted(w, op)
	}
}

func (s *Server) runDesktopStoreOperation(operationID string) {
	go func() {
		ctx, cancel := desktopStoreOperationContext(s.ShutdownCh, 30*time.Minute)
		defer cancel()
		store, err := s.getDesktopStoreService(ctx)
		if err != nil {
			if s.Logger != nil {
				s.Logger.Warn("Desktop store operation skipped", "operation_id", operationID, "error", err)
			}
			return
		}
		if err := store.RunOperation(ctx, operationID); err != nil && s.Logger != nil {
			s.Logger.Warn("Desktop store operation failed", "operation_id", operationID, "error", err)
		}
		if err := s.reconcileDesktopStoreTailscale(ctx); err != nil && s.Logger != nil {
			s.Logger.Warn("Desktop store Tailscale proxy reconcile failed", "operation_id", operationID, "error", err)
		}
		s.broadcastDesktopStoreChanged(operationID)
	}()
}

func desktopStoreOperationContext(shutdownCh <-chan struct{}, timeout time.Duration) (context.Context, context.CancelFunc) {
	baseCtx, baseCancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	if shutdownCh != nil {
		go func() {
			select {
			case <-shutdownCh:
				baseCancel()
			case <-done:
			}
		}()
	}
	ctx, timeoutCancel := context.WithTimeout(baseCtx, timeout)
	cancel := func() {
		select {
		case <-done:
		default:
			close(done)
		}
		timeoutCancel()
		baseCancel()
	}
	return ctx, cancel
}

func (s *Server) broadcastDesktopStoreChanged(operationID string) {
	s.DesktopMu.Lock()
	hub := s.DesktopHub
	s.DesktopMu.Unlock()
	event := desktop.Event{
		Type: "desktop_changed",
		Payload: map[string]any{
			"operation":    "desktop_store_changed",
			"operation_id": operationID,
		},
		CreatedAt: time.Now().UTC(),
	}
	broadcastDesktopEvent(s, hub, event)
}

func writeDesktopStoreOperationAccepted(w http.ResponseWriter, op desktopstore.Operation) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":       "accepted",
		"operation_id": op.ID,
		"operation":    op,
	})
}

func writeDesktopStoreStartError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, desktopstore.ErrOperationInProgress) {
		status = http.StatusConflict
	}
	jsonError(w, err.Error(), status)
}

func rejectDesktopStoreMutationIfDisabled(s *Server, w http.ResponseWriter) bool {
	reason := s.desktopStoreMutationDisabledReason()
	if reason == "" {
		return false
	}
	jsonError(w, desktopStoreMutationDisabledMessage(reason), http.StatusForbidden)
	return true
}

func (s *Server) desktopStoreMutationDisabledReason() string {
	if s == nil || s.Cfg == nil {
		return ""
	}
	s.CfgMu.RLock()
	defer s.CfgMu.RUnlock()
	switch {
	case s.Cfg.VirtualDesktop.ReadOnly:
		return "desktop_readonly"
	case !s.Cfg.Docker.Enabled:
		return "docker_disabled"
	case s.Cfg.Docker.ReadOnly:
		return "docker_readonly"
	default:
		return ""
	}
}

func desktopStoreMutationDisabledMessage(reason string) string {
	switch reason {
	case "desktop_readonly":
		return "Virtual Desktop is in read-only mode. Store actions are disabled."
	case "docker_disabled":
		return "Docker integration is disabled. Store actions are disabled."
	case "docker_readonly":
		return "Docker is in read-only mode. Store actions are disabled."
	default:
		return "Store actions are disabled."
	}
}

func (s *Server) storeTailnetRequestInfo(r *http.Request) (bool, string) {
	if s == nil || s.TsNetManager == nil || r == nil {
		return false, ""
	}
	status := s.TsNetManager.GetStatus()
	dns := strings.Trim(strings.TrimSpace(status.DNS), ".")
	if !status.Running || dns == "" {
		return false, dns
	}
	host := strings.Trim(strings.ToLower(hostWithoutPort(r.Host)), ".")
	return strings.EqualFold(host, strings.ToLower(dns)), dns
}

func hostWithoutPort(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		return parsed
	}
	return host
}

func (s *Server) reconcileDesktopStoreTailscale(ctx context.Context) error {
	if s == nil || s.TsNetManager == nil {
		return nil
	}
	store, err := s.getDesktopStoreService(ctx)
	if err != nil {
		return err
	}
	apps, err := store.ListApps(ctx)
	if err != nil {
		return err
	}
	status := s.TsNetManager.GetStatus()
	if !status.Running {
		for _, app := range apps {
			if app.TailscaleEnabled {
				_ = store.SetTailscaleStatus(ctx, app.AppID, desktopstore.TailscaleStatusPending)
			}
		}
		return nil
	}
	specs, active := desktopStoreTailscaleProxySpecs(apps)
	var cfgSnapshot *config.Config
	s.CfgMu.RLock()
	if s.Cfg != nil {
		copy := *s.Cfg
		cfgSnapshot = &copy
	}
	s.CfgMu.RUnlock()
	specs = append(specs, dograhTailscaleProxySpecs(cfgSnapshot)...)
	if err := s.TsNetManager.ReconcileStoreAppProxies(specs); err != nil {
		return err
	}
	for _, app := range apps {
		if !app.TailscaleEnabled {
			continue
		}
		if _, ok := active[app.AppID]; ok {
			_ = store.SetTailscaleStatus(ctx, app.AppID, desktopstore.TailscaleStatusActive)
		} else {
			_ = store.SetTailscaleStatus(ctx, app.AppID, desktopstore.TailscaleStatusPending)
		}
	}
	return nil
}

func desktopStoreTailscaleProxySpecs(apps []desktopstore.InstalledApp) ([]tsnetnode.StoreAppProxySpec, map[string]struct{}) {
	var specs []tsnetnode.StoreAppProxySpec
	active := map[string]struct{}{}
	for _, app := range apps {
		if !app.TailscaleEnabled || app.Status != desktopstore.AppStatusRunning {
			continue
		}
		ports := app.Ports
		if len(ports) == 0 && app.HostPort > 0 {
			ports = []desktopstore.PortBinding{{ID: "main", HostPort: app.HostPort}}
		}
		for i, port := range ports {
			if port.HostPort <= 0 {
				continue
			}
			id := app.AppID
			if i > 0 && strings.TrimSpace(port.ID) != "" {
				id = app.AppID + "-" + strings.ToLower(strings.TrimSpace(port.ID))
			}
			specs = append(specs, tsnetnode.StoreAppProxySpec{
				ID:        id,
				Port:      port.HostPort,
				TargetURL: fmtStoreLocalTarget(port.HostPort),
				Enabled:   true,
			})
			if i == 0 {
				active[app.AppID] = struct{}{}
			}
		}
	}
	return specs, active
}

func dograhTailscaleProxySpecs(cfg *config.Config) []tsnetnode.StoreAppProxySpec {
	if cfg == nil || !cfg.Dograh.Enabled || !dograhURLNeedsServerProxy(cfg.Dograh.UIURL) {
		return nil
	}
	port := dograhURLPort(cfg.Dograh.UIURL)
	if port <= 0 {
		port = cfg.Dograh.UIHostPort
	}
	if port <= 0 {
		port = cfg.Dograh.UIPort
	}
	if port <= 0 {
		return nil
	}
	target := dograhProxyTargetURL(cfg.Dograh.UIURL, port)
	if target == "" {
		return nil
	}
	return []tsnetnode.StoreAppProxySpec{{
		ID:        "dograh",
		Port:      port,
		TargetURL: target,
		Enabled:   true,
	}}
}

func dograhURLPort(rawURL string) int {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil || parsed.Port() == "" {
		return 0
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		return 0
	}
	return port
}

func dograhProxyTargetURL(rawURL string, fallbackPort int) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil || parsed.Hostname() == "" {
		return ""
	}
	if parsed.Scheme == "" {
		parsed.Scheme = "http"
	}
	host := parsed.Hostname()
	if strings.Trim(strings.ToLower(host), "[]") == "0.0.0.0" || strings.Trim(strings.ToLower(host), "[]") == "::" {
		host = "127.0.0.1"
	}
	port := parsed.Port()
	if port == "" && fallbackPort > 0 {
		port = strconv.Itoa(fallbackPort)
	}
	if port != "" {
		parsed.Host = net.JoinHostPort(host, port)
	} else {
		parsed.Host = host
	}
	if parsed.Path == "" {
		parsed.Path = "/"
	}
	return parsed.String()
}

func fmtStoreLocalTarget(port int) string {
	return "http://127.0.0.1:" + strconv.Itoa(port) + "/"
}

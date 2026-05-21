package server

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/desktopstore"
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
		DBPath:     filepath.Join(desktopCfg.DataDir, "desktop_store.db"),
		DockerHost: desktopCfg.DockerHost,
		DataDir:    desktopCfg.DataDir,
		Docker:     desktopstore.NewToolsDockerAdapter(desktopCfg.DockerHost, desktopCfg.WorkspaceDir, s.Logger),
		Desktop:    desktopSvc,
		Launchpad:  launchpadAdapter,
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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "ok",
			"catalog":   store.Catalog(),
			"installed": apps,
		})
	}
}

func handleDesktopStoreApps(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
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
		if !requireDesktopPermission(s, w, r, desktopScopeRead) {
			return
		}
		store, err := s.getDesktopStoreService(r.Context())
		if err != nil {
			jsonError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		fromTailnet, tailnetDNS := s.storeTailnetRequestInfo(r)
		openURL, app, err := store.OpenURL(r.Context(), appID, r.Host, fromTailnet, tailnetDNS)
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

func handleDesktopStoreDelete(s *Server, appID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
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
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
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
	specs := make([]tsnetnode.StoreAppProxySpec, 0, len(apps))
	active := map[string]struct{}{}
	for _, app := range apps {
		if !app.TailscaleEnabled || app.Status != desktopstore.AppStatusRunning || app.TailscalePort <= 0 {
			continue
		}
		specs = append(specs, tsnetnode.StoreAppProxySpec{
			ID:        app.AppID,
			Port:      app.TailscalePort,
			TargetURL: fmtStoreLocalTarget(app.HostPort),
			Enabled:   true,
		})
		active[app.AppID] = struct{}{}
	}
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

func fmtStoreLocalTarget(port int) string {
	return "http://127.0.0.1:" + strconv.Itoa(port) + "/"
}

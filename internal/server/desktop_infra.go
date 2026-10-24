package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/infra"
	"aurago/internal/tools"
)

const (
	desktopInfraBasePath  = "/api/desktop/infra"
	desktopInfraBodyLimit = 256 << 10
	desktopInfraTTL       = 30 * time.Second
	desktopInfraErrorTTL  = 5 * time.Second
)

type desktopInfraRuntime struct {
	store          *infra.Store
	mu             sync.Mutex
	cache          map[string]*desktopInfraProviderCache
	refreshDone    chan struct{}
	refreshClosing bool
	operations     map[uint64]context.CancelFunc
	nextOperation  uint64
	configKey      string
	configSnapshot *config.Config
	dockerRequest  func(context.Context, tools.DockerConfig, string, string, string) ([]byte, int, error)
}

type desktopInfraProviderCache struct {
	configKey  string
	retryAt    time.Time
	inflight   chan struct{}
	err        error
	generation uint64
}

func registerDesktopInfraRoutes(mux *http.ServeMux, s *Server) {
	handler := handleDesktopInfra(s)
	mux.HandleFunc(desktopInfraBasePath, handler)
	mux.HandleFunc(desktopInfraBasePath+"/", handler)
}

func handleDesktopInfra(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		desktopInfraHeaders(w)
		path, ok := desktopInfraRoutePath(r.URL.Path)
		if !ok {
			desktopInfraError(w, http.StatusNotFound, "not_found", "Infra route not found")
			return
		}
		write := r.Method != http.MethodGet && r.Method != http.MethodHead
		cfg, store, runtime, finish, ok := desktopInfraAccess(s, w, r, write)
		if !ok {
			return
		}
		if finish != nil {
			defer finish()
		}
		switch {
		case path == "snapshot" && r.Method == http.MethodGet:
			desktopInfraSnapshot(w, r, s, cfg, store, runtime)
		case path == "candidates" && r.Method == http.MethodGet:
			desktopInfraCandidates(w, r, s, cfg, store, runtime)
		case path == "targets":
			desktopInfraTargetsHandler(w, r, s, cfg)
		case path == "nodes" && r.Method == http.MethodPost:
			desktopInfraCreateNode(w, r, s, cfg, store)
		case strings.HasPrefix(path, "nodes/"):
			desktopInfraNodeRoute(w, r, s, cfg, store, runtime, strings.TrimPrefix(path, "nodes/"))
		case path == "import" && r.Method == http.MethodPost:
			desktopInfraImport(w, r, s, cfg, store, runtime)
		case path == "edges" && r.Method == http.MethodPost:
			desktopInfraCreateEdge(w, r, store)
		case strings.HasPrefix(path, "edges/"):
			desktopInfraEdgeRoute(w, r, store, strings.TrimPrefix(path, "edges/"))
		case path == "layout" && r.Method == http.MethodPut:
			desktopInfraSaveLayout(w, r, s, cfg, store)
		default:
			desktopInfraMethodNotAllowed(w)
		}
	}
}

func desktopInfraRoutePath(path string) (string, bool) {
	if path == desktopInfraBasePath {
		return "snapshot", true
	}
	if !strings.HasPrefix(path, desktopInfraBasePath+"/") {
		return "", false
	}
	value := strings.TrimPrefix(path, desktopInfraBasePath+"/")
	if value == "" || strings.Contains(value, "//") || strings.HasSuffix(value, "/") {
		return "", false
	}
	return value, true
}

func desktopInfraAccess(s *Server, w http.ResponseWriter, r *http.Request, write bool) (*config.Config, *infra.Store, *desktopInfraRuntime, context.CancelFunc, bool) {
	if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
		return nil, nil, nil, nil, false
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled {
		desktopInfraError(w, http.StatusServiceUnavailable, "desktop_disabled", "Virtual Desktop is disabled")
		return nil, nil, nil, nil, false
	}
	if !cfg.VirtualDesktop.Infra.Enabled {
		desktopInfraError(w, http.StatusServiceUnavailable, "infra_disabled", "Infra is disabled")
		return nil, nil, nil, nil, false
	}
	if write {
		if !checkDesktopOperation(s, w, r, desktopWrite) {
			return nil, nil, nil, nil, false
		}
		if cfg.VirtualDesktop.Infra.ReadOnly {
			desktopInfraError(w, http.StatusForbidden, "infra_readonly", "Infra is read-only")
			return nil, nil, nil, nil, false
		}
	}
	if s.InventoryDB == nil {
		desktopInfraError(w, http.StatusServiceUnavailable, "infra_unavailable", "Infra storage is unavailable")
		return nil, nil, nil, nil, false
	}
	runtime, err := s.desktopInfraRuntime()
	if err != nil {
		desktopInfraError(w, http.StatusServiceUnavailable, "infra_unavailable", "Infra storage is unavailable")
		return nil, nil, nil, nil, false
	}
	if !write {
		return cfg, runtime.store, runtime, nil, true
	}
	ctx, finish := runtime.beginOperation(r.Context(), desktopInfraRuntimeConfigKey(s, cfg), cfg)
	if ctx.Err() != nil || !s.desktopInfraConfigAdmitted(cfg) {
		finish()
		// A config publication stores the admission fence before the new snapshot
		// becomes visible. Retry once with the newly published snapshot instead of
		// reporting a spurious policy change.
		if next := s.ConfigSnapshot(); next != nil && next != cfg && s.desktopInfraConfigAdmitted(next) &&
			next.VirtualDesktop.Enabled && next.VirtualDesktop.Infra.Enabled &&
			!next.VirtualDesktop.ReadOnly && !next.VirtualDesktop.Infra.ReadOnly {
			cfg = next
			ctx, finish = runtime.beginOperation(r.Context(), desktopInfraRuntimeConfigKey(s, cfg), cfg)
			if ctx.Err() == nil && s.desktopInfraConfigAdmitted(cfg) {
				*r = *r.WithContext(ctx)
				return cfg, runtime.store, runtime, finish, true
			}
			finish()
		}
		desktopInfraError(w, http.StatusForbidden, "infra_policy_changed", "Infra access changed; reload and try again")
		return nil, nil, nil, nil, false
	}
	*r = *r.WithContext(ctx)
	return cfg, runtime.store, runtime, finish, true
}

func (s *Server) desktopInfraConfigAdmitted(snapshot *config.Config) bool {
	if s == nil || snapshot == nil || s.ConfigSnapshot() != snapshot {
		return false
	}
	admitted := s.infraConfigAdmission.Load()
	return admitted == nil || admitted == snapshot
}

func (r *desktopInfraRuntime) beginOperation(parent context.Context, configKey string, snapshot *config.Config) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	if r == nil {
		return ctx, cancel
	}
	r.mu.Lock()
	if r.configSnapshot != nil && r.configSnapshot != snapshot {
		r.mu.Unlock()
		cancel()
		return ctx, cancel
	}
	operations := map[uint64]context.CancelFunc(nil)
	if r.configKey != "" && r.configKey != configKey {
		operations = r.operations
		r.operations = make(map[uint64]context.CancelFunc)
		// Provider identities are part of the config key; cached freshness for the
		// previous configuration never applies to the new one.
		r.cache = make(map[string]*desktopInfraProviderCache)
	}
	r.configKey = configKey
	r.configSnapshot = snapshot
	if r.operations == nil {
		r.operations = make(map[uint64]context.CancelFunc)
	}
	r.nextOperation++
	id := r.nextOperation
	r.operations[id] = cancel
	r.mu.Unlock()
	for _, stop := range operations {
		stop()
	}
	return ctx, func() {
		cancel()
		r.mu.Lock()
		delete(r.operations, id)
		r.mu.Unlock()
	}
}

func (r *desktopInfraRuntime) cancelOperations(configKey string, snapshot *config.Config) {
	if r == nil {
		return
	}
	r.mu.Lock()
	operations := r.operations
	r.operations = make(map[uint64]context.CancelFunc)
	r.cache = make(map[string]*desktopInfraProviderCache)
	r.configKey = configKey
	r.configSnapshot = snapshot
	r.mu.Unlock()
	for _, cancel := range operations {
		cancel()
	}
}

func (s *Server) cancelDesktopInfraOperations(cfg *config.Config) {
	if s == nil {
		return
	}
	s.infraMu.Lock()
	runtime := s.infraRuntime
	s.infraMu.Unlock()
	if runtime != nil {
		runtime.cancelOperations(desktopInfraRuntimeConfigKey(s, cfg), cfg)
	}
}

func (s *Server) updateDesktopInfraConfigSnapshot(cfg *config.Config) {
	if s == nil || cfg == nil {
		return
	}
	s.infraMu.Lock()
	runtime := s.infraRuntime
	s.infraMu.Unlock()
	if runtime == nil {
		return
	}
	runtime.mu.Lock()
	runtime.configKey = desktopInfraRuntimeConfigKey(s, cfg)
	runtime.configSnapshot = cfg
	runtime.mu.Unlock()
}

func (s *Server) desktopInfraRuntime() (*desktopInfraRuntime, error) {
	s.infraMu.Lock()
	defer s.infraMu.Unlock()
	if s.infraRuntime != nil {
		return s.infraRuntime, nil
	}
	if s.infraRuntimeErr != nil {
		return nil, s.infraRuntimeErr
	}
	if s.InventoryDB == nil {
		return nil, errors.New("infra inventory database is unavailable")
	}
	store, err := infra.New(s.InventoryDB)
	if err != nil {
		// Cache the failure: a retry would repeat the migration and its backup on
		// every request without a chance to succeed.
		s.infraRuntimeErr = fmt.Errorf("initialize infra store: %w", err)
		return nil, s.infraRuntimeErr
	}
	s.infraRuntime = &desktopInfraRuntime{
		store: store,
		cache: make(map[string]*desktopInfraProviderCache),
	}
	return s.infraRuntime, nil
}

// startDesktopInfraStore initializes the store during server startup so a schema
// migration and its database backup never run inside a request.
func (s *Server) startDesktopInfraStore() {
	if s == nil {
		return
	}
	if _, err := s.desktopInfraRuntime(); err != nil {
		logger := s.Logger
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("Infra storage is unavailable", "error", err)
	}
}

func desktopInfraSnapshot(w http.ResponseWriter, r *http.Request, s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime) {
	if r.URL.Query().Get("refresh") == "1" {
		if ctx, finish, ok := desktopInfraRefreshContext(s, r, cfg, runtime); ok {
			desktopInfraRefreshConfigured(ctx, s, cfg, store, runtime, true)
			finish()
		}
	} else {
		desktopInfraScheduleRefreshConfigured(s, cfg, store, runtime)
	}
	snapshot, err := store.SnapshotWith(r.Context(), infra.SnapshotOptions{IncludeIgnored: r.URL.Query().Get("include_ignored") == "1"})
	if err != nil {
		desktopInfraStoreError(w, err)
		return
	}
	snapshot.Capabilities = desktopInfraCapabilities(cfg)
	desktopInfraDecorateSnapshot(r.Context(), s, cfg, &snapshot)
	desktopInfraJSON(w, http.StatusOK, desktopInfraSnapshotResponse(snapshot, runtime))
}

// desktopInfraRefreshContext registers a read-triggered provider refresh with the
// same revocation fence as a write. It reports false when policy already changed,
// so the caller serves the stored snapshot without provider I/O.
func desktopInfraRefreshContext(s *Server, r *http.Request, cfg *config.Config, runtime *desktopInfraRuntime) (context.Context, func(), bool) {
	if runtime == nil || s == nil {
		return nil, func() {}, false
	}
	ctx, finish := runtime.beginOperation(r.Context(), desktopInfraRuntimeConfigKey(s, cfg), cfg)
	if ctx.Err() != nil || !s.desktopInfraConfigAdmitted(cfg) {
		finish()
		return nil, func() {}, false
	}
	return ctx, finish, true
}

func desktopInfraSnapshotResponse(snapshot infra.Snapshot, runtime *desktopInfraRuntime) map[string]any {
	sourceHosts := snapshot.SourceHosts
	if sourceHosts == nil {
		sourceHosts = []infra.SourceHost{}
	}
	response := map[string]any{
		"nodes": snapshot.Nodes, "edges": snapshot.Edges, "sources": snapshot.Sources,
		"source_hosts": sourceHosts, "capabilities": snapshot.Capabilities,
	}
	if sourceErrors := runtime.sourceErrors(); len(sourceErrors) > 0 {
		response["source_errors"] = sourceErrors
	}
	return response
}

func desktopInfraScheduleRefreshConfigured(s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime) {
	if runtime == nil || s == nil || !s.desktopInfraConfigAdmitted(cfg) {
		return
	}
	runtime.mu.Lock()
	if runtime.refreshClosing || runtime.refreshDone != nil {
		runtime.mu.Unlock()
		return
	}
	done := make(chan struct{})
	runtime.refreshDone = done
	runtime.mu.Unlock()

	parent := s.integrationCtx
	if parent == nil {
		parent = context.Background()
	}
	ctx, finish := runtime.beginOperation(parent, desktopInfraRuntimeConfigKey(s, cfg), cfg)
	finishRefresh := func() {
		runtime.mu.Lock()
		if runtime.refreshDone == done {
			runtime.refreshDone = nil
		}
		close(done)
		runtime.mu.Unlock()
	}
	if ctx.Err() != nil || !s.desktopInfraConfigAdmitted(cfg) {
		finish()
		finishRefresh()
		return
	}
	go func() {
		defer finishRefresh()
		defer finish()
		desktopInfraRefreshConfigured(ctx, s, cfg, store, runtime, false)
	}()
}

func (s *Server) drainDesktopInfraRefresh() {
	if s == nil {
		return
	}
	s.infraMu.Lock()
	runtime := s.infraRuntime
	s.infraMu.Unlock()
	if runtime == nil {
		return
	}
	runtime.mu.Lock()
	runtime.refreshClosing = true
	operations := runtime.operations
	runtime.operations = make(map[uint64]context.CancelFunc)
	done := runtime.refreshDone
	runtime.mu.Unlock()
	for _, cancel := range operations {
		cancel()
	}
	if done != nil {
		<-done
	}
}

func desktopInfraCandidates(w http.ResponseWriter, r *http.Request, s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime) {
	if ctx, finish, ok := desktopInfraRefreshContext(s, r, cfg, runtime); ok {
		desktopInfraRefreshConfigured(ctx, s, cfg, store, runtime, r.URL.Query().Get("refresh") == "1")
		finish()
	}
	// Ignored sources stay out of discoveries unless the caller asks for them;
	// they then carry ignored: true.
	includeIgnored := r.URL.Query().Get("include_ignored") == "1"
	sources, err := desktopInfraConfiguredSources(r.Context(), s, cfg, store, includeIgnored)
	if err != nil {
		desktopInfraStoreError(w, err)
		return
	}
	candidates := make([]infra.Candidate, 0, len(sources))
	for _, source := range sources {
		if !desktopInfraCandidateNamed(source) {
			continue
		}
		candidate, err := store.GetCandidate(r.Context(), infra.CandidateID(source.SourceIdentity))
		if err != nil {
			if errors.Is(err, infra.ErrSourceNotFound) || errors.Is(err, infra.ErrSourceRemoved) || errors.Is(err, infra.ErrSourceLinked) {
				continue
			}
			desktopInfraStoreError(w, err)
			return
		}
		candidates = append(candidates, candidate)
	}
	desktopInfraJSON(w, http.StatusOK, map[string]any{"candidates": candidates, "sources": sources})
}

func desktopInfraCreateNode(w http.ResponseWriter, r *http.Request, s *Server, cfg *config.Config, store *infra.Store) {
	var node infra.Node
	if !desktopInfraDecode(w, r, &node) {
		return
	}
	if !desktopInfraCheckAppBindings(w, r, s, cfg, node.Profile.AppBindings, nil) {
		return
	}
	var created infra.Node
	var err error
	if node.Kind == infra.NodeDevice {
		created, err = store.CreateManualNode(r.Context(), node)
	} else {
		created, err = store.CreateNode(r.Context(), node)
	}
	if err != nil {
		desktopInfraStoreError(w, err)
		return
	}
	created = desktopInfraDecorateNode(r.Context(), s, cfg, created)
	desktopInfraJSON(w, http.StatusCreated, created)
}

func desktopInfraNodeRoute(w http.ResponseWriter, r *http.Request, s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime, tail string) {
	parts := strings.Split(tail, "/")
	if len(parts) == 0 || !desktopInfraValidPathID(parts[0]) {
		desktopInfraError(w, http.StatusBadRequest, "invalid_node_id", "Node ID is invalid")
		return
	}
	nodeID := parts[0]
	if len(parts) == 2 && parts[1] == "source-edit" {
		desktopInfraSourceEdit(w, r, s, cfg, store, runtime, nodeID)
		return
	}
	if len(parts) == 2 && parts[1] == "actions" && r.Method == http.MethodPost {
		desktopInfraAction(w, r, s, cfg, store, runtime, nodeID)
		return
	}
	if len(parts) != 1 {
		desktopInfraError(w, http.StatusNotFound, "not_found", "Infra route not found")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var node infra.Node
		if !desktopInfraDecode(w, r, &node) {
			return
		}
		if node.ID != nodeID {
			desktopInfraError(w, http.StatusBadRequest, "invalid_node_id", "Node ID does not match the route")
			return
		}
		existing, err := store.GetNode(r.Context(), nodeID)
		if err != nil {
			desktopInfraStoreError(w, err)
			return
		}
		if !desktopInfraCheckAppBindings(w, r, s, cfg, node.Profile.AppBindings, existing.Profile.AppBindings) {
			return
		}
		updated, err := store.UpdateNode(r.Context(), node)
		if err != nil {
			desktopInfraStoreError(w, err)
			return
		}
		updated = desktopInfraDecorateNode(r.Context(), s, cfg, updated)
		desktopInfraJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		version, ok := desktopInfraVersionQuery(w, r)
		if !ok {
			return
		}
		ignoreSources, ok := desktopInfraFlagQuery(w, r, "ignore_sources")
		if !ok {
			return
		}
		node, err := store.GetNode(r.Context(), nodeID)
		if err != nil {
			desktopInfraStoreError(w, err)
			return
		}
		ignored := []infra.SourceIdentity{}
		if ignoreSources {
			ignored, err = store.DeleteNodeAndIgnoreSources(r.Context(), nodeID, version)
		} else {
			err = store.DeleteNode(r.Context(), nodeID, version)
		}
		if err != nil {
			desktopInfraStoreError(w, err)
			return
		}
		instances := make(map[string]struct{})
		for _, source := range node.Sources {
			if source.Kind == "proxmox" {
				instances[source.Instance] = struct{}{}
			}
		}
		for instance := range instances {
			_ = desktopInfraReconcileProxmoxEdges(r.Context(), store, instance)
		}
		response := map[string]any{"status": "deleted", "id": nodeID}
		if ignoreSources {
			response["ignored"] = ignored
		}
		desktopInfraJSON(w, http.StatusOK, response)
	default:
		desktopInfraMethodNotAllowed(w)
	}
}

func desktopInfraImport(w http.ResponseWriter, r *http.Request, s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime) {
	var req struct {
		CandidateID string `json:"candidate_id"`
		NodeID      string `json:"node_id"`
	}
	if !desktopInfraDecode(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.CandidateID) == "" {
		desktopInfraError(w, http.StatusBadRequest, "invalid_candidate_id", "Candidate ID is required")
		return
	}
	candidate, err := store.GetCandidate(r.Context(), req.CandidateID)
	if err != nil {
		desktopInfraStoreError(w, err)
		return
	}
	if !desktopInfraSourceConfigured(cfg, candidate.Source) {
		desktopInfraError(w, http.StatusNotFound, "source_unavailable", "This source is no longer configured")
		return
	}
	desktopInfraRefreshSource(r.Context(), s, cfg, store, runtime, candidate.Source.SourceIdentity)
	candidate, err = store.GetCandidate(r.Context(), req.CandidateID)
	if err != nil {
		desktopInfraStoreError(w, err)
		return
	}
	if candidate.Source.State != infra.SourceCurrent {
		desktopInfraError(w, http.StatusConflict, "source_stale", "The source could not be freshly verified")
		return
	}
	var node infra.Node
	if req.NodeID != "" {
		if !desktopInfraValidPathID(req.NodeID) {
			desktopInfraError(w, http.StatusBadRequest, "invalid_node_id", "Node ID is invalid")
			return
		}
		node, err = store.LinkSource(r.Context(), req.NodeID, candidate.Source.SourceIdentity)
	} else {
		candidate.Node.ID = ""
		candidate.Node.Version = 0
		candidate.Node.RegistryID = ""
		candidate.Node.Sources = []infra.Source{}
		node, err = store.CreateImportedNode(r.Context(), candidate.Node, candidate.Source.SourceIdentity)
	}
	if err != nil {
		desktopInfraStoreError(w, err)
		return
	}
	if candidate.Source.Kind == "proxmox" {
		// The graph write already succeeded; refreshing a provider-owned placement
		// here makes a newly imported guest/host pair visible without another poll.
		_ = desktopInfraReconcileProxmoxEdges(r.Context(), store, candidate.Source.Instance)
	}
	node = desktopInfraDecorateNode(r.Context(), s, cfg, node)
	desktopInfraJSON(w, http.StatusCreated, node)
}

func desktopInfraCreateEdge(w http.ResponseWriter, r *http.Request, store *infra.Store) {
	var edge infra.Edge
	if !desktopInfraDecode(w, r, &edge) {
		return
	}
	created, err := store.CreateEdge(r.Context(), edge)
	if err != nil {
		desktopInfraStoreError(w, err)
		return
	}
	desktopInfraJSON(w, http.StatusCreated, created)
}

func desktopInfraEdgeRoute(w http.ResponseWriter, r *http.Request, store *infra.Store, tail string) {
	if !desktopInfraValidPathID(tail) || strings.Contains(tail, "/") {
		desktopInfraError(w, http.StatusBadRequest, "invalid_edge_id", "Edge ID is invalid")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var edge infra.Edge
		if !desktopInfraDecode(w, r, &edge) {
			return
		}
		if edge.ID != tail {
			desktopInfraError(w, http.StatusBadRequest, "invalid_edge_id", "Edge ID does not match the route")
			return
		}
		updated, err := store.UpdateEdge(r.Context(), edge)
		if err != nil {
			desktopInfraStoreError(w, err)
			return
		}
		desktopInfraJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		version, ok := desktopInfraVersionQuery(w, r)
		if !ok {
			return
		}
		if err := store.DeleteEdge(r.Context(), tail, version); err != nil {
			desktopInfraStoreError(w, err)
			return
		}
		desktopInfraJSON(w, http.StatusOK, map[string]any{"status": "deleted", "id": tail})
	default:
		desktopInfraMethodNotAllowed(w)
	}
}

func desktopInfraSaveLayout(w http.ResponseWriter, r *http.Request, s *Server, cfg *config.Config, store *infra.Store) {
	var request infra.LayoutRequest
	if !desktopInfraDecode(w, r, &request) {
		return
	}
	updated, err := store.SaveLayout(r.Context(), request)
	if err != nil {
		desktopInfraStoreError(w, err)
		return
	}
	if updated == nil {
		updated = []infra.Node{}
	}
	updated = desktopInfraProjectNodes(r.Context(), s, cfg, updated)
	desktopInfraJSON(w, http.StatusOK, map[string]any{"nodes": updated})
}

func desktopInfraCapabilities(cfg *config.Config) infra.Capabilities {
	if cfg == nil {
		return infra.Capabilities{}
	}
	write := cfg.VirtualDesktop.Enabled && cfg.VirtualDesktop.Infra.Enabled && !cfg.VirtualDesktop.ReadOnly && !cfg.VirtualDesktop.Infra.ReadOnly
	return infra.Capabilities{
		Write:      write,
		SourceEdit: write && cfg.VirtualDesktop.Infra.AllowSourceEdit,
		Control:    write && cfg.VirtualDesktop.Infra.AllowControl,
	}
}

func desktopInfraDecode(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, desktopInfraBodyLimit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			desktopInfraError(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request body is too large")
			return false
		}
		desktopInfraError(w, http.StatusBadRequest, "invalid_request", "Request body is invalid")
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		desktopInfraError(w, http.StatusBadRequest, "invalid_request", "Request body must contain one JSON value")
		return false
	}
	return true
}

func desktopInfraVersionQuery(w http.ResponseWriter, r *http.Request) (int64, bool) {
	value := r.URL.Query().Get("version")
	version, err := strconv.ParseInt(value, 10, 64)
	if err != nil || version <= 0 {
		desktopInfraError(w, http.StatusBadRequest, "invalid_version", "A positive version is required")
		return 0, false
	}
	return version, true
}

// desktopInfraFlagQuery reads an optional boolean query flag ("1"/"true" or
// "0"/"false"; absent means false). Anything else is refused so a typo never
// silently drops the requested behaviour.
func desktopInfraFlagQuery(w http.ResponseWriter, r *http.Request, name string) (bool, bool) {
	switch r.URL.Query().Get(name) {
	case "":
		return false, true
	case "1", "true":
		return true, true
	case "0", "false":
		return false, true
	default:
		desktopInfraError(w, http.StatusBadRequest, "invalid_request", "Query flag "+name+" must be 1 or 0")
		return false, false
	}
}

func desktopInfraValidPathID(id string) bool {
	return id != "" && len(id) <= 128 && !strings.ContainsAny(id, "/\\\x00") && id != "." && id != ".."
}

func desktopInfraStoreError(w http.ResponseWriter, err error) {
	status, code, message := desktopInfraStoreErrorStatus(err)
	desktopInfraError(w, status, code, message)
}

// desktopInfraStoreErrorStatus maps a store error to a status, a stable code and
// a sanitized message. Batch imports report the same codes per item.
func desktopInfraStoreErrorStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, infra.ErrConflict):
		return http.StatusConflict, "conflict", "The item changed; refresh and try again"
	case errors.Is(err, infra.ErrNotFound), errors.Is(err, infra.ErrSourceNotFound):
		return http.StatusNotFound, "not_found", "The Infra item was not found"
	case errors.Is(err, infra.ErrSourceRemoved), errors.Is(err, infra.ErrSourceLinked):
		return http.StatusConflict, "source_unavailable", "The source is no longer available"
	case errors.Is(err, infra.ErrHasChildren):
		return http.StatusConflict, "has_children", "Move or remove child nodes first"
	case errors.Is(err, infra.ErrEdgeExists):
		return http.StatusConflict, "edge_exists", "This connection already exists"
	case errors.Is(err, infra.ErrReadOnly):
		return http.StatusForbidden, "source_readonly", "This source is read-only"
	case errors.Is(err, infra.ErrValidation):
		return http.StatusBadRequest, "invalid_request", "Infra data is invalid"
	default:
		return http.StatusInternalServerError, "infra_error", "Infra could not complete the request"
	}
}

func desktopInfraProviderError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, infra.ErrConflict):
		desktopInfraError(w, http.StatusConflict, "conflict", "The source changed; refresh before editing")
	case errors.Is(err, infra.ErrSourceRemoved), errors.Is(err, infra.ErrSourceNotFound):
		desktopInfraError(w, http.StatusNotFound, "source_unavailable", "The source is no longer available")
	default:
		// Provider errors can contain URLs, credentials, or raw response details.
		desktopInfraError(w, http.StatusBadGateway, "provider_unavailable", "The source could not be read")
	}
}

func desktopInfraHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
}

func desktopInfraJSON(w http.ResponseWriter, status int, value any) {
	desktopInfraHeaders(w)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func desktopInfraError(w http.ResponseWriter, status int, code, message string) {
	desktopInfraJSON(w, status, map[string]string{"error": code, "message": message})
}

func desktopInfraMethodNotAllowed(w http.ResponseWriter) {
	desktopInfraError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
}

func desktopInfraRefreshConfigured(ctx context.Context, s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime, force bool) {
	refreshes := []func(context.Context){
		func(ctx context.Context) { desktopInfraRefreshRegistry(ctx, s, cfg, store, runtime, force) },
	}
	if cfg.Docker.Enabled {
		refreshes = append(refreshes, func(ctx context.Context) { desktopInfraRefreshDocker(ctx, cfg, store, runtime, force) })
	}
	if cfg.Proxmox.Enabled {
		refreshes = append(refreshes, func(ctx context.Context) { desktopInfraRefreshProxmox(ctx, cfg, store, runtime, force) })
	}
	if desktopInfraFritzAllowed(cfg) {
		refreshes = append(refreshes, func(ctx context.Context) { desktopInfraRefreshFritz(ctx, s, cfg, store, runtime, force) })
	}
	desktopInfraRunRefreshes(ctx, 15*time.Second, refreshes...)
}

func desktopInfraRunRefreshes(ctx context.Context, timeout time.Duration, refreshes ...func(context.Context)) {
	if len(refreshes) == 0 {
		return
	}
	refreshCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var wait sync.WaitGroup
	wait.Add(len(refreshes))
	for _, refresh := range refreshes {
		refresh := refresh
		go func() {
			defer wait.Done()
			refresh(refreshCtx)
		}()
	}
	wait.Wait()
}

// desktopInfraRefreshSource performs a fresh, bounded enumeration for the
// provider behind an import candidate. Import is the explicit point where a
// cached projection must be revalidated against its current source identity.
func desktopInfraRefreshSource(ctx context.Context, s *Server, cfg *config.Config, store *infra.Store, runtime *desktopInfraRuntime, identity infra.SourceIdentity) {
	switch identity.Kind {
	case "registry":
		if identity.Instance == "inventory" {
			desktopInfraRefreshRegistry(ctx, s, cfg, store, runtime, true)
		}
	case "docker":
		if cfg.Docker.Enabled && identity.Instance == desktopInfraDockerInstance(cfg) {
			desktopInfraRefreshDocker(ctx, cfg, store, runtime, true)
		}
	case "proxmox":
		if cfg.Proxmox.Enabled && identity.Instance == desktopInfraProxmoxInstance(cfg) {
			desktopInfraRefreshProxmox(ctx, cfg, store, runtime, true)
		}
	case "fritzbox":
		if desktopInfraFritzAllowed(cfg) && identity.Instance == desktopInfraFritzInstance(cfg) {
			desktopInfraRefreshFritz(ctx, s, cfg, store, runtime, true)
		}
	}
}

func desktopInfraConfiguredSources(ctx context.Context, s *Server, cfg *config.Config, store *infra.Store, includeIgnored bool) ([]infra.Source, error) {
	sources := make([]infra.Source, 0)
	providers := []struct {
		kind, instance string
		enabled        bool
	}{
		{"registry", "inventory", s.InventoryDB != nil},
		{"docker", desktopInfraDockerInstance(cfg), cfg.Docker.Enabled},
		{"proxmox", desktopInfraProxmoxInstance(cfg), cfg.Proxmox.Enabled},
		{"fritzbox", desktopInfraFritzInstance(cfg), desktopInfraFritzAllowed(cfg)},
	}
	list := store.ListSources
	if includeIgnored {
		list = store.ListSourcesIncludingIgnored
	}
	for _, provider := range providers {
		if !provider.enabled {
			continue
		}
		found, err := list(ctx, provider.kind, provider.instance)
		if err != nil {
			return nil, err
		}
		sources = append(sources, found...)
	}
	if sources == nil {
		sources = []infra.Source{}
	}
	return sources, nil
}

func desktopInfraContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 15*time.Second)
}

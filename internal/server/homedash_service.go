package server

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/homedash"
)

// homeDashService is the server's HomeDash runtime: hub, homedash.db store and widget catalog. It is
// built once at start (initHomeDash) on the server lifetime context and closed after the HTTP drain
// (closeHomeDash). The store path is fixed at start; a data_dir change needs a restart.
type homeDashService struct {
	hub      *homedash.Hub
	store    *homedash.Store
	cat      homedash.WidgetCatalog
	specs    []homeDashSourceSpec
	coalesce time.Duration // the stream writer flushes at most once per interval
	log      *slog.Logger
	ctx      context.Context    // service lifetime: the server context, cancelled by close; background catalog refreshes stop with it
	cancel   context.CancelFunc // ends ctx
	haProbe  atomic.Bool        // one background HA catalog refresh at a time (templates)
	streams  atomic.Int64       // open SSE streams (endHomeDashStreams waits for them, Task 12)
}

// homeDashDeps are the parts newHomeDashService does not build itself.
type homeDashDeps struct {
	Logger     *slog.Logger
	Sources    []homeDashSourceSpec
	Audit      func(ctx context.Context, cmd homedash.Command, outcome string)
	HubOptions homedash.Options // Logger and Audit are taken from the fields above
}

func newHomeDashService(parent context.Context, dbPath string, cfg *config.Config, deps homeDashDeps) (*homeDashService, error) {
	store, err := homedash.OpenStore(dbPath)
	if err != nil {
		return nil, err
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	opts := deps.HubOptions
	opts.Logger, opts.Audit = logger, deps.Audit
	coalesce := opts.CoalesceInterval
	if coalesce <= 0 {
		coalesce = 100 * time.Millisecond
	}
	hub := homedash.NewHub(parent, opts)
	for _, spec := range deps.Sources {
		hub.Register(spec.ID, spec.Factory)
	}
	sctx, cancel := context.WithCancel(parent)
	svc := &homeDashService{hub: hub, store: store, cat: homedash.Catalog(), specs: deps.Sources, coalesce: coalesce, log: logger, ctx: sctx, cancel: cancel}
	svc.reconfigure(cfg)
	return svc, nil
}

// reconfigure publishes a configuration. It never blocks, so replaceConfigSnapshot may call it
// under CfgMu: the hub only stores the snapshot and wakes its own goroutine.
func (svc *homeDashService) reconfigure(cfg *config.Config) {
	svc.store.SetMaxDashboards(cfg.HomeDash.Limits.MaxDashboards)
	svc.hub.Reconfigure(homeDashRuntimeConfig(cfg, svc.specs))
}

// close ends every stream ("shutdown"), stops the sources (waits up to 10 s) and closes the store.
func (svc *homeDashService) close() {
	svc.cancel()
	svc.hub.Close()
	if err := svc.store.Close(); err != nil {
		svc.log.Warn("[HomeDash] Store did not close cleanly", "error", err)
	}
}

// homeDashRuntimeConfig builds the hub's immutable snapshot. It reads cfg only (no lock).
func homeDashRuntimeConfig(cfg *config.Config, specs []homeDashSourceSpec) homedash.RuntimeConfig {
	hd := cfg.HomeDash
	rc := homedash.RuntimeConfig{
		Enabled:           hd.Enabled,
		ReadOnly:          hd.ReadOnly,
		DesktopReadOnly:   cfg.VirtualDesktop.ReadOnly,
		EggMode:           cfg.EggMode.Enabled,
		KioskEnabled:      hd.Kiosk.Enabled,
		KioskAllowControl: hd.Kiosk.AllowControl,
		Limits: homedash.Limits{
			MaxDashboards: hd.Limits.MaxDashboards, MaxStreams: hd.Limits.MaxStreams, MaxKioskStreams: hd.Limits.MaxKioskStreams,
			MaxSubscriptions: hd.Limits.MaxSubscriptions, CommandsPerMinute: hd.Limits.CommandsPerMinute,
		},
		History: homedash.HistoryConfig{RingPoints: hd.History.RingPoints, RingEntities: hd.History.RingEntities, SampleSeconds: hd.History.SampleSeconds},
		Sources: make(map[homedash.SourceID]homedash.SourceSettings, len(specs)),
	}
	for _, spec := range specs {
		set := spec.Settings(cfg)
		set.ConfigKey = spec.ConfigKey
		rc.Sources[spec.ID] = set
	}
	return rc
}

// initHomeDash opens homedash.db and starts the hub on the server lifetime context. A failure is
// logged and leaves HomeDash unavailable (routes answer 503 storage_unavailable); the server runs on.
// The publication runs under CfgMu (read) so no config change is lost or applied twice.
func (s *Server) initHomeDash(ctx context.Context) {
	if s == nil || ctx.Err() != nil {
		return
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil {
		return
	}
	path := config.SQLiteArchivePaths(cfg)[config.SQLiteArchivePrefix+"homedash.db"]
	if path == "" {
		s.logHomeDash("HomeDash has no data directory and stays unavailable", nil)
		return
	}
	svc, err := newHomeDashService(ctx, path, cfg, homeDashDeps{Logger: s.Logger, Sources: s.homeDashSourceSpecs(), Audit: s.homeDashAudit})
	if err != nil {
		s.logHomeDash("HomeDash store could not be opened", err)
		return
	}
	s.CfgMu.RLock()
	s.homeDash.Store(svc)
	// Writers hold CfgMu (write) across replaceConfigSnapshot, which calls the HomeDash hook before it
	// publishes the snapshot: under the read lock no publication is half done, so latest is current
	// and no reconfigure of a writer can interleave (lead amendment A2).
	if latest := s.ConfigSnapshot(); latest != nil {
		svc.reconfigure(latest)
	}
	s.CfgMu.RUnlock()
}

// closeHomeDash withdraws and closes the service. Idempotent; Start defers it right after
// initHomeDash, so it runs after the HTTP drain and before the integrations close.
func (s *Server) closeHomeDash() {
	if s == nil {
		return
	}
	if svc := s.homeDash.Swap(nil); svc != nil {
		svc.close()
	}
}

func (s *Server) logHomeDash(message string, err error) {
	if s != nil && s.Logger != nil {
		s.Logger.Warn("[HomeDash] "+message, "error", err)
	}
}

// homeDashAuditKey carries the HTTP attribution of a command into the hub's audit callback.
type homeDashAuditKey struct{}

// homeDashAudit writes one desktop audit row per command outcome (ok, superseded, refused:<code>,
// failed). Best effort: a failure is logged and never changes the command's answer.
func (s *Server) homeDashAudit(ctx context.Context, cmd homedash.Command, outcome string) {
	info, _ := ctx.Value(homeDashAuditKey{}).(desktop.AuditRequestInfo)
	actx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	svc, _, err := s.getDesktopService(actx)
	if err == nil {
		err = svc.AuditWithRequest(actx, "homedash.command", string(cmd.Entity), map[string]string{
			"command": string(cmd.Name), "outcome": outcome, "dashboard_id": cmd.DashboardID,
			"widget_id": cmd.WidgetID, "principal": string(cmd.Principal.Kind),
		}, desktop.SourceUser, info)
	}
	if err != nil {
		s.logHomeDash("Command audit could not be written", err)
	}
}

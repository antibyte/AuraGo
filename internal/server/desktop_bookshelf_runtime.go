package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"aurago/internal/bookshelf"
	"aurago/internal/config"
	"aurago/internal/fileutil"
	"aurago/internal/security"
	"aurago/internal/smbclient"
	"aurago/internal/tools"
)

const (
	bookshelfWorkspaceSourceID = "workspace"
	bookshelfImportsSourceID   = "bookshelf-imports"
	bookshelfImportsMountID    = "bookshelf-imports"
	bookshelfCacheBytes        = int64(1 << 30)
	bookshelfMinFreeBytes      = int64(64 << 20)
)

type desktopBookshelfRuntime struct {
	service         *bookshelf.Service
	adapter         bookshelf.SourceAdapter
	root            string
	importsDir      string
	maxBookBytes    atomic.Int64
	jobs            map[string]*bookshelfJob
	tickets         map[string]*bookshelfTicket
	schedulerCancel context.CancelFunc
	closed          bool
}

type bookshelfJob struct {
	ID        string               `json:"job_id"`
	Kind      string               `json:"kind,omitempty"`
	BookID    string               `json:"book_id,omitempty"`
	SourceID  string               `json:"source_id"`
	Status    string               `json:"status"`
	Scanned   int                  `json:"scanned"`
	Imported  int                  `json:"imported"`
	Skipped   int                  `json:"skipped"`
	Error     string               `json:"error,omitempty"`
	Rendition *bookshelf.Rendition `json:"rendition,omitempty"`
	StartedAt time.Time            `json:"started_at"`
	EndedAt   time.Time            `json:"finished_at,omitempty"`
	cancel    context.CancelFunc
}

type bookshelfTicket struct {
	mu               sync.Mutex
	revoked          atomic.Bool
	BookID           string
	SourceID         string
	SourceGeneration int64
	Path             string
	Size             int64
	SourceSize       int64
	BookRevision     int64
	CatalogRevision  string
	Revision         string
	BaseRevision     string
	Format           string
	Pinned           bool
	Expires          time.Time
	expiryTimer      *time.Timer
	expiryGeneration uint64
	Lease            *bookshelf.Lease
}

func bookshelfEffectiveMaxBytes(cfg *config.Config) int64 {
	limit := bookshelf.MaxSourceBookBytes
	if cfg == nil {
		return limit
	}
	maxMB := cfg.VirtualDesktop.MaxFileSizeMB
	if maxMB <= 0 {
		maxMB = 50
	}
	if int64(maxMB) < limit/(1<<20) {
		limit = int64(maxMB) << 20
	}
	return limit
}

func (s *Server) bookshelfMaxBytes() int64 {
	return bookshelfEffectiveMaxBytes(s.ConfigSnapshot())
}

func (s *Server) desktopBookshelf(ctx context.Context) (*desktopBookshelfRuntime, error) {
	if s == nil {
		return nil, errors.New("bookshelf unavailable")
	}
	cfg := s.ConfigSnapshot()
	if cfg == nil || !cfg.VirtualDesktop.Enabled || !cfg.VirtualDesktop.BookshelfEnabled {
		return nil, errBookshelfDisabled
	}
	if _, _, err := s.getDesktopService(ctx); err != nil {
		return nil, err
	}
	s.bookshelfMu.Lock()
	defer s.bookshelfMu.Unlock()
	if s.bookshelf != nil && !s.bookshelf.closed {
		limit := bookshelfEffectiveMaxBytes(cfg)
		if err := s.bookshelf.service.SetMaxBookBytes(limit); err != nil {
			return nil, err
		}
		s.bookshelf.maxBookBytes.Store(limit)
		return s.bookshelf, nil
	}
	root := filepath.Join(cfg.Directories.DataDir, "bookshelf")
	importsDir := filepath.Join(root, "imports")
	if err := os.MkdirAll(importsDir, 0o700); err != nil {
		return nil, fmt.Errorf("create Bookshelf managed imports directory: %w", err)
	}
	adapter := &desktopBookshelfAdapter{s: s, importsDir: importsDir}
	service, err := bookshelf.New(ctx, bookshelf.Options{
		Root: root, Adapters: map[string]bookshelf.SourceAdapter{
			bookshelf.ProviderLocal:  adapter,
			bookshelf.ProviderWebDAV: adapter,
			bookshelf.ProviderKoofr:  adapter,
			bookshelf.ProviderSMB:    adapter,
		},
		MaxFilesPerScan: 10_000,
		MaxBookBytes:    bookshelfEffectiveMaxBytes(cfg),
		MaxCacheBytes:   bookshelfCacheBytes,
		MinFreeBytes:    bookshelfMinFreeBytes,
	})
	if err != nil {
		return nil, err
	}
	runtime := &desktopBookshelfRuntime{service: service, adapter: adapter, root: root, importsDir: importsDir, jobs: make(map[string]*bookshelfJob), tickets: make(map[string]*bookshelfTicket)}
	runtime.maxBookBytes.Store(bookshelfEffectiveMaxBytes(cfg))
	sources, err := service.ListSources(ctx)
	if err != nil {
		_ = service.Close()
		return nil, err
	}
	haveWorkspace, haveImports := false, false
	for _, source := range sources {
		haveWorkspace = haveWorkspace || source.ID == bookshelfWorkspaceSourceID
		haveImports = haveImports || source.ID == bookshelfImportsSourceID
	}
	if !haveWorkspace {
		if _, err := service.AddSource(ctx, bookshelf.Source{ID: bookshelfWorkspaceSourceID, Name: "Desktop workspace", Provider: bookshelf.ProviderLocal, MountID: "workspace"}); err != nil {
			_ = service.Close()
			return nil, err
		}
	}
	if !haveImports {
		if _, err := service.AddSource(ctx, bookshelf.Source{ID: bookshelfImportsSourceID, Name: "Bookshelf uploads", Provider: bookshelf.ProviderLocal, MountID: bookshelfImportsMountID}); err != nil {
			_ = service.Close()
			return nil, err
		}
	}
	s.bookshelf = runtime
	schedulerParent := s.integrationCtx
	if schedulerParent == nil {
		schedulerParent = context.Background()
	}
	schedulerCtx, schedulerCancel := context.WithCancel(schedulerParent)
	runtime.schedulerCancel = schedulerCancel
	go s.runBookshelfRefreshScheduler(runtime, schedulerCtx)
	return runtime, nil
}

func (s *Server) updateDesktopBookshelfMaxBytes(cfg *config.Config) error {
	if s == nil || cfg == nil {
		return nil
	}
	limit := bookshelfEffectiveMaxBytes(cfg)
	s.bookshelfMu.Lock()
	defer s.bookshelfMu.Unlock()
	runtime := s.bookshelf
	if runtime == nil || runtime.closed || runtime.service == nil {
		return nil
	}
	if err := runtime.service.SetMaxBookBytes(limit); err != nil {
		return err
	}
	runtime.maxBookBytes.Store(limit)
	return nil
}

func (s *Server) startDesktopBookshelfAtStartup(ctx context.Context) {
	if s == nil || ctx == nil || !s.bookshelfEnabled() {
		return
	}
	go func() {
		if ctx.Err() != nil {
			return
		}
		storageCtx, cancel := s.storageContext(ctx)
		defer cancel()
		if _, err := s.desktopBookshelf(storageCtx); err != nil && storageCtx.Err() == nil {
			logger := s.Logger
			if logger == nil {
				logger = slog.Default()
			}
			logger.Warn("Bookshelf runtime initialization failed", "error", err)
		}
	}()
}

func (s *Server) shutdownDesktopBookshelf() {
	if s == nil {
		return
	}
	s.bookshelfMu.Lock()
	runtime := s.bookshelf
	s.bookshelf = nil
	var tickets []*bookshelfTicket
	if runtime != nil {
		runtime.closed = true
		if runtime.schedulerCancel != nil {
			runtime.schedulerCancel()
		}
		for _, job := range runtime.jobs {
			if job.cancel != nil {
				job.cancel()
			}
		}
		for _, ticket := range runtime.tickets {
			ticket.revoked.Store(true)
			tickets = append(tickets, ticket)
		}
		runtime.tickets = make(map[string]*bookshelfTicket)
	}
	s.bookshelfMu.Unlock()
	closeBookshelfTickets(tickets)
	if runtime != nil && runtime.service != nil {
		_ = runtime.service.Close()
	}
}

func closeBookshelfTickets(tickets []*bookshelfTicket) {
	for _, ticket := range tickets {
		ticket.mu.Lock()
		stopBookshelfTicketExpiryLocked(ticket)
		if ticket.Lease != nil {
			_ = ticket.Lease.Close()
			ticket.Lease = nil
		}
		ticket.mu.Unlock()
	}
}

func (s *Server) revokeDesktopBookshelfTickets() {
	if s == nil {
		return
	}
	s.bookshelfMu.Lock()
	runtime := s.bookshelf
	var tickets []*bookshelfTicket
	if runtime != nil {
		for _, ticket := range runtime.tickets {
			ticket.revoked.Store(true)
			tickets = append(tickets, ticket)
		}
		runtime.tickets = make(map[string]*bookshelfTicket)
	}
	s.bookshelfMu.Unlock()
	closeBookshelfTickets(tickets)
}

func (s *Server) revokeDesktopBookshelfSourceTickets(runtime *desktopBookshelfRuntime, sourceID string) {
	if s == nil || runtime == nil || sourceID == "" {
		return
	}
	s.bookshelfMu.Lock()
	var tickets []*bookshelfTicket
	if s.bookshelf == runtime {
		for id, ticket := range runtime.tickets {
			if ticket.SourceID == sourceID {
				ticket.revoked.Store(true)
				tickets = append(tickets, ticket)
				delete(runtime.tickets, id)
			}
		}
	}
	s.bookshelfMu.Unlock()
	closeBookshelfTickets(tickets)
}

func (s *Server) cancelDesktopBookshelfJobs() {
	if s == nil {
		return
	}
	s.bookshelfMu.Lock()
	defer s.bookshelfMu.Unlock()
	if s.bookshelf == nil {
		return
	}
	for _, job := range s.bookshelf.jobs {
		if job.cancel != nil {
			job.cancel()
		}
	}
}

func (s *Server) runBookshelfRefreshScheduler(runtime *desktopBookshelfRuntime, ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	lastStart := make(map[string]time.Time)
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			cfg := s.ConfigSnapshot()
			if cfg == nil || !cfg.VirtualDesktop.Enabled || !cfg.VirtualDesktop.BookshelfEnabled || cfg.VirtualDesktop.ReadOnly || runtime.closed {
				continue
			}
			sources, err := runtime.service.ListSources(ctx)
			if err != nil {
				continue
			}
			for _, source := range sources {
				minutes := source.RefreshMinutes
				if minutes < 15 || minutes > 10_080 || source.ID == bookshelfWorkspaceSourceID || source.ID == bookshelfImportsSourceID {
					continue
				}
				previous, ok := lastStart[source.ID]
				if !ok {
					lastStart[source.ID] = now
					continue
				}
				if now.Sub(previous) < time.Duration(minutes)*time.Minute {
					continue
				}
				if _, err := s.startBookshelfScanJob(runtime, source.ID, nil); err == nil {
					lastStart[source.ID] = now
				}
			}
		}
	}
}

func (s *Server) bookshelfEnabled() bool {
	cfg := s.ConfigSnapshot()
	return cfg != nil && cfg.VirtualDesktop.Enabled && cfg.VirtualDesktop.BookshelfEnabled
}

type desktopBookshelfAdapter struct {
	s          *Server
	importsDir string
}

func (a *desktopBookshelfAdapter) List(ctx context.Context, source bookshelf.Source, dir string) ([]bookshelf.Entry, error) {
	if err := a.authorized(ctx); err != nil {
		return nil, err
	}
	rel, err := joinBookshelfPath(source.Path, dir)
	if err != nil {
		return nil, err
	}
	switch source.Provider {
	case bookshelf.ProviderLocal:
		if source.MountID == bookshelfImportsMountID {
			return listBookshelfImports(a.importsDir, rel)
		}
		if source.MountID != "workspace" {
			return nil, tools.ErrStorageAccessDenied
		}
		svc, _, err := a.s.getDesktopService(ctx)
		if err != nil {
			return nil, err
		}
		workspacePath := rel
		if source.Path != "" && dir != "" {
			workspacePath = path.Join(source.Path, dir)
		} else if source.Path != "" {
			workspacePath = source.Path
		}
		entries, err := svc.ListFiles(ctx, workspacePath)
		if err != nil {
			return nil, err
		}
		out := make([]bookshelf.Entry, 0, len(entries))
		for _, item := range entries {
			if item.Mount != "" || item.Name == "" || item.Name == "." || item.Name == ".." {
				continue
			}
			entryPath := path.Join(dir, item.Name)
			out = append(out, bookshelf.Entry{Path: entryPath, Name: item.Name, Format: formatBookshelfPath(item.Name), Size: item.Size, Revision: localFileRevision(item.Size, item.Modified), IsDir: item.Type == "directory", Modified: item.Modified})
		}
		return out, nil
	case bookshelf.ProviderSMB:
		client, err := a.dialSMB(ctx, source)
		if err != nil {
			return nil, bookshelfSourceProviderError(err)
		}
		defer client.Close(context.Background())
		entries, err := client.List(ctx, rel)
		if err != nil {
			return nil, bookshelfSourceProviderError(err)
		}
		out := make([]bookshelf.Entry, 0, len(entries))
		for _, item := range entries {
			entryPath := strings.TrimPrefix(item.Path, strings.Trim(source.Path, "/")+"/")
			if source.Path == "" {
				entryPath = item.Path
			}
			out = append(out, bookshelf.Entry{Path: entryPath, Name: item.Name, Format: formatBookshelfPath(item.Name), Size: item.Size, Revision: localFileRevision(item.Size, item.Modified), IsDir: item.IsDir, Modified: item.Modified})
		}
		return out, nil
	case bookshelf.ProviderWebDAV, bookshelf.ProviderKoofr:
		d, loc, err := a.cloudSession(ctx, source, rel)
		if err != nil {
			return nil, err
		}
		entries, err := d.cloud.List(ctx, loc)
		if err != nil {
			return nil, bookshelfSourceProviderError(err)
		}
		out := make([]bookshelf.Entry, 0, len(entries))
		for _, item := range entries {
			name := item.Name
			if name == "" || strings.ContainsAny(name, "/\\") {
				continue
			}
			modified, _ := time.Parse(time.RFC3339, item.Modified)
			out = append(out, bookshelf.Entry{Path: path.Join(dir, name), Name: name, Format: formatBookshelfPath(name), Size: item.Size, Revision: item.Version, IsDir: item.Type == "directory", Modified: modified})
		}
		return out, nil
	default:
		return nil, bookshelf.ErrUnsupported
	}
}

func (a *desktopBookshelfAdapter) Stat(ctx context.Context, source bookshelf.Source, raw string) (bookshelf.Entry, error) {
	if err := a.authorized(ctx); err != nil {
		return bookshelf.Entry{}, err
	}
	rel, err := joinBookshelfPath(source.Path, raw)
	if err != nil {
		return bookshelf.Entry{}, err
	}
	switch source.Provider {
	case bookshelf.ProviderLocal:
		if source.MountID == bookshelfImportsMountID {
			entry, err := statBookshelfImport(a.importsDir, raw)
			return a.checkBookshelfEntry(entry, err)
		}
		if source.MountID != "workspace" {
			return bookshelf.Entry{}, tools.ErrStorageAccessDenied
		}
		svc, _, err := a.s.getDesktopService(ctx)
		if err != nil {
			return bookshelf.Entry{}, err
		}
		workspacePath, err := joinBookshelfPath(source.Path, raw)
		if err != nil {
			return bookshelf.Entry{}, err
		}
		file, item, _, err := svc.OpenPreviewFile(ctx, workspacePath)
		if err != nil {
			return bookshelf.Entry{}, err
		}
		_ = file.Close()
		entry := bookshelf.Entry{Path: raw, Name: item.Name, Format: formatBookshelfPath(item.Name), Size: item.Size, Revision: localFileRevision(item.Size, item.Modified), Modified: item.Modified}
		return a.checkBookshelfEntry(entry, nil)
	case bookshelf.ProviderSMB:
		client, err := a.dialSMB(ctx, source)
		if err != nil {
			return bookshelf.Entry{}, bookshelfSourceProviderError(err)
		}
		defer client.Close(context.Background())
		item, err := client.Stat(ctx, rel)
		if err != nil {
			return bookshelf.Entry{}, bookshelfSourceProviderError(err)
		}
		entry := bookshelf.Entry{Path: raw, Name: item.Name, Format: formatBookshelfPath(item.Name), Size: item.Size, Revision: localFileRevision(item.Size, item.Modified), IsDir: item.IsDir, Modified: item.Modified}
		return a.checkBookshelfEntry(entry, nil)
	case bookshelf.ProviderWebDAV, bookshelf.ProviderKoofr:
		d, loc, err := a.cloudSession(ctx, source, rel)
		if err != nil {
			return bookshelf.Entry{}, err
		}
		item, err := d.cloud.Stat(ctx, loc)
		if err != nil {
			return bookshelf.Entry{}, bookshelfSourceProviderError(err)
		}
		modified, _ := time.Parse(time.RFC3339, item.Modified)
		entry := bookshelf.Entry{Path: raw, Name: item.Name, Format: formatBookshelfPath(item.Name), Size: item.Size, Revision: item.Version, IsDir: item.Type == "directory", Modified: modified}
		return a.checkBookshelfEntry(entry, nil)
	default:
		return bookshelf.Entry{}, bookshelf.ErrUnsupported
	}
}

func (a *desktopBookshelfAdapter) checkBookshelfEntry(entry bookshelf.Entry, err error) (bookshelf.Entry, error) {
	if err != nil {
		return bookshelf.Entry{}, err
	}
	if !entry.IsDir && (entry.Size < 0 || entry.Size > a.s.bookshelfMaxBytes()) {
		return bookshelf.Entry{}, bookshelf.ErrLimitExceeded
	}
	return entry, nil
}

func (a *desktopBookshelfAdapter) Open(ctx context.Context, source bookshelf.Source, raw string) (io.ReadCloser, error) {
	if err := a.authorized(ctx); err != nil {
		return nil, err
	}
	rel, err := joinBookshelfPath(source.Path, raw)
	if err != nil {
		return nil, err
	}
	switch source.Provider {
	case bookshelf.ProviderLocal:
		if source.MountID == bookshelfImportsMountID {
			return openBookshelfImport(a.importsDir, raw)
		}
		if source.MountID != "workspace" {
			return nil, tools.ErrStorageAccessDenied
		}
		svc, _, err := a.s.getDesktopService(ctx)
		if err != nil {
			return nil, err
		}
		workspacePath, err := joinBookshelfPath(source.Path, raw)
		if err != nil {
			return nil, err
		}
		file, _, _, err := svc.OpenPreviewFile(ctx, workspacePath)
		return file, err
	case bookshelf.ProviderSMB:
		client, err := a.dialSMB(ctx, source)
		if err != nil {
			return nil, bookshelfSourceProviderError(err)
		}
		reader, err := client.Open(ctx, rel)
		if err != nil {
			_ = client.Close(context.Background())
			return nil, bookshelfSourceProviderError(err)
		}
		return reader, nil
	case bookshelf.ProviderWebDAV, bookshelf.ProviderKoofr:
		d, loc, err := a.cloudSession(ctx, source, rel)
		if err != nil {
			return nil, err
		}
		stat, err := d.cloud.Stat(ctx, loc)
		if err != nil {
			return nil, bookshelfSourceProviderError(err)
		}
		reader, err := d.cloud.Open(ctx, loc, stat.Version)
		return reader, bookshelfSourceProviderError(err)
	default:
		return nil, bookshelf.ErrUnsupported
	}
}

func (a *desktopBookshelfAdapter) authorized(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !a.s.bookshelfEnabled() {
		return bookshelf.ErrSourceRevoked
	}
	return nil
}

func bookshelfSourceAuthorization(err error) error {
	if err == nil {
		return nil
	}
	return bookshelfSourceProviderError(err)
}

func bookshelfSourceProviderError(err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, tools.ErrStorageAccessDenied) || errors.Is(err, tools.ErrStorageReadOnly) {
		return errors.Join(bookshelf.ErrSourceRevoked, err)
	}
	if errors.Is(err, tools.ErrStorageUnavailable) {
		return errors.Join(bookshelf.ErrSourceUnavailable, err)
	}
	var networkError net.Error
	if errors.As(err, &networkError) {
		return errors.Join(bookshelf.ErrSourceUnavailable, err)
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"logon failure", "authentication failed", "access denied", "status_logon_failure", "status_access_denied", "status_password_expired", "status_account_disabled"} {
		if strings.Contains(message, marker) {
			return errors.Join(bookshelf.ErrSourceRevoked, err)
		}
	}
	return err
}

func (a *desktopBookshelfAdapter) cloudSession(ctx context.Context, source bookshelf.Source, rel string) (*desktopStorageSession, tools.StorageLocation, error) {
	cfg := a.s.ConfigSnapshot()
	if cfg == nil {
		return nil, tools.StorageLocation{}, errBookshelfDisabled
	}
	svc, _, err := a.s.getDesktopService(ctx)
	if err != nil {
		return nil, tools.StorageLocation{}, err
	}
	cloud := desktopStorageCloud(cfg)
	loc := tools.StorageLocation{Provider: source.Provider, MountID: source.MountID, Path: rel}
	d := &desktopStorageSession{s: a.s, svc: svc, cloud: cloud, maxBytes: int64(svc.Config().MaxFileSizeMB) << 20, ctx: ctx}
	if err := d.authorize(loc, false); err != nil {
		return nil, tools.StorageLocation{}, bookshelfSourceAuthorization(err)
	}
	return d, loc, nil
}

func (a *desktopBookshelfAdapter) dialSMB(ctx context.Context, source bookshelf.Source) (*smbclient.Client, error) {
	if source.ID == bookshelfWorkspaceSourceID || source.ID == bookshelfImportsSourceID || source.Provider != bookshelf.ProviderSMB {
		return nil, tools.ErrStorageAccessDenied
	}
	if a.s.Vault == nil {
		return nil, errors.Join(bookshelf.ErrSourceRevoked, errors.New("SMB credentials are unavailable"))
	}
	password, err := a.s.Vault.ReadSecret(bookshelfSMBSecretKey(source.ID))
	if err != nil || strings.TrimSpace(password) == "" {
		return nil, errors.Join(bookshelf.ErrSourceRevoked, errors.New("SMB credentials are unavailable"))
	}
	security.RegisterSensitive(password)
	client, err := smbclient.Dial(ctx, smbclient.Config{Host: source.Host, Share: source.Share, Username: source.Username, Domain: source.Domain, Password: password})
	return client, bookshelfSourceProviderError(err)
}

func bookshelfSMBSecretKey(sourceID string) string { return "bookshelf_smb_" + sourceID + "_password" }

func joinBookshelfPath(root, relative string) (string, error) {
	for _, value := range []string{root, relative} {
		if strings.ContainsAny(value, "\\\x00") || strings.HasPrefix(value, "/") || strings.Contains(value, ":") {
			return "", bookshelf.ErrInvalidPath
		}
	}
	joined := path.Clean(path.Join(root, relative))
	if joined == "." {
		return "", nil
	}
	if joined == ".." || strings.HasPrefix(joined, "../") || path.IsAbs(joined) {
		return "", bookshelf.ErrInvalidPath
	}
	return joined, nil
}

func localFileRevision(size int64, modified time.Time) string {
	if modified.IsZero() {
		return ""
	}
	return fmt.Sprintf("local:%d:%d", size, modified.UnixNano())
}

func formatBookshelfPath(name string) string {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".fb2.zip") {
		return "fb2.zip"
	}
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(lower)), ".")
}

func listBookshelfImports(root, rel string) ([]bookshelf.Entry, error) {
	dir, err := safeBookshelfImportPath(root, rel, true)
	if err != nil {
		return nil, err
	}
	items, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	entries := make([]bookshelf.Entry, 0, len(items))
	for _, item := range items {
		info, err := item.Info()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() || item.Type()&os.ModeSymlink != 0 {
			continue
		}
		entries = append(entries, bookshelf.Entry{Path: path.Join(rel, item.Name()), Name: item.Name(), Format: formatBookshelfPath(item.Name()), Size: info.Size(), Revision: localFileRevision(info.Size(), info.ModTime()), Modified: info.ModTime()})
	}
	return entries, nil
}

func statBookshelfImport(root, raw string) (bookshelf.Entry, error) {
	file, err := safeBookshelfImportPath(root, raw, false)
	if err != nil {
		return bookshelf.Entry{}, err
	}
	info, err := os.Lstat(file)
	if err != nil {
		return bookshelf.Entry{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return bookshelf.Entry{}, errors.New("Bookshelf import is not a regular file")
	}
	return bookshelf.Entry{Path: raw, Name: filepath.Base(file), Format: formatBookshelfPath(file), Size: info.Size(), Revision: localFileRevision(info.Size(), info.ModTime()), Modified: info.ModTime()}, nil
}

func openBookshelfImport(root, raw string) (io.ReadCloser, error) {
	file, err := safeBookshelfImportPath(root, raw, false)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(file)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("Bookshelf import is not a regular file")
	}
	return os.Open(file)
}

func safeBookshelfImportPath(root, rel string, allowDirectory bool) (string, error) {
	if strings.ContainsAny(rel, "\\\x00") || strings.HasPrefix(rel, "/") || strings.Contains(rel, ":") {
		return "", bookshelf.ErrInvalidPath
	}
	clean := path.Clean(rel)
	if clean == "." {
		if allowDirectory {
			return root, nil
		}
		return "", bookshelf.ErrInvalidPath
	}
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", bookshelf.ErrInvalidPath
	}
	target := filepath.Join(root, filepath.FromSlash(clean))
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", bookshelf.ErrInvalidPath
	}
	for current := target; current != root; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) && allowDirectory {
				continue
			}
			if errors.Is(err, os.ErrNotExist) {
				return "", err
			}
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", bookshelf.ErrInvalidPath
		}
	}
	return target, nil
}

func stageBookshelfUpload(ctx context.Context, root, name string, input io.Reader, maxBytes int64) (string, int64, string, error) {
	if maxBytes <= 0 || maxBytes > bookshelf.MaxSourceBookBytes {
		maxBytes = bookshelf.MaxSourceBookBytes
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", 0, "", err
	}
	tmp, err := os.CreateTemp(root, ".import-*.tmp")
	if err != nil {
		return "", 0, "", err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(input, maxBytes+1))
	if copyErr != nil {
		_ = tmp.Close()
		return "", 0, "", copyErr
	}
	if written > maxBytes {
		_ = tmp.Close()
		return "", 0, "", bookshelf.ErrLimitExceeded
	}
	if err := ctx.Err(); err != nil {
		_ = tmp.Close()
		return "", 0, "", err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return "", 0, "", err
	}
	if err := tmp.Close(); err != nil {
		return "", 0, "", err
	}
	base := path.Base(strings.ReplaceAll(name, "\\", "/"))
	ext := strings.ToLower(filepath.Ext(base))
	if strings.HasSuffix(strings.ToLower(base), ".fb2.zip") {
		ext = ".fb2.zip"
	}
	if len(ext) > 16 || strings.ContainsAny(ext, "/\\\x00") {
		ext = ""
	}
	for _, ch := range ext {
		if ch != '.' && (ch < '0' || ch > '9') && (ch < 'a' || ch > 'z') {
			ext = ""
			break
		}
	}
	if ext == "" {
		return "", 0, "", bookshelf.ErrUnsupported
	}
	entryName := randomBookshelfID() + ext
	destination := filepath.Join(root, entryName)
	if err := fileutil.RenameContext(ctx, tmpPath, destination); err != nil {
		return "", 0, "", err
	}
	return entryName, written, "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

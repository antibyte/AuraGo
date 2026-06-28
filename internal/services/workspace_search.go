package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	slashpath "path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"aurago/internal/config"

	_ "modernc.org/sqlite"
)

const (
	workspaceSearchDBName             = "workspace_search.db"
	workspaceSearchMaxPatternChars    = 512
	workspaceSearchDefaultReadyWait   = 2 * time.Second
	workspaceSearchBinarySampleBytes  = 4096
	workspaceSearchDefaultFileSizeMB  = 10
	workspaceSearchDefaultIndexSizeMB = 256
	workspaceSearchDefaultMaxResults  = 100
)

var ErrWorkspaceSearchInvalidPattern = errors.New("workspace search invalid pattern")

// WorkspaceSearchRequest carries a single workspace search operation request.
type WorkspaceSearchRequest struct {
	Operation     string `json:"operation,omitempty"`
	Query         string `json:"query,omitempty"`
	Pattern       string `json:"pattern,omitempty"`
	Glob          string `json:"glob,omitempty"`
	Mode          string `json:"mode,omitempty"`
	OutputMode    string `json:"output_mode,omitempty"`
	CaseSensitive bool   `json:"case_sensitive,omitempty"`
	Limit         int    `json:"limit,omitempty"`
}

// WorkspaceSearchFileResult describes a file returned by find/glob/recent.
type WorkspaceSearchFileResult struct {
	Path           string  `json:"path"`
	Size           int64   `json:"size"`
	ModifiedAt     string  `json:"modified_at"`
	Score          float64 `json:"score,omitempty"`
	AccessCount    int     `json:"access_count,omitempty"`
	IndexedContent bool    `json:"indexed_content"`
}

// WorkspaceSearchMatch describes a single grep match.
type WorkspaceSearchMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
	Match   string `json:"match,omitempty"`
}

// WorkspaceSearchGrepResult is the structured output for grep.
type WorkspaceSearchGrepResult struct {
	Matches    []WorkspaceSearchMatch `json:"matches,omitempty"`
	Total      int                    `json:"total"`
	FilesCount int                    `json:"files_count"`
	ByFile     map[string]int         `json:"by_file,omitempty"`
}

// WorkspaceSearchStatus is safe to expose in tool/API responses.
type WorkspaceSearchStatus struct {
	Enabled             bool     `json:"enabled"`
	Running             bool     `json:"running"`
	Ready               bool     `json:"ready"`
	Root                string   `json:"root"`
	Files               int      `json:"files"`
	IndexedFiles        int      `json:"indexed_files"`
	ContentFiles        int      `json:"content_files"`
	IndexedContentBytes int64    `json:"indexed_content_bytes"`
	LastScanAt          string   `json:"last_scan_at,omitempty"`
	LastScanDurationMs  int64    `json:"last_scan_duration_ms,omitempty"`
	LastError           string   `json:"last_error,omitempty"`
	Excludes            []string `json:"excludes,omitempty"`
}

type workspaceIndexedFile struct {
	path           string
	absPath        string
	size           int64
	modTime        time.Time
	hash           string
	lines          []string
	indexedContent bool
	accessCount    int
}

type workspaceSearchIndex struct {
	files               map[string]*workspaceIndexedFile
	orderedPaths        []string
	contentFiles        int
	indexedContentBytes int64
}

type workspaceAccessStat struct {
	Count      int
	LastAccess time.Time
	LastWrite  time.Time
}

// WorkspaceSearchService keeps a resident, workspace-scoped file index.
type WorkspaceSearchService struct {
	cfg    *config.Config
	cfgMu  *sync.RWMutex
	logger *slog.Logger

	root string
	db   *sql.DB

	lifecycleMu sync.Mutex
	cancel      context.CancelFunc
	wg          sync.WaitGroup

	scanMu sync.Mutex
	mu     sync.RWMutex
	index  workspaceSearchIndex
	status WorkspaceSearchStatus

	ready     chan struct{}
	readyOnce sync.Once
}

// NewWorkspaceSearchService creates the workspace search service and its SQLite metadata store.
func NewWorkspaceSearchService(cfg *config.Config, cfgMu *sync.RWMutex, logger *slog.Logger) (*WorkspaceSearchService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if strings.TrimSpace(cfg.Directories.WorkspaceDir) == "" {
		return nil, fmt.Errorf("directories.workspace_dir is required")
	}
	if logger == nil {
		logger = slog.Default()
	}

	root, err := filepath.Abs(cfg.Directories.WorkspaceDir)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace dir: %w", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace dir: %w", err)
	}

	dataDir := cfg.Directories.DataDir
	if strings.TrimSpace(dataDir) == "" {
		dataDir = "data"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dbPath := filepath.Join(dataDir, workspaceSearchDBName)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open workspace search db: %w", err)
	}
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure workspace search db: %w", err)
	}
	if err := initWorkspaceSearchDB(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	s := &WorkspaceSearchService{
		cfg:    cfg,
		cfgMu:  cfgMu,
		logger: logger,
		root:   root,
		db:     db,
		ready:  make(chan struct{}),
		index: workspaceSearchIndex{
			files: map[string]*workspaceIndexedFile{},
		},
		status: WorkspaceSearchStatus{
			Enabled:  cfg.WorkspaceSearch.Enabled,
			Root:     root,
			Excludes: normalizedWorkspaceSearchExcludes(cfg.WorkspaceSearch.Exclude),
		},
	}
	return s, nil
}

func initWorkspaceSearchDB(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS workspace_file_access (
	path TEXT PRIMARY KEY,
	access_count INTEGER NOT NULL DEFAULT 0,
	last_access TEXT NOT NULL,
	last_write TEXT
)`)
	if err != nil {
		return fmt.Errorf("initialize workspace search db: %w", err)
	}
	return nil
}

// Start launches asynchronous initial scanning and background polling.
func (s *WorkspaceSearchService) Start(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if !s.enabled() {
		s.mu.Lock()
		s.status.Enabled = false
		s.status.Running = false
		s.mu.Unlock()
		return nil
	}

	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.cancel != nil {
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.mu.Lock()
	s.status.Enabled = true
	s.status.Running = true
	s.status.Root = s.root
	s.status.Excludes = normalizedWorkspaceSearchExcludes(s.cfg.WorkspaceSearch.Exclude)
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := s.Rescan(runCtx); err != nil && !errors.Is(err, context.Canceled) {
			s.logger.Warn("workspace search initial scan failed", "err", err)
		}
		ticker := time.NewTicker(s.pollInterval())
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if err := s.Rescan(runCtx); err != nil && !errors.Is(err, context.Canceled) {
					s.logger.Warn("workspace search poll scan failed", "err", err)
				}
			}
		}
	}()
	return nil
}

// Stop requests background scanning to stop and waits for it.
func (s *WorkspaceSearchService) Stop() {
	if s == nil {
		return
	}
	s.lifecycleMu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.lifecycleMu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
	s.mu.Lock()
	s.status.Running = false
	s.mu.Unlock()
}

// Close releases the SQLite handle.
func (s *WorkspaceSearchService) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Status returns a snapshot of the service state.
func (s *WorkspaceSearchService) Status() WorkspaceSearchStatus {
	if s == nil {
		return WorkspaceSearchStatus{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := s.status
	status.Excludes = append([]string(nil), status.Excludes...)
	return status
}

// Ready reports whether at least one scan has completed.
func (s *WorkspaceSearchService) Ready() bool {
	if s == nil {
		return false
	}
	return s.Status().Ready
}

// ReadyWithin waits briefly for the initial scan.
func (s *WorkspaceSearchService) ReadyWithin(timeout time.Duration) bool {
	if s == nil {
		return false
	}
	if timeout <= 0 {
		timeout = workspaceSearchDefaultReadyWait
	}
	if s.Ready() {
		return true
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-s.ready:
		return true
	case <-timer.C:
		return s.Ready()
	}
}

// Rescan rebuilds the in-memory index from the configured workspace.
func (s *WorkspaceSearchService) Rescan(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if !s.enabled() {
		s.mu.Lock()
		s.status.Enabled = false
		s.status.Ready = false
		s.status.LastError = ""
		s.mu.Unlock()
		return nil
	}

	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	start := time.Now()
	next, err := s.scanWorkspace(ctx)
	if err != nil {
		s.mu.Lock()
		s.status.LastError = err.Error()
		s.mu.Unlock()
		return err
	}
	stats, err := s.loadAccessStats()
	if err != nil {
		s.logger.Warn("workspace search frecency load failed", "err", err)
	}
	for rel, stat := range stats {
		if file := next.files[rel]; file != nil {
			file.accessCount = stat.Count
		}
	}

	s.mu.Lock()
	s.index = next
	s.status.Enabled = true
	s.status.Ready = true
	s.status.Root = s.root
	s.status.Files = len(next.files)
	s.status.IndexedFiles = len(next.files)
	s.status.ContentFiles = next.contentFiles
	s.status.IndexedContentBytes = next.indexedContentBytes
	s.status.LastScanAt = time.Now().UTC().Format(time.RFC3339)
	s.status.LastScanDurationMs = time.Since(start).Milliseconds()
	s.status.LastError = ""
	s.status.Excludes = normalizedWorkspaceSearchExcludes(s.cfg.WorkspaceSearch.Exclude)
	s.mu.Unlock()
	s.readyOnce.Do(func() { close(s.ready) })
	return nil
}

// Find returns files ranked by path match quality and frecency.
func (s *WorkspaceSearchService) Find(ctx context.Context, req WorkspaceSearchRequest) ([]WorkspaceSearchFileResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	snapshot := s.snapshot()
	query := firstNonEmpty(strings.TrimSpace(req.Query), strings.TrimSpace(req.Pattern))
	limit := s.limit(req.Limit)
	threshold := s.fuzzyThreshold()
	stats, _ := s.loadAccessStats()

	results := make([]WorkspaceSearchFileResult, 0, min(limit, len(snapshot.orderedPaths)))
	for _, rel := range snapshot.orderedPaths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file := snapshot.files[rel]
		if file == nil {
			continue
		}
		if req.Glob != "" && !workspaceMatchesAnyGlob(req.Glob, rel) {
			continue
		}
		baseScore := 1.0
		if query != "" {
			baseScore = workspacePathScore(query, rel)
			if baseScore < threshold && !strings.Contains(strings.ToLower(rel), strings.ToLower(query)) {
				continue
			}
		}
		stat := stats[rel]
		score := baseScore + workspaceFrecencyBoost(stat)
		results = append(results, workspaceFileResult(file, score, stat.Count))
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Path < results[j].Path
		}
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// Glob returns files matching a glob pattern without fuzzy scoring.
func (s *WorkspaceSearchService) Glob(ctx context.Context, req WorkspaceSearchRequest) ([]WorkspaceSearchFileResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pattern := strings.TrimSpace(req.Glob)
	if pattern == "" {
		pattern = strings.TrimSpace(req.Query)
	}
	if pattern == "" {
		return nil, fmt.Errorf("%w: glob is required", ErrWorkspaceSearchInvalidPattern)
	}
	limit := s.limit(req.Limit)
	snapshot := s.snapshot()
	stats, _ := s.loadAccessStats()
	results := make([]WorkspaceSearchFileResult, 0, min(limit, len(snapshot.orderedPaths)))
	for _, rel := range snapshot.orderedPaths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !workspaceMatchesAnyGlob(pattern, rel) {
			continue
		}
		stat := stats[rel]
		results = append(results, workspaceFileResult(snapshot.files[rel], workspaceFrecencyBoost(stat), stat.Count))
		if len(results) >= limit {
			break
		}
	}
	return results, nil
}

// Grep searches indexed text lines.
func (s *WorkspaceSearchService) Grep(ctx context.Context, req WorkspaceSearchRequest) (WorkspaceSearchGrepResult, error) {
	if err := ctx.Err(); err != nil {
		return WorkspaceSearchGrepResult{}, err
	}
	query := firstNonEmpty(strings.TrimSpace(req.Query), strings.TrimSpace(req.Pattern))
	if query == "" {
		return WorkspaceSearchGrepResult{}, fmt.Errorf("%w: query is required", ErrWorkspaceSearchInvalidPattern)
	}
	if len(query) > workspaceSearchMaxPatternChars {
		return WorkspaceSearchGrepResult{}, fmt.Errorf("%w: pattern too long (max %d characters)", ErrWorkspaceSearchInvalidPattern, workspaceSearchMaxPatternChars)
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		mode = "plain"
	}
	var re *regexp.Regexp
	if mode == "regex" {
		pattern := query
		if !req.CaseSensitive {
			pattern = "(?i)" + pattern
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return WorkspaceSearchGrepResult{}, fmt.Errorf("%w: invalid regex: %v", ErrWorkspaceSearchInvalidPattern, err)
		}
		re = compiled
	} else if mode != "plain" {
		return WorkspaceSearchGrepResult{}, fmt.Errorf("%w: grep supports mode plain or regex", ErrWorkspaceSearchInvalidPattern)
	}

	limit := s.limit(req.Limit)
	outputMode := strings.ToLower(strings.TrimSpace(req.OutputMode))
	countOnly := outputMode == "count"
	snapshot := s.snapshot()
	result := WorkspaceSearchGrepResult{ByFile: map[string]int{}}
	searchNeedle := query
	if !req.CaseSensitive && mode == "plain" {
		searchNeedle = strings.ToLower(query)
	}

	for _, rel := range snapshot.orderedPaths {
		if err := ctx.Err(); err != nil {
			return WorkspaceSearchGrepResult{}, err
		}
		file := snapshot.files[rel]
		if file == nil || !file.indexedContent {
			continue
		}
		if req.Glob != "" && !workspaceMatchesAnyGlob(req.Glob, rel) {
			continue
		}
		for i, line := range file.lines {
			matchText := ""
			if re != nil {
				matchText = re.FindString(line)
				if matchText == "" {
					continue
				}
			} else {
				haystack := line
				if !req.CaseSensitive {
					haystack = strings.ToLower(line)
				}
				idx := strings.Index(haystack, searchNeedle)
				if idx < 0 {
					continue
				}
				end := idx + len(searchNeedle)
				if end > len(line) {
					end = len(line)
				}
				matchText = line[idx:end]
			}
			result.Total++
			result.ByFile[rel]++
			if !countOnly && len(result.Matches) < limit {
				result.Matches = append(result.Matches, WorkspaceSearchMatch{
					File:    rel,
					Line:    i + 1,
					Content: strings.TrimRight(line, "\r\n"),
					Match:   matchText,
				})
			}
			if !countOnly && len(result.Matches) >= limit {
				break
			}
		}
		if !countOnly && len(result.Matches) >= limit {
			break
		}
	}
	result.FilesCount = len(result.ByFile)
	if len(result.ByFile) == 0 {
		result.ByFile = nil
	}
	return result, nil
}

// Recent returns files ordered by recorded access time.
func (s *WorkspaceSearchService) Recent(ctx context.Context, req WorkspaceSearchRequest) ([]WorkspaceSearchFileResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	limit := s.limit(req.Limit)
	rows, err := s.db.QueryContext(ctx, `SELECT path, access_count FROM workspace_file_access ORDER BY last_access DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query workspace search recent: %w", err)
	}
	defer rows.Close()
	snapshot := s.snapshot()
	results := []WorkspaceSearchFileResult{}
	for rows.Next() {
		var rel string
		var count int
		if err := rows.Scan(&rel, &count); err != nil {
			return nil, fmt.Errorf("scan workspace search recent: %w", err)
		}
		if file := snapshot.files[rel]; file != nil {
			results = append(results, workspaceFileResult(file, float64(count), count))
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspace search recent: %w", err)
	}
	return results, nil
}

// TrackAccess records successful reads/writes for ranking without storing file content.
func (s *WorkspaceSearchService) TrackAccess(path, kind string) error {
	if s == nil || strings.TrimSpace(path) == "" {
		return nil
	}
	rel, ok := s.toRelativePath(path)
	if !ok {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	lastWrite := (*string)(nil)
	if strings.EqualFold(kind, "write") {
		lastWrite = &now
	}
	if lastWrite != nil {
		_, err := s.db.Exec(`
INSERT INTO workspace_file_access(path, access_count, last_access, last_write)
VALUES(?, 1, ?, ?)
ON CONFLICT(path) DO UPDATE SET
	access_count = access_count + 1,
	last_access = excluded.last_access,
	last_write = excluded.last_write`, rel, now, *lastWrite)
		if err != nil {
			return fmt.Errorf("track workspace file access: %w", err)
		}
		return nil
	}
	_, err := s.db.Exec(`
INSERT INTO workspace_file_access(path, access_count, last_access)
VALUES(?, 1, ?)
ON CONFLICT(path) DO UPDATE SET
	access_count = access_count + 1,
	last_access = excluded.last_access`, rel, now)
	if err != nil {
		return fmt.Errorf("track workspace file access: %w", err)
	}
	return nil
}

// ExecuteTool serializes a workspace_search operation to AuraGo's common tool result shape.
func (s *WorkspaceSearchService) ExecuteTool(ctx context.Context, req WorkspaceSearchRequest) string {
	type toolResult struct {
		Status  string      `json:"status"`
		Message string      `json:"message,omitempty"`
		Data    interface{} `json:"data,omitempty"`
	}
	encode := func(result toolResult) string {
		data, err := json.Marshal(result)
		if err != nil {
			return `{"status":"error","message":"internal: result serialization failed"}`
		}
		return string(data)
	}
	if s == nil {
		return encode(toolResult{Status: "error", Message: "workspace_search service is not available"})
	}
	if !s.enabled() {
		return encode(toolResult{Status: "error", Message: "workspace_search is disabled"})
	}
	op := strings.ToLower(strings.TrimSpace(req.Operation))
	if op == "" {
		op = "find"
	}
	switch op {
	case "status":
		return encode(toolResult{Status: "success", Data: s.Status()})
	case "rescan":
		if err := s.Rescan(ctx); err != nil {
			return encode(toolResult{Status: "error", Message: err.Error()})
		}
		return encode(toolResult{Status: "success", Data: s.Status()})
	}
	if !s.ReadyWithin(workspaceSearchDefaultReadyWait) {
		return encode(toolResult{Status: "error", Message: "workspace_search index is still building"})
	}
	switch op {
	case "find":
		files, err := s.Find(ctx, req)
		if err != nil {
			return encode(toolResult{Status: "error", Message: err.Error()})
		}
		return encode(toolResult{Status: "success", Data: map[string]interface{}{"count": len(files), "files": files}})
	case "glob":
		files, err := s.Glob(ctx, req)
		if err != nil {
			return encode(toolResult{Status: "error", Message: err.Error()})
		}
		return encode(toolResult{Status: "success", Data: map[string]interface{}{"count": len(files), "files": files}})
	case "grep":
		result, err := s.Grep(ctx, req)
		if err != nil {
			return encode(toolResult{Status: "error", Message: err.Error()})
		}
		return encode(toolResult{Status: "success", Data: result})
	case "recent":
		files, err := s.Recent(ctx, req)
		if err != nil {
			return encode(toolResult{Status: "error", Message: err.Error()})
		}
		return encode(toolResult{Status: "success", Data: map[string]interface{}{"count": len(files), "files": files}})
	default:
		return encode(toolResult{Status: "error", Message: fmt.Sprintf("Unknown workspace_search operation '%s'. Valid: find, grep, glob, recent, rescan, status", op)})
	}
}

// ExecuteLegacyFileSearch returns the old file_search JSON shapes for delegated operations.
func (s *WorkspaceSearchService) ExecuteLegacyFileSearch(ctx context.Context, operation, pattern, filePath, glob, outputMode string) (string, bool) {
	type legacyResult struct {
		Status  string      `json:"status"`
		Message string      `json:"message,omitempty"`
		Data    interface{} `json:"data,omitempty"`
	}
	encode := func(result legacyResult) string {
		data, err := json.Marshal(result)
		if err != nil {
			return `{"status":"error","message":"internal: result serialization failed"}`
		}
		return string(data)
	}
	if s == nil || !s.enabled() || !s.ReadyWithin(workspaceSearchDefaultReadyWait) {
		return "", false
	}
	op := strings.ToLower(strings.TrimSpace(operation))
	switch op {
	case "find":
		effectiveGlob := glob
		if pattern != "" && (glob == "" || glob == "**/*") {
			effectiveGlob = pattern
		}
		files, err := s.Glob(ctx, WorkspaceSearchRequest{Glob: effectiveGlob, Limit: 1000})
		if err != nil {
			return encode(legacyResult{Status: "error", Message: err.Error()}), true
		}
		paths := make([]string, 0, len(files))
		for _, file := range files {
			paths = append(paths, file.Path)
		}
		return encode(legacyResult{Status: "success", Data: map[string]interface{}{"count": len(paths), "files": paths}}), true
	case "grep_recursive":
		if strings.TrimSpace(pattern) == "" {
			return encode(legacyResult{Status: "error", Message: "'pattern' is required for grep_recursive"}), true
		}
		result, err := s.Grep(ctx, WorkspaceSearchRequest{Query: pattern, Glob: glob, Mode: "regex", OutputMode: outputMode, Limit: 500})
		if err != nil {
			return encode(legacyResult{Status: "error", Message: err.Error()}), true
		}
		if strings.EqualFold(outputMode, "count") {
			return encode(legacyResult{Status: "success", Data: map[string]interface{}{
				"total":       result.Total,
				"files_count": result.FilesCount,
				"by_file":     result.ByFile,
			}}), true
		}
		matches := make([]map[string]interface{}, 0, len(result.Matches))
		for _, match := range result.Matches {
			matches = append(matches, map[string]interface{}{
				"file":    match.File,
				"line":    match.Line,
				"content": match.Content,
			})
		}
		return encode(legacyResult{Status: "success", Data: matches}), true
	case "grep":
		_ = filePath
		return "", false
	default:
		return "", false
	}
}

func (s *WorkspaceSearchService) enabled() bool {
	if s == nil || s.cfg == nil {
		return false
	}
	if s.cfgMu != nil {
		s.cfgMu.RLock()
		defer s.cfgMu.RUnlock()
	}
	return s.cfg.WorkspaceSearch.Enabled
}

func (s *WorkspaceSearchService) pollInterval() time.Duration {
	seconds := s.cfg.WorkspaceSearch.PollIntervalSeconds
	if seconds <= 0 {
		seconds = 5
	}
	return time.Duration(seconds) * time.Second
}

func (s *WorkspaceSearchService) maxFileSizeBytes() int64 {
	sizeMB := s.cfg.WorkspaceSearch.MaxFileSizeMB
	if sizeMB <= 0 {
		sizeMB = workspaceSearchDefaultFileSizeMB
	}
	return int64(sizeMB) * 1024 * 1024
}

func (s *WorkspaceSearchService) maxIndexSizeBytes() int64 {
	sizeMB := s.cfg.WorkspaceSearch.MaxIndexSizeMB
	if sizeMB <= 0 {
		sizeMB = workspaceSearchDefaultIndexSizeMB
	}
	return int64(sizeMB) * 1024 * 1024
}

func (s *WorkspaceSearchService) limit(requested int) int {
	if requested > 0 {
		if requested > 1000 {
			return 1000
		}
		return requested
	}
	configured := s.cfg.WorkspaceSearch.MaxResults
	if configured <= 0 {
		configured = workspaceSearchDefaultMaxResults
	}
	return configured
}

func (s *WorkspaceSearchService) fuzzyThreshold() float64 {
	threshold := s.cfg.WorkspaceSearch.FuzzyThreshold
	if threshold <= 0 {
		return 0.35
	}
	return threshold
}

func (s *WorkspaceSearchService) snapshot() workspaceSearchIndex {
	s.mu.RLock()
	defer s.mu.RUnlock()
	files := make(map[string]*workspaceIndexedFile, len(s.index.files))
	for path, file := range s.index.files {
		clone := *file
		clone.lines = append([]string(nil), file.lines...)
		files[path] = &clone
	}
	return workspaceSearchIndex{
		files:               files,
		orderedPaths:        append([]string(nil), s.index.orderedPaths...),
		contentFiles:        s.index.contentFiles,
		indexedContentBytes: s.index.indexedContentBytes,
	}
}

func (s *WorkspaceSearchService) scanWorkspace(ctx context.Context) (workspaceSearchIndex, error) {
	rootEval, err := filepath.EvalSymlinks(s.root)
	if err != nil {
		rootEval = s.root
	}
	rootEval, _ = filepath.Abs(rootEval)
	maxFileSize := s.maxFileSizeBytes()
	maxIndexSize := s.maxIndexSizeBytes()
	excludes := normalizedWorkspaceSearchExcludes(s.cfg.WorkspaceSearch.Exclude)
	next := workspaceSearchIndex{
		files: map[string]*workspaceIndexedFile{},
	}

	err = filepath.WalkDir(s.root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == s.root {
			return nil
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if workspacePathExcluded(rel, entry.IsDir(), excludes) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		eval, err := filepath.EvalSymlinks(path)
		if err == nil {
			evalAbs, _ := filepath.Abs(eval)
			if !pathWithinRoot(evalAbs, rootEval) {
				return nil
			}
		}
		if looksLikeBinaryWorkspaceFile(path) {
			return nil
		}

		file := &workspaceIndexedFile{
			path:    rel,
			absPath: path,
			size:    info.Size(),
			modTime: info.ModTime().UTC(),
		}
		if info.Size() <= maxFileSize && next.indexedContentBytes < maxIndexSize {
			data, err := os.ReadFile(path)
			if err == nil && !workspaceDataLooksBinary(data) && utf8.Valid(data) {
				if int64(len(data))+next.indexedContentBytes <= maxIndexSize {
					file.lines = splitWorkspaceLines(string(data))
					file.hash = workspaceContentHash(data)
					file.indexedContent = true
					next.contentFiles++
					next.indexedContentBytes += int64(len(data))
				}
			}
		}
		next.files[rel] = file
		next.orderedPaths = append(next.orderedPaths, rel)
		return nil
	})
	if err != nil {
		return workspaceSearchIndex{}, err
	}
	sort.Strings(next.orderedPaths)
	return next, nil
}

func splitWorkspaceLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func looksLikeBinaryWorkspaceFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, workspaceSearchBinarySampleBytes)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	return workspaceDataLooksBinary(buf[:n])
}

func workspaceDataLooksBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	if strings.IndexByte(string(data), 0) >= 0 {
		return true
	}
	if !utf8.Valid(data) {
		return true
	}
	control := 0
	for _, b := range data {
		if b < 0x20 && b != '\n' && b != '\r' && b != '\t' && b != '\f' {
			control++
		}
	}
	return control > len(data)/8
}

func workspaceContentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}

func workspaceFileResult(file *workspaceIndexedFile, score float64, accessCount int) WorkspaceSearchFileResult {
	return WorkspaceSearchFileResult{
		Path:           file.path,
		Size:           file.size,
		ModifiedAt:     file.modTime.Format(time.RFC3339),
		Score:          math.Round(score*1000) / 1000,
		AccessCount:    accessCount,
		IndexedContent: file.indexedContent,
	}
}

func (s *WorkspaceSearchService) loadAccessStats() (map[string]workspaceAccessStat, error) {
	rows, err := s.db.Query(`SELECT path, access_count, last_access, COALESCE(last_write, '') FROM workspace_file_access`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stats := map[string]workspaceAccessStat{}
	for rows.Next() {
		var rel, lastAccessRaw, lastWriteRaw string
		var count int
		if err := rows.Scan(&rel, &count, &lastAccessRaw, &lastWriteRaw); err != nil {
			return nil, err
		}
		stat := workspaceAccessStat{Count: count}
		stat.LastAccess, _ = time.Parse(time.RFC3339Nano, lastAccessRaw)
		stat.LastWrite, _ = time.Parse(time.RFC3339Nano, lastWriteRaw)
		stats[filepath.ToSlash(rel)] = stat
	}
	return stats, rows.Err()
}

func workspaceFrecencyBoost(stat workspaceAccessStat) float64 {
	if stat.Count <= 0 {
		return 0
	}
	countBoost := math.Min(0.75, math.Log1p(float64(stat.Count))*0.25)
	recencyBoost := 0.0
	if !stat.LastAccess.IsZero() {
		ageHours := time.Since(stat.LastAccess).Hours()
		if ageHours < 0 {
			ageHours = 0
		}
		recencyBoost = 0.25 / (1 + ageHours/24)
	}
	return countBoost + recencyBoost
}

func (s *WorkspaceSearchService) toRelativePath(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	candidate := raw
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(s.root, filepath.FromSlash(candidate))
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", false
	}
	rootEval, err := filepath.EvalSymlinks(s.root)
	if err != nil {
		rootEval = s.root
	}
	targetEval, err := filepath.EvalSymlinks(abs)
	if err != nil {
		targetEval = abs
	}
	rootEval, _ = filepath.Abs(rootEval)
	targetEval, _ = filepath.Abs(targetEval)
	if !pathWithinRoot(targetEval, rootEval) {
		return "", false
	}
	rel, err := filepath.Rel(s.root, abs)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func pathWithinRoot(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if strings.EqualFold(path, root) {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizedWorkspaceSearchExcludes(excludes []string) []string {
	if len(excludes) == 0 {
		excludes = config.DefaultWorkspaceSearchExcludes()
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(excludes))
	for _, exclude := range excludes {
		exclude = strings.TrimSpace(filepath.ToSlash(exclude))
		exclude = strings.Trim(exclude, "/")
		if exclude == "" || seen[exclude] {
			continue
		}
		seen[exclude] = true
		result = append(result, exclude)
	}
	return result
}

func workspacePathExcluded(rel string, isDir bool, excludes []string) bool {
	rel = strings.Trim(filepath.ToSlash(rel), "/")
	base := slashpath.Base(rel)
	for _, exclude := range excludes {
		if exclude == "" {
			continue
		}
		if strings.ContainsAny(exclude, "*?[") {
			if workspaceMatchGlob(exclude, rel) || workspaceMatchGlob(exclude, base) {
				return true
			}
			continue
		}
		if rel == exclude || base == exclude || strings.HasPrefix(rel, exclude+"/") || strings.Contains(rel, "/"+exclude+"/") {
			return true
		}
		if isDir && (base == exclude || strings.HasSuffix(rel, "/"+exclude)) {
			return true
		}
	}
	return false
}

func workspaceMatchesAnyGlob(glob, rel string) bool {
	patterns := normalizeWorkspaceSearchGlobs(glob)
	for _, pattern := range patterns {
		if workspaceMatchGlob(pattern, rel) {
			return true
		}
	}
	return false
}

func normalizeWorkspaceSearchGlobs(glob string) []string {
	if strings.TrimSpace(glob) == "" {
		return []string{"**/*"}
	}
	parts := strings.FieldsFunc(glob, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	patterns := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(filepath.ToSlash(part))
		if part != "" {
			patterns = append(patterns, part)
		}
	}
	if len(patterns) == 0 {
		return []string{"**/*"}
	}
	return patterns
}

func workspaceMatchGlob(pattern, rel string) bool {
	pattern = strings.Trim(filepath.ToSlash(strings.TrimSpace(pattern)), "/")
	rel = strings.Trim(filepath.ToSlash(rel), "/")
	if pattern == "" {
		return false
	}
	if pattern == "**/*" || pattern == "**" {
		return true
	}
	if !strings.Contains(pattern, "/") && !strings.Contains(pattern, "**") {
		matched, err := slashpath.Match(pattern, slashpath.Base(rel))
		return err == nil && matched
	}
	return workspaceMatchGlobSegments(splitWorkspaceSearchPattern(pattern), splitWorkspaceSearchPattern(rel))
}

func splitWorkspaceSearchPattern(value string) []string {
	if value == "" {
		return nil
	}
	return strings.Split(strings.Trim(value, "/"), "/")
}

func workspaceMatchGlobSegments(patternParts, pathParts []string) bool {
	if len(patternParts) == 0 {
		return len(pathParts) == 0
	}
	if patternParts[0] == "**" {
		if workspaceMatchGlobSegments(patternParts[1:], pathParts) {
			return true
		}
		return len(pathParts) > 0 && workspaceMatchGlobSegments(patternParts, pathParts[1:])
	}
	if len(pathParts) == 0 {
		return false
	}
	matched, err := slashpath.Match(patternParts[0], pathParts[0])
	if err != nil || !matched {
		return false
	}
	return workspaceMatchGlobSegments(patternParts[1:], pathParts[1:])
}

func workspacePathScore(query, rel string) float64 {
	q := strings.ToLower(strings.TrimSpace(query))
	p := strings.ToLower(rel)
	base := strings.ToLower(slashpath.Base(rel))
	stem := strings.TrimSuffix(base, slashpath.Ext(base))
	if q == "" {
		return 1
	}
	switch {
	case p == q || base == q || stem == q:
		return 1
	case strings.HasPrefix(stem, q):
		return 0.95
	case strings.Contains(stem, q):
		return 0.85
	case strings.Contains(base, q):
		return 0.8
	case strings.Contains(p, q):
		return 0.72
	default:
		return workspaceSubsequenceScore(q, p)
	}
}

func workspaceSubsequenceScore(query, path string) float64 {
	if query == "" || path == "" {
		return 0
	}
	qi := 0
	consecutive := 0
	bestConsecutive := 0
	for _, r := range path {
		if qi >= len(query) {
			break
		}
		if byte(r) == query[qi] {
			qi++
			consecutive++
			if consecutive > bestConsecutive {
				bestConsecutive = consecutive
			}
		} else {
			consecutive = 0
		}
	}
	if qi != len(query) {
		return 0
	}
	coverage := float64(len(query)) / float64(len(path))
	streak := float64(bestConsecutive) / float64(len(query))
	return 0.35 + coverage*0.35 + streak*0.2
}

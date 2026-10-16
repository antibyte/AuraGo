package codegraph

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Project is an internal capability. HTTP/model adapters resolve ID and Root from existing registrations.
type Project struct{ ID, Root string }

type Status struct {
	Project      string            `json:"project"`
	Generation   uint64            `json:"generation"`
	UpdatedAt    time.Time         `json:"updated_at"`
	Files        int               `json:"files"`
	Indexed      int               `json:"indexed"`
	Skipped      map[string]string `json:"skipped"`
	SkippedCount int               `json:"skipped_count"`
	Bytes        int64             `json:"bytes"`
	DurationMS   int64             `json:"duration_ms"`
	Complete     bool              `json:"complete"`
	Unresolved   int               `json:"unresolved"`
	Stale        bool              `json:"stale"`
	Warning      string            `json:"warning"`
}

type Request struct {
	Operation string `json:"operation"`
	Query     string `json:"query"`
	File      string `json:"file"`
	Symbol    string `json:"symbol"`
	Limit     int    `json:"limit"`
	Depth     int    `json:"depth"`
}
type Result struct {
	Status    Status   `json:"status"`
	Symbols   []Symbol `json:"symbols"`
	Edges     []Edge   `json:"edges"`
	Truncated bool     `json:"truncated"`
}
type snapshot struct {
	project Project
	files   map[string]*fileIndex
	symbols []Symbol
	edges   []Edge
	status  Status
}

type Manager struct {
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	worker    chan struct{}
	mu        sync.RWMutex
	projects  map[string]*snapshot
	enabled   func() bool
	authorize func(Project) bool
	onChange  func(context.Context, Project, string, string)
}

func (s *Status) skip(file, reason string) {
	s.SkippedCount++
	if len(s.Skipped) < 100 {
		s.Skipped[file] = reason
	}
}

func indexBytes(f *fileIndex) int64 {
	return int64(len(f.source)*2 + len(f.symbols)*1024 + len(f.edges)*512 + len(f.imports)*512 + len(f.bindings)*256)
}

// Reserve the published snapshot while building its replacement. Unchanged
// source/facts are shared; only their new resolved arrays need extra space.
func replacementBytes(f *fileIndex, reused bool) int64 {
	if reused {
		return int64(len(f.symbols)*256 + len(f.edges)*256)
	}
	return indexBytes(f)
}

func New(ctx context.Context, enabled func() bool, authorize func(Project) bool, onChange func(context.Context, Project, string, string)) *Manager {
	ctx, cancel := context.WithCancel(ctx)
	m := &Manager{ctx: ctx, cancel: cancel, worker: make(chan struct{}, 1), projects: map[string]*snapshot{}, enabled: enabled, authorize: authorize, onChange: onChange}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !m.allowed() {
					m.mu.Lock()
					m.projects = map[string]*snapshot{}
					m.mu.Unlock()
					continue
				}
				m.mu.RLock()
				var projects []Project
				for _, s := range m.projects {
					projects = append(projects, s.project)
				}
				m.mu.RUnlock()
				for _, p := range projects {
					_ = m.Rescan(ctx, p)
				}
			}
		}
	}()
	return m
}

func (m *Manager) allowed() bool {
	return m != nil && m.ctx.Err() == nil && (m.enabled == nil || m.enabled())
}

func (m *Manager) authorized(p Project) bool {
	return m.allowed() && (m.authorize == nil || m.authorize(p))
}

func (m *Manager) Forget(id string) {
	m.worker <- struct{}{}
	defer func() { <-m.worker }()
	m.mu.Lock()
	delete(m.projects, id)
	m.mu.Unlock()
}
func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.cancel()
	m.wg.Wait()
	m.worker <- struct{}{}
	m.mu.Lock()
	m.projects = map[string]*snapshot{}
	m.mu.Unlock()
	<-m.worker
}

func (m *Manager) Rescan(ctx context.Context, p Project) error {
	if !m.authorized(p) {
		m.Forget(p.ID)
		return fmt.Errorf("code graph disabled")
	}
	if p.ID == "" || !filepath.IsAbs(p.Root) {
		return fmt.Errorf("unregistered project")
	}
	select {
	case m.worker <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	case <-m.ctx.Done():
		return m.ctx.Err()
	}
	defer func() { <-m.worker }()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(m.ctx, cancel)
	defer stop()
	m.mu.RLock()
	old := m.projects[p.ID]
	otherBytes := int64(0)
	for id, s := range m.projects {
		if id != p.ID {
			otherBytes += s.status.Bytes
		}
	}
	count := len(m.projects)
	m.mu.RUnlock()
	if old == nil && count >= 16 {
		return fmt.Errorf("project_limit")
	}
	if old != nil && old.project.Root != p.Root {
		return fmt.Errorf("project root changed; restart index")
	}
	root, err := os.OpenRoot(p.Root)
	if err != nil {
		return fmt.Errorf("project unavailable")
	}
	defer root.Close()
	started := time.Now()
	next := &snapshot{project: p, files: map[string]*fileIndex{}, status: Status{Project: p.ID, Skipped: map[string]string{}, Warning: "Static analysis is incomplete for dynamic or external code; missing edges never prove safety."}}
	if old != nil {
		next.status.Generation = old.status.Generation
	}
	next.status.Generation++
	retainedBytes := otherBytes
	if old != nil {
		retainedBytes += old.status.Bytes
	}
	newBytes := int64(0)
	module := ""
	if b, e := readFile(root, "go.mod", 65536); e == nil {
		for _, line := range strings.Split(string(b), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[0] == "module" {
				module = fields[1]
				break
			}
		}
	}
	entries := 0
	err = fs.WalkDir(root.FS(), ".", func(name string, d fs.DirEntry, walkErr error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !m.authorized(p) {
			return context.Canceled
		}
		entries++
		if entries > 50000 {
			next.status.skip(".", "entry_limit")
			return fs.SkipAll
		}
		if walkErr != nil {
			next.status.skip(name, "read_error")
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "venv", ".venv", "__pycache__", "dist", "build", "reports", "disposable":
				return fs.SkipDir
			}
			return nil
		}
		if !supported(name) {
			return nil
		}
		next.status.Files++
		if d.Type()&os.ModeSymlink != 0 {
			next.status.skip(name, "symlink")
			return nil
		}
		b, e := readFile(root, name, MaxFileBytes)
		if e != nil {
			next.status.skip(name, "read_or_size_limit")
			return nil
		}
		h := sha256.Sum256(b)
		hash := hex.EncodeToString(h[:])
		var parsed *fileIndex
		reused := old != nil && old.files[name] != nil && old.files[name].hash == hash
		if reused {
			parsed = old.files[name]
		} else {
			parsed, e = parseFile(ctx, name, string(b), hash)
		}
		if e != nil {
			next.status.skip(name, e.Error())
			return nil
		}
		// Account retained source and graph structures conservatively, including strings.
		size := indexBytes(parsed)
		growth := replacementBytes(parsed, reused)
		if next.status.Bytes+otherBytes+size > MaxIndexBytes || retainedBytes+newBytes+growth > MaxIndexBytes {
			next.status.skip(name, "index_budget")
			return nil
		}
		newBytes += growth
		next.status.Bytes += size
		next.files[name] = parsed
		next.status.Indexed++
		return nil
	})
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	next.symbols, next.edges = resolve(ctx, next.files, module)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	sort.Slice(next.edges, func(i, j int) bool {
		a, b := next.edges[i], next.edges[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.offset != b.offset {
			return a.offset < b.offset
		}
		return a.Name < b.Name
	})
	sort.Slice(next.symbols, func(i, j int) bool {
		if next.symbols[i].File != next.symbols[j].File {
			return next.symbols[i].File < next.symbols[j].File
		}
		return next.symbols[i].Start < next.symbols[j].Start
	})
	for _, e := range next.edges {
		if e.To == "" {
			next.status.Unresolved++
		}
	}
	next.status.Complete = next.status.SkippedCount == 0
	next.status.UpdatedAt = time.Now().UTC()
	next.status.DurationMS = time.Since(started).Milliseconds()
	if !m.authorized(p) {
		return context.Canceled
	}
	m.mu.Lock()
	m.projects[p.ID] = next
	m.mu.Unlock()
	if m.onChange != nil {
		for name, f := range next.files {
			if old == nil || old.files[name] == nil || old.files[name].hash != f.hash {
				m.onChange(ctx, p, name, f.hash)
			}
		}
		if old != nil {
			for name := range old.files {
				if next.files[name] == nil && next.status.Skipped[name] == "" {
					if _, err := root.Lstat(name); os.IsNotExist(err) {
						m.onChange(ctx, p, name, "missing")
					}
				}
			}
		}
	}
	return nil
}

func readFile(root *os.Root, name string, limit int64) ([]byte, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("not a bounded regular file")
	}
	f, err := openSource(root, name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	actual, err := f.Stat()
	if err != nil || !actual.Mode().IsRegular() || !os.SameFile(info, actual) {
		return nil, fmt.Errorf("file changed")
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(b)) > limit {
		err = fmt.Errorf("file_limit")
	}
	return b, err
}

func Fingerprint(p Project, resource string) (string, error) {
	if !fs.ValidPath(resource) || path.Clean(resource) != resource || strings.ContainsAny(resource, "\\:") {
		return "", fmt.Errorf("invalid project resource")
	}
	root, err := os.OpenRoot(p.Root)
	if err != nil {
		return "", err
	}
	defer root.Close()
	parts := strings.Split(resource, "/")
	for i := range parts {
		info, err := root.Lstat(strings.Join(parts[:i+1], "/"))
		if os.IsNotExist(err) {
			return "missing", nil
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("linked resource")
		}
	}
	b, err := readFile(root, resource, MaxFileBytes)
	if os.IsNotExist(err) {
		return "missing", nil
	}
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func (m *Manager) Query(ctx context.Context, p Project, q Request) (result Result, resultErr error) {
	if !m.authorized(p) {
		m.Forget(p.ID)
		return Result{}, fmt.Errorf("code graph disabled")
	}
	defer func() {
		if !m.authorized(p) || ctx.Err() != nil {
			result, resultErr = Result{}, errors.New("code graph authorization revoked or request cancelled")
		}
	}()
	switch q.Operation {
	case "status", "search", "outline", "callers", "callees", "impact", "rescan":
	default:
		return Result{}, fmt.Errorf("invalid code graph operation")
	}
	m.mu.RLock()
	s := m.projects[p.ID]
	m.mu.RUnlock()
	if s == nil || q.Operation == "rescan" {
		if err := m.Rescan(ctx, p); err != nil {
			return Result{}, err
		}
		m.mu.RLock()
		s = m.projects[p.ID]
		m.mu.RUnlock()
	}
	if s == nil || s.project != p {
		return Result{}, fmt.Errorf("project unavailable")
	}
	r := Result{Status: s.status, Symbols: []Symbol{}, Edges: []Edge{}}
	r.Status.Skipped = maps.Clone(s.status.Skipped)
	r.Status.Stale = time.Since(r.Status.UpdatedAt) > 10*time.Second
	limit := q.Limit
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	depth := q.Depth
	if depth < 1 {
		depth = 1
	}
	if depth > 4 {
		depth = 4
	}
	addSymbol := func(s Symbol) {
		if len(r.Symbols) < limit {
			r.Symbols = append(r.Symbols, s)
		} else {
			r.Truncated = true
		}
	}
	if q.Operation == "status" || q.Operation == "rescan" {
		return r, nil
	}
	if q.Operation == "search" || q.Operation == "outline" {
		for _, symbol := range s.symbols {
			if q.Operation == "outline" {
				if symbol.File == q.File {
					addSymbol(symbol)
				}
			} else if strings.Contains(strings.ToLower(symbol.Name), strings.ToLower(q.Query)) {
				addSymbol(symbol)
			}
		}
		return r, nil
	}
	var matches []Symbol
	for _, symbol := range s.symbols {
		if (symbol.ID == q.Symbol || symbol.Name == q.Symbol) && (q.File == "" || symbol.File == q.File) {
			matches = append(matches, symbol)
		}
	}
	if len(matches) != 1 {
		return r, fmt.Errorf("select one exact symbol ID from search or outline")
	}
	frontier := map[string]bool{matches[0].ID: true}
	seen := map[string]bool{}
	addSymbol(matches[0])
	for level := 0; level < depth; level++ {
		next := map[string]bool{}
		for _, e := range s.edges {
			target := ""
			if q.Operation == "callees" && frontier[e.From] {
				target = e.To
			} else if q.Operation != "callees" && frontier[e.To] {
				target = e.From
			} else {
				continue
			}
			key := fmt.Sprintf("%s:%d:%s:%s", e.File, e.Line, e.From, e.Name)
			if seen[key] {
				continue
			}
			seen[key] = true
			if len(r.Edges) < limit {
				r.Edges = append(r.Edges, e)
			} else {
				r.Truncated = true
			}
			if target != "" {
				next[target] = true
			}
		}
		if q.Operation != "impact" {
			break
		}
		frontier = next
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if !m.allowed() {
		return Result{}, errors.New("code graph authorization revoked")
	}
	return r, nil
}

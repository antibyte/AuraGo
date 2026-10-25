package selftest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"aurago/internal/fileutil"
)

var ErrBusy = errors.New("self-test already running")
var runID = regexp.MustCompile(`^[a-f0-9]{32}$`)

type Executor func(context.Context, string, Case, int, func(Evidence)) Execution

type Manager struct {
	mu       sync.Mutex
	root     *os.Root
	catalog  []Template
	runs     map[string]*Run
	cancel   context.CancelFunc
	active   string
	closed   bool
	wg       sync.WaitGroup
	execute  Executor
	bindings map[string][]Binding
}

type bindingKey struct{}

func BindingFromContext(ctx context.Context) (Binding, bool) {
	b, ok := ctx.Value(bindingKey{}).(Binding)
	return b, ok
}

func NewManager(dir string, execute Executor) (*Manager, error) {
	catalog, err := Catalog()
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	m := &Manager{root: root, catalog: catalog, runs: map[string]*Run{}, execute: execute, bindings: map[string][]Binding{}}
	if err = m.restore(); err != nil {
		root.Close()
		return nil, err
	}
	return m, nil
}

func (m *Manager) restore() error {
	entries, err := fs.ReadDir(m.root.FS(), ".")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") && runID.MatchString(strings.TrimSuffix(entry.Name(), ".tmp")) && entry.Type()&os.ModeSymlink == 0 {
			if err := m.root.Remove(entry.Name()); err != nil {
				return err
			}
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".json")
		if !runID.MatchString(id) || entry.Name() != id+".json" || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 2<<20 {
			continue
		}
		data, err := m.root.ReadFile(entry.Name())
		if err != nil {
			return err
		}
		var run Run
		if json.Unmarshal(data, &run) != nil || run.ID != id {
			continue
		}
		if run.Status == "running" {
			run.Status = "interrupted"
			now := time.Now().UTC()
			run.Finished = &now
			for i := range run.Cases {
				if run.Cases[i].Verdict == "RUNNING" {
					run.Cases[i].Verdict, run.Cases[i].Code = "UNKNOWN", "process_interrupted"
					if run.Cases[i].Cleanup == "pending" {
						run.Cases[i].Cleanup = "unknown"
					}
				}
				if run.Cases[i].Verdict == "PENDING" {
					run.Cases[i].Verdict, run.Cases[i].Code = "CANCELLED", "process_interrupted_before_execution"
				}
			}
		}
		Summarize(&run)
		m.runs[id] = &run
	}
	return m.prune()
}

func (m *Manager) prune() error {
	var list []*Run
	for _, run := range m.runs {
		if run.ID != m.active {
			list = append(list, run)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Started.After(list[j].Started) })
	var size int64
	for i, run := range list {
		data, _ := json.Marshal(run)
		size += int64(len(data))
		retain := i < 100 && time.Since(run.Started) < 14*24*time.Hour && size < 100<<20
		for _, c := range run.Cases {
			if c.Cleanup == "failed" || c.Cleanup == "unknown" || c.Verdict == "UNKNOWN" {
				retain = true
			}
		}
		if retain {
			continue
		}
		if err := m.root.Remove(run.ID + ".json"); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		delete(m.runs, run.ID)
	}
	return nil
}

func (m *Manager) persist(run *Run) error {
	Summarize(run)
	data, err := json.Marshal(run)
	if err != nil {
		return err
	}
	if len(data) > 2<<20 {
		return fmt.Errorf("self-test record too large")
	}
	tmp := run.ID + ".tmp"
	f, err := m.root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer m.root.Remove(tmp)
	defer f.Close()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return fileutil.RenameRootContext(context.Background(), m.root, tmp, run.ID+".json")
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func matches(target string, cap Capability) bool {
	if target == cap.Name {
		return true
	}
	return target == map[string]string{"mcp": "MCP:*", "custom": "Custom:*", "skill": "Skill:*", "agent_skill": "AgentSkill:*"}[cap.Kind] || target == "Composio:*" && strings.HasPrefix(cap.Name, "composio:")
}

func (m *Manager) cases(inventory []Capability, req Request) []Case {
	var result []Case
	mapped := map[string]bool{}
	for _, t := range m.catalog {
		for _, target := range t.Targets {
			var caps []Capability
			for _, cap := range inventory {
				if matches(target, cap) {
					caps = append(caps, cap)
				}
			}
			if len(caps) == 0 {
				caps = []Capability{{Name: target, Instance: "default", Status: "disabled"}}
			}
			for _, cap := range caps {
				key := cap.Name + "@" + cap.Instance
				mapped[key] = true
				c := Case{ID: t.ID + ":" + key, TemplateID: t.ID, Tool: cap.Name, Instance: cap.Instance, Profile: t.Profile, Prompt: t.Prompt, Verdict: "PENDING", Cleanup: "not_needed"}
				switch {
				case len(req.Targets) > 0 && !contains(req.Targets, cap.Name), len(req.CaseIDs) > 0 && !contains(req.CaseIDs, t.ID):
					c.Verdict = "SKIPPED_PROFILE"
				case !cap.Enabled:
					c.Verdict = "SKIPPED_DISABLED"
				case !cap.Callable:
					c.Verdict, c.Code = "BLOCKED_SETUP", cap.Status
					if cap.Status == "admin_only" {
						c.Verdict = "BLOCKED_ACCESS"
					}
					if cap.Status == "denied" {
						c.Verdict = "BLOCKED_PERMISSION"
					}
				case req.Profile == "targeted" && hasBinding(req, t.ID, cap.Name):
				case req.Profile == "targeted":
					c.Verdict = "SKIPPED_PROFILE"
				case t.Profile == "L" || t.Profile == "H":
					c.Verdict, c.Code = "BLOCKED_FIXTURE", "isolated_or_interactive_fixture_required"
				case t.Profile == "X":
					c.Verdict, c.Code = "BLOCKED_PERMISSION", "explicit_effect_target_required"
				case t.Profile == "W" && req.Profile != "fixtures":
					c.Verdict = "SKIPPED_PROFILE"
				}
				result = append(result, c)
			}
		}
	}
	for _, cap := range inventory {
		if cap.Enabled && !mapped[cap.Name+"@"+cap.Instance] {
			if cap.Kind == "integration" && cap.Callable {
				c := Case{ID: "OP-01:" + cap.Name + "@" + cap.Instance, TemplateID: "OP-01", Tool: cap.Name, Instance: cap.Instance, Profile: "R", Verdict: "PENDING", Cleanup: "not_needed"}
				for _, t := range m.catalog {
					if t.ID == "OP-01" {
						c.Prompt = t.Prompt
						break
					}
				}
				if req.Profile == "targeted" || len(req.Targets) > 0 && !contains(req.Targets, cap.Name) || len(req.CaseIDs) > 0 && !contains(req.CaseIDs, "OP-01") {
					c.Verdict = "SKIPPED_PROFILE"
				}
				result = append(result, c)
				continue
			}
			result = append(result, Case{ID: "OP-04:" + cap.Name + "@" + cap.Instance, TemplateID: "OP-04", Tool: cap.Name, Instance: cap.Instance, Profile: "R", Verdict: "NOT_COVERED", Code: "unreviewed_capability", Cleanup: "not_needed"})
		}
	}
	return result
}

func (m *Manager) Start(parent context.Context, req Request, inventory []Capability) (Run, error) {
	if req.Profile == "" {
		req.Profile = "read"
	}
	if req.Profile != "read" && req.Profile != "fixtures" && req.Profile != "targeted" {
		return Run{}, fmt.Errorf("invalid self-test profile")
	}
	if req.TimeoutSeconds == 0 {
		req.TimeoutSeconds = 600
	}
	if req.TokenLimit == 0 {
		req.TokenLimit = 32768
	}
	if req.TimeoutSeconds < 5 || req.TimeoutSeconds > 2700 || req.TokenLimit < 512 || req.TokenLimit > 262144 || len(req.Targets) > 100 || len(req.CaseIDs) > 320 || len(inventory) > 2000 {
		return Run{}, fmt.Errorf("invalid self-test limits")
	}
	for _, id := range req.CaseIDs {
		if !caseID.MatchString(id) {
			return Run{}, fmt.Errorf("invalid case id")
		}
	}
	if err := m.validateBindings(&req, inventory); err != nil {
		return Run{}, err
	}
	if parent == nil || parent.Err() != nil || m.execute == nil {
		return Run{}, fmt.Errorf("self-test runtime unavailable")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return Run{}, fmt.Errorf("self-test service closed")
	}
	if m.active != "" {
		return Run{}, ErrBusy
	}
	if err := m.prune(); err != nil {
		return Run{}, err
	}
	var storedBytes int
	for _, record := range m.runs {
		b, _ := json.Marshal(record)
		storedBytes += len(b)
	}
	if storedBytes+(2<<20) > (100 << 20) {
		return Run{}, fmt.Errorf("unresolved self-test records exhausted storage budget")
	}
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return Run{}, err
	}
	run := &Run{ID: hex.EncodeToString(idBytes), Revision: 1, Status: "running", Started: time.Now().UTC(), Request: req, Inventory: inventory}
	run.Cases = m.cases(inventory, req)
	bindings := req.Bindings
	run.Request.Bindings = nil
	if len(run.Cases) > 5000 {
		return Run{}, fmt.Errorf("too many self-test cases")
	}
	if err := m.persist(run); err != nil {
		return Run{}, err
	}
	m.runs[run.ID] = run
	m.bindings[run.ID] = bindings
	ctx, cancel := context.WithTimeout(parent, time.Duration(req.TimeoutSeconds)*time.Second)
	m.active, m.cancel = run.ID, cancel
	m.wg.Add(1)
	go m.work(ctx, run.ID)
	return clone(run), nil
}

func clone(run *Run) Run {
	data, _ := json.Marshal(run)
	var result Run
	_ = json.Unmarshal(data, &result)
	return result
}

func (m *Manager) work(ctx context.Context, id string) {
	defer m.wg.Done()
	defer func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		run := m.runs[id]
		if recover() != nil {
			run.Status = "interrupted"
			for i := range run.Cases {
				if run.Cases[i].Verdict == "RUNNING" {
					run.Cases[i].Verdict, run.Cases[i].Code = "UNKNOWN", "runtime_failure"
				}
			}
		}
		for i := range run.Cases {
			if run.Cases[i].Verdict == "PENDING" {
				run.Cases[i].Verdict = "CANCELLED"
			}
		}
		if run.Status == "running" {
			run.Status = "complete"
			if ctx.Err() != nil {
				run.Status = "cancelled"
			}
		}
		now := time.Now().UTC()
		run.Finished = &now
		if err := m.persist(run); err != nil {
			run.Status = "storage_error"
		}
		delete(m.bindings, id)
		m.cancel()
		m.cancel = nil
		m.active = ""
	}()
	for i := 0; ; i++ {
		m.mu.Lock()
		run := m.runs[id]
		if i >= len(run.Cases) {
			m.mu.Unlock()
			return
		}
		c := run.Cases[i]
		if c.Verdict != "PENDING" {
			m.mu.Unlock()
			continue
		}
		remaining := run.Request.TokenLimit - run.Summary.Tokens
		if ctx.Err() != nil || remaining < 512 {
			run.Cases[i].Verdict = "BLOCKED_BUDGET"
			if errors.Is(ctx.Err(), context.Canceled) {
				run.Cases[i].Verdict = "CANCELLED"
			}
			if err := m.persist(run); err != nil {
				run.Status = "storage_error"
				m.mu.Unlock()
				return
			}
			m.mu.Unlock()
			continue
		}
		run.Cases[i].Verdict = "RUNNING"
		if c.Profile == "W" || strings.HasPrefix(c.TemplateID, "FILE-") {
			run.Cases[i].Cleanup = "pending"
		}
		if err := m.persist(run); err != nil {
			run.Status = "storage_error"
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()
		start := time.Now()
		caseCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		m.mu.Lock()
		for _, b := range m.bindings[id] {
			if b.CaseID == c.TemplateID && b.Tool == c.Tool {
				caseCtx = context.WithValue(caseCtx, bindingKey{}, b)
				break
			}
		}
		m.mu.Unlock()
		out := m.execute(caseCtx, id, c, remaining, func(e Evidence) {
			m.mu.Lock()
			defer m.mu.Unlock()
			if len(m.runs[id].Cases[i].Evidence) < 16 {
				m.runs[id].Cases[i].Evidence = append(m.runs[id].Cases[i].Evidence, e)
				if err := m.persist(m.runs[id]); err != nil {
					cancel()
				}
			}
		})
		cancel()
		m.mu.Lock()
		run = m.runs[id]
		actual := &run.Cases[i]
		actual.Verdict, actual.Code, actual.Operation = out.Verdict, out.Code, out.Operation
		actual.Tokens, actual.Assertions, actual.DurationMS = out.Tokens, out.Assertions, time.Since(start).Milliseconds()
		if out.Cleanup != "" {
			actual.Cleanup = out.Cleanup
		}
		if len(out.Evidence) > 0 {
			actual.Evidence = out.Evidence
		}
		hasEvidence := false
		for _, e := range actual.Evidence {
			if e.Tool == actual.Tool && e.Executed && e.Status == "success" {
				hasEvidence = true
			}
		}
		if actual.Verdict == "" || actual.Verdict == "PASS" && (!hasEvidence || len(actual.Assertions) == 0) {
			actual.Verdict, actual.Code = "INCONCLUSIVE", "missing_execution_evidence"
		}
		if err := m.persist(run); err != nil {
			run.Status = "storage_error"
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()
	}
}

func (m *Manager) Get(id string) (Run, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[id]
	if !ok {
		return Run{}, false
	}
	return clone(run), true
}
func (m *Manager) List() []Run {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := []Run{}
	for _, run := range m.runs {
		r := clone(run)
		r.Inventory = nil
		r.Cases = nil
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Started.After(result[j].Started) })
	return result
}
func (m *Manager) Cancel(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active != id || m.cancel == nil {
		return false
	}
	m.cancel()
	return true
}
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Unlock()
	m.wg.Wait()
	_ = m.root.Close()
}

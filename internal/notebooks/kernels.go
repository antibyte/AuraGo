package notebooks

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/kaggle"
)

type Runtime interface {
	EnsureImage(context.Context) error
	Create(context.Context, string, config.KaggleLocalConfig, bool) (string, error)
	Remove(context.Context, string) error
	Cleanup(context.Context) error
	CopyIn(context.Context, string, *kaggle.Service, string, []string) error
	Attach(context.Context, string) (io.ReadWriteCloser, error)
}
type Session struct {
	ID        string `json:"id"`
	Project   string `json:"project_id"`
	File      string `json:"file"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	Internet  bool   `json:"internet"`
	Execution string `json:"execution_id,omitempty"`
	Revision  string `json:"revision,omitempty"`
	Cell      string `json:"cell_id,omitempty"`
	container string
	stream    io.ReadWriteCloser
	cancel    context.CancelFunc
	files     []string
	last      time.Time
	deadline  time.Time
	writeMu   sync.Mutex
	stopMu    sync.Mutex
}
type Kernels struct {
	storage     *kaggle.Service
	documents   *Documents
	runtime     Runtime
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	sessions    map[string]*Session
	wg          sync.WaitGroup
	recovery    sync.Once
	recoveryErr error
}

func NewKernels(parent context.Context, storage *kaggle.Service, documents *Documents, runtime Runtime) *Kernels {
	ctx, cancel := context.WithCancel(parent)
	k := &Kernels{storage: storage, documents: documents, runtime: runtime, ctx: ctx, cancel: cancel, sessions: map[string]*Session{}}
	k.wg.Add(1)
	go k.maintain()
	return k
}
func (k *Kernels) Close() {
	k.cancel()
	k.mu.Lock()
	ids := []string{}
	for id := range k.sessions {
		ids = append(ids, id)
	}
	k.mu.Unlock()
	for _, id := range ids {
		k.Stop(id, "server_shutdown")
	}
	k.wg.Wait()
}
func kernelID() string { var b [16]byte; rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (k *Kernels) permission(internet bool) error {
	s := k.storage.Snapshot()
	c := s.Config
	if !c.Enabled || c.ReadOnly || !c.Local.Enabled || !c.Local.AllowExecute {
		return errors.New("local notebook execution is disabled or read-only")
	}
	if !s.Access.Write || !s.Access.Python || !s.Access.Docker {
		return errors.New("local notebooks require filesystem, Python and Docker permissions")
	}
	if internet && (!c.Local.AllowInternet || !s.Access.Network) {
		return errors.New("local kernel internet access is disabled")
	}
	return nil
}
func snapshotSession(s *Session) Session {
	return Session{ID: s.ID, Project: s.Project, File: s.File, Status: s.Status, Error: s.Error, Internet: s.Internet, Execution: s.Execution, Revision: s.Revision, Cell: s.Cell}
}
func (k *Kernels) List() []Session {
	k.mu.Lock()
	defer k.mu.Unlock()
	out := []Session{}
	for _, s := range k.sessions {
		out = append(out, snapshotSession(s))
	}
	return out
}
func (k *Kernels) Start(project, file string, files []string, internet bool) (Session, error) {
	if err := k.permission(internet); err != nil {
		return Session{}, err
	}
	if _, err := k.documents.Load(project, file); err != nil {
		return Session{}, err
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	active := 0
	for _, s := range k.sessions {
		if s.Status != "stopped" && s.Status != "error" {
			active++
		}
	}
	if active >= k.storage.Snapshot().Config.Local.MaxSessions {
		return Session{}, errors.New("local notebook session limit reached")
	}
	ctx, cancel := context.WithCancel(k.ctx)
	s := &Session{ID: kernelID(), Project: project, File: file, Status: "starting", Internet: internet, files: append([]string(nil), files...), cancel: cancel, last: time.Now()}
	k.sessions[s.ID] = s
	k.wg.Add(1)
	go k.start(ctx, s)
	return snapshotSession(s), nil
}
func (k *Kernels) start(ctx context.Context, s *Session) {
	defer k.wg.Done()
	failed := true
	defer func() {
		if failed {
			k.Stop(s.ID, "runtime_start_failed")
		}
	}()
	// Cleanup only once, before this manager creates its first container.
	k.recovery.Do(func() {
		k.recoveryErr = k.runtime.Cleanup(k.ctx)
		k.storage.Store.Emit(k.ctx, "kernel_state_lost", map[string]string{"reason": "backend_restart"})
	})
	if k.recoveryErr != nil {
		k.setError(s, k.recoveryErr)
		return
	}
	if err := k.runtime.EnsureImage(ctx); err != nil {
		k.setError(s, err)
		return
	}
	if err := k.permission(s.Internet); err != nil {
		k.setError(s, err)
		return
	}
	id, err := k.runtime.Create(ctx, s.ID, k.storage.Snapshot().Config.Local, s.Internet)
	if err != nil {
		k.setError(s, err)
		return
	}
	k.mu.Lock()
	s.container = id
	stopped := s.Status == "stopped" || ctx.Err() != nil
	k.mu.Unlock()
	if stopped {
		return
	}
	if err = k.runtime.CopyIn(ctx, id, k.storage, s.Project, s.files); err != nil {
		k.setError(s, err)
		return
	}
	stream, err := k.runtime.Attach(ctx, id)
	if err != nil {
		k.setError(s, err)
		return
	}
	k.mu.Lock()
	if s.Status == "stopped" || ctx.Err() != nil {
		k.mu.Unlock()
		stream.Close()
		return
	}
	s.stream = stream
	k.mu.Unlock()
	failed = false
	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 64<<10), (1<<20)+1)
	for scanner.Scan() {
		var event map[string]any
		if len(scanner.Bytes()) > 1<<20 || json.Unmarshal(scanner.Bytes(), &event) != nil {
			break
		}
		if event["protocol"] != float64(1) {
			break
		}
		kind, _ := event["kind"].(string)
		k.mu.Lock()
		if ctx.Err() != nil || s.Status == "stopping" || s.Status == "stopped" {
			k.mu.Unlock()
			break
		}
		if kind == "ready" {
			s.Status = "idle"
		} else {
			exec, _ := event["execution_id"].(string)
			if exec == "" || exec != s.Execution {
				k.mu.Unlock()
				continue
			}
			if kind == "done" || kind == "completion" || kind == "error" {
				s.Status = "idle"
				s.deadline = time.Time{}
			}
		}
		s.last = time.Now()
		event["session_id"] = s.ID
		event["project_id"] = s.Project
		event["file"] = s.File
		state := snapshotSession(s)
		k.mu.Unlock()
		_, _ = k.storage.Store.Emit(ctx, "cell", event)
		_, _ = k.storage.Store.Emit(ctx, "session", state)
	}
	k.Stop(s.ID, "kernel_disconnected")
}
func (k *Kernels) setError(s *Session, err error) {
	k.mu.Lock()
	s.Error = kaggle.PublicError(err)
	k.mu.Unlock()
}
func (k *Kernels) Stop(id, reason string) error {
	k.mu.Lock()
	s := k.sessions[id]
	k.mu.Unlock()
	if s == nil {
		return errors.New("unknown notebook session")
	}
	// A disconnect, revocation and explicit stop may arrive concurrently.
	// Retain the owned container and its slot until Docker confirms removal.
	s.stopMu.Lock()
	defer s.stopMu.Unlock()
	k.mu.Lock()
	if s.Status == "stopped" && s.container == "" && s.stream == nil {
		k.mu.Unlock()
		return nil
	}
	s.Status = "stopping"
	container, stream, cancel := s.container, s.stream, s.cancel
	s.stream = nil
	k.mu.Unlock()
	cancel()
	if stream != nil {
		stream.Close()
	}
	ctx, done := context.WithTimeout(context.WithoutCancel(k.ctx), 20*time.Second)
	defer done()
	var err error
	if container != "" {
		err = k.runtime.Remove(ctx, container)
	}
	k.mu.Lock()
	if err != nil {
		s.Status = "stopping"
		s.Error = kaggle.PublicError(err)
	} else {
		// Creation may have finished after a concurrent early stop. Its start
		// goroutine observes cancellation and removes that container itself.
		if s.container == container {
			s.container = ""
		}
		s.Status = "stopped"
		if reason != "runtime_start_failed" || s.Error == "" {
			s.Error = reason
		}
	}
	state := snapshotSession(s)
	k.mu.Unlock()
	_, _ = k.storage.Store.Emit(ctx, "session", state)
	return err
}
func (k *Kernels) Restart(id string) (Session, error) {
	k.mu.Lock()
	s := k.sessions[id]
	if s == nil {
		k.mu.Unlock()
		return Session{}, errors.New("unknown notebook session")
	}
	p, f, files, internet := s.Project, s.File, append([]string(nil), s.files...), s.Internet
	k.mu.Unlock()
	if err := k.Stop(id, "kernel_restarted"); err != nil {
		return Session{}, err
	}
	return k.Start(p, f, files, internet)
}
func (k *Kernels) Execute(id, expected, cell string) (string, error) {
	k.mu.Lock()
	s := k.sessions[id]
	if s == nil {
		k.mu.Unlock()
		return "", errors.New("unknown notebook session")
	}
	project, file, internet := s.Project, s.File, s.Internet
	k.mu.Unlock()
	if err := k.permission(internet); err != nil {
		return "", err
	}
	doc, err := k.documents.Load(project, file)
	if err != nil {
		return "", err
	}
	if doc.Revision != expected {
		return "", ErrConflict
	}
	code, err := CellSource(doc, cell)
	if err != nil {
		return "", err
	}
	return k.command(id, "execute", expected, cell, code, 0)
}
func (k *Kernels) Complete(id, code string, cursor int) (string, error) {
	if cursor < 0 || cursor > len(code) {
		return "", errors.New("invalid completion cursor")
	}
	return k.command(id, "complete", "", "", code, cursor)
}
func (k *Kernels) command(id, command, revision, cell, code string, cursor int) (string, error) {
	if len(code) > 512<<10 {
		return "", errors.New("cell exceeds 512 KiB")
	}
	k.mu.Lock()
	s := k.sessions[id]
	if s == nil || s.Status != "idle" || s.stream == nil {
		k.mu.Unlock()
		return "", errors.New("kernel is not ready or busy")
	}
	if err := k.permission(s.Internet); err != nil {
		k.mu.Unlock()
		return "", err
	}
	exec := kernelID()
	s.Execution = exec
	s.Revision = revision
	s.Cell = cell
	s.Status = "busy"
	s.last = time.Now()
	seconds := k.storage.Snapshot().Config.Local.CellTimeoutMinutes * 60
	s.deadline = s.last.Add(time.Duration(seconds+5) * time.Second)
	stream := s.stream
	k.mu.Unlock()
	body, _ := json.Marshal(map[string]any{"command": command, "execution_id": exec, "revision": revision, "cell_id": cell, "code": code, "cursor": cursor, "timeout_seconds": seconds})
	s.writeMu.Lock()
	_, err := stream.Write(append(body, '\n'))
	s.writeMu.Unlock()
	if err != nil {
		k.Stop(id, "bridge_write_failed")
		return "", errors.New("kernel communication failed")
	}
	return exec, nil
}
func (k *Kernels) Interrupt(id string) error {
	k.mu.Lock()
	s := k.sessions[id]
	if s == nil || s.stream == nil {
		k.mu.Unlock()
		return errors.New("kernel is unavailable")
	}
	stream, exec := s.stream, s.Execution
	k.mu.Unlock()
	body, _ := json.Marshal(map[string]string{"command": "interrupt", "execution_id": exec})
	s.writeMu.Lock()
	_, err := stream.Write(append(body, '\n'))
	s.writeMu.Unlock()
	return err
}
func (k *Kernels) maintain() {
	defer k.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-k.ctx.Done():
			return
		case now := <-ticker.C:
			k.mu.Lock()
			stop := map[string]string{}
			idle := time.Duration(k.storage.Snapshot().Config.Local.IdleMinutes) * time.Minute
			for id, s := range k.sessions {
				if s.Status == "stopping" {
					stop[id] = "container_cleanup_pending"
					continue
				}
				if s.Status == "stopped" {
					continue
				}
				if k.permission(s.Internet) != nil {
					stop[id] = "permission_revoked"
				} else if !s.deadline.IsZero() && now.After(s.deadline) {
					stop[id] = "cell_timeout"
				} else if s.Status == "idle" && now.Sub(s.last) > idle {
					stop[id] = "idle_timeout"
				}
			}
			k.mu.Unlock()
			for id, reason := range stop {
				k.Stop(id, reason)
			}
		}
	}
}

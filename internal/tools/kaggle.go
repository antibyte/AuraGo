package tools

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"sync"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/kaggle"
	"aurago/internal/notebooks"
)

type KaggleBackend struct {
	Platform  *kaggle.Service
	Documents *notebooks.Documents
	Kernels   *notebooks.Kernels
}

var kaggleBackends = struct {
	sync.RWMutex
	byDir map[string]*KaggleBackend
}{byDir: map[string]*KaggleBackend{}}

func RegisterKaggleBackend(dir string, b *KaggleBackend) func() {
	key, _ := filepath.Abs(dir)
	kaggleBackends.Lock()
	kaggleBackends.byDir[key] = b
	kaggleBackends.Unlock()
	return func() {
		kaggleBackends.Lock()
		if kaggleBackends.byDir[key] == b {
			delete(kaggleBackends.byDir, key)
		}
		kaggleBackends.Unlock()
	}
}

type KaggleRuntimeRequest struct {
	Operation string          `json:"operation"`
	ProjectID string          `json:"project_id,omitempty"`
	File      string          `json:"file,omitempty"`
	Title     string          `json:"title,omitempty"`
	Revision  string          `json:"revision,omitempty"`
	CellID    string          `json:"cell_id,omitempty"`
	SessionID string          `json:"session_id,omitempty"`
	JobID     string          `json:"job_id,omitempty"`
	Files     []string        `json:"files,omitempty"`
	Internet  bool            `json:"internet,omitempty"`
	Code      string          `json:"code,omitempty"`
	Cursor    int             `json:"cursor,omitempty"`
	After     int64           `json:"after,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
}

func (b *KaggleBackend) Runtime(ctx context.Context, r KaggleRuntimeRequest) (any, error) {
	switch r.Operation {
	case "status":
		return b.Platform.Status(), nil
	case "projects":
		return b.Platform.Store.Projects(ctx)
	case "create_project":
		return b.Platform.CreateProject(ctx, r.Title)
	case "files":
		return b.Platform.Files(r.ProjectID)
	case "read_file":
		f, err := b.Platform.OpenFile(r.ProjectID, r.File)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, (256<<10)+1))
		if err != nil {
			return nil, err
		}
		truncated := len(data) > 256<<10
		if truncated {
			data = data[:256<<10]
			for len(data) > 0 && !utf8.Valid(data) {
				data = data[:len(data)-1]
				if len(data) < (256<<10)-4 {
					break
				}
			}
		}
		if !utf8.Valid(data) {
			return nil, errors.New("binary file has no text preview; use the file download")
		}
		return map[string]any{"file": r.File, "text": string(data), "truncated": truncated}, nil
	case "load_notebook":
		return b.Documents.Load(r.ProjectID, r.File)
	case "save_notebook":
		return b.Documents.Save(ctx, r.ProjectID, r.File, r.Revision, r.Content)
	case "jobs":
		return b.Platform.Store.List(ctx)
	case "events":
		events, err := b.Platform.Store.Events(ctx, r.After)
		next := r.After
		if len(events) > 0 {
			next = events[len(events)-1].ID
		}
		return map[string]any{"events": events, "next_cursor": next, "gap": r.After > 0 && len(events) > 0 && events[0].ID > r.After+1}, err
	case "cancel_job":
		return nil, b.Platform.Cancel(ctx, r.JobID)
	case "sessions":
		return b.Kernels.List(), nil
	case "start":
		return b.Kernels.Start(r.ProjectID, r.File, r.Files, r.Internet)
	case "execute":
		id, err := b.Kernels.Execute(r.SessionID, r.Revision, r.CellID)
		return map[string]string{"execution_id": id}, err
	case "complete":
		id, err := b.Kernels.Complete(r.SessionID, r.Code, r.Cursor)
		return map[string]string{"execution_id": id}, err
	case "interrupt":
		return nil, b.Kernels.Interrupt(r.SessionID)
	case "restart":
		return b.Kernels.Restart(r.SessionID)
	case "stop":
		return nil, b.Kernels.Stop(r.SessionID, "user_stopped")
	default:
		return nil, errors.New("unknown Kaggle runtime operation")
	}
}

func RunKaggle(ctx context.Context, cfg *config.Config, domain string, raw json.RawMessage) string {
	failure := func(err error) string {
		b, _ := json.Marshal(map[string]string{"status": "error", "message": kaggle.PublicError(err)})
		return string(b)
	}
	if cfg == nil || !cfg.Kaggle.Enabled {
		return failure(errors.New("Kaggle integration is disabled"))
	}
	key, _ := filepath.Abs(cfg.Directories.DataDir)
	kaggleBackends.RLock()
	backend := kaggleBackends.byDir[key]
	kaggleBackends.RUnlock()
	if backend == nil {
		return failure(errors.New("Kaggle service is unavailable"))
	}
	perms, ok := EffectiveRuntimePermissions(ctx)
	if !ok {
		perms = RuntimePermissionsFromConfig(cfg)
	}
	access := kaggle.Access{Network: perms.AllowNetworkRequests, Write: perms.AllowFilesystemWrite && !cfg.VirtualDesktop.ReadOnly, Python: perms.AllowPython, Docker: perms.DockerEnabled && !perms.DockerReadOnly}
	var result any
	var err error
	if domain == "runtime" {
		var req KaggleRuntimeRequest
		if json.Unmarshal(raw, &req) != nil {
			return failure(errors.New("invalid runtime request"))
		}
		read := req.Operation == "status" || req.Operation == "projects" || req.Operation == "files" || req.Operation == "read_file" || req.Operation == "load_notebook" || req.Operation == "jobs" || req.Operation == "events" || req.Operation == "sessions"
		cleanup := req.Operation == "stop" || req.Operation == "interrupt" || req.Operation == "cancel_job"
		if !read && !cleanup && !access.Write {
			return failure(errors.New("filesystem writes are disabled"))
		}
		if req.Operation == "start" || req.Operation == "execute" || req.Operation == "complete" || req.Operation == "restart" {
			for _, session := range backend.Kernels.List() {
				if session.ID == req.SessionID && session.Internet {
					req.Internet = true
				}
			}
			if cfg.Kaggle.ReadOnly || !cfg.Kaggle.Local.Enabled || !cfg.Kaggle.Local.AllowExecute || !access.Python || !access.Docker {
				return failure(errors.New("local notebook execution is disabled"))
			}
			if req.Internet && (!cfg.Kaggle.Local.AllowInternet || !access.Network) {
				return failure(errors.New("local internet permission is disabled"))
			}
		}
		result, err = backend.Runtime(ctx, req)
	} else {
		var req kaggle.Request
		if json.Unmarshal(raw, &req) != nil {
			return failure(errors.New("invalid Kaggle request"))
		}
		req.Domain = domain
		ctx = kaggle.WithConfigSnapshot(ctx, cfg.Kaggle)
		result, err = backend.Platform.Execute(ctx, req, access)
	}
	if err != nil {
		return failure(err)
	}
	b, err := json.Marshal(result)
	if err != nil {
		return failure(err)
	}
	return string(b)
}

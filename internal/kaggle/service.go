package kaggle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
)

type Snapshot struct {
	Config config.KaggleConfig
	Access Access
}
type Request struct {
	ID         string          `json:"id,omitempty"`
	Domain     string          `json:"domain"`
	Operation  string          `json:"operation"`
	Parameters json.RawMessage `json:"parameters"`
	ProjectID  string          `json:"project_id,omitempty"`
	File       string          `json:"file,omitempty"`
}
type Result struct {
	Data json.RawMessage `json:"data,omitempty"`
	Job  *Job            `json:"job,omitempty"`
}
type SecretStore interface {
	ReadSecret(string) (string, error)
	WriteSecret(string, string) error
}

type Service struct {
	Store          *Store
	Root           string
	current        func() Snapshot
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	mu             sync.Mutex
	filesMu        sync.Mutex
	active         map[string]context.CancelFunc
	clientFactory  func(Credentials, time.Duration) (*Client, error)
	storageFactory func(context.Context, string, time.Duration) (*http.Client, error)
	secrets        SecretStore
	// BeginJob attaches the host's global revocation and atomic publication gate.
	// Set before exposing this service to callers.
	BeginJob func(context.Context) (context.Context, context.CancelFunc, error)
}

func NewService(parent context.Context, dataDir string, current func() Snapshot, secrets ...SecretStore) (*Service, error) {
	root := filepath.Join(dataDir, "kaggle", "projects")
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	store, err := OpenStore(filepath.Join(dataDir, config.KaggleDBFilename))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	s := &Service{Store: store, Root: root, current: current, ctx: ctx, cancel: cancel, active: map[string]context.CancelFunc{}, clientFactory: NewClient}
	s.storageFactory = storageClient
	if len(secrets) > 0 {
		s.secrets = secrets[0]
	}
	jobs, err := s.Store.List(ctx)
	if err != nil {
		store.Close()
		cancel()
		return nil, err
	}
	for _, j := range jobs {
		if j.Status == "queued" || j.Status == "running" {
			s.Store.update(ctx, j.ID, "unknown", j.Progress, j.Result, "AuraGo restarted; no mutation was replayed")
		}
	}
	s.wg.Add(1)
	go s.monitor()
	return s, nil
}
func (s *Service) Close() error       { s.cancel(); s.wg.Wait(); return s.Store.Close() }
func (s *Service) Snapshot() Snapshot { return s.current() }
func intersect(a, b Access) Access {
	return Access{Network: a.Network && b.Network, Write: a.Write && b.Write, Python: a.Python && b.Python, Docker: a.Docker && b.Docker, Admin: a.Admin && b.Admin}
}
func (s *Service) client() (Snapshot, *Client, error) {
	snap := s.current()
	c, err := s.clientFactory(Credentials{Mode: snap.Config.AuthMode, Token: snap.Config.Token, Username: snap.Config.Username, Key: snap.Config.LegacyKey}, time.Duration(snap.Config.RequestTimeoutSeconds)*time.Second)
	return snap, c, err
}
func (s *Service) Test(ctx context.Context) (map[string]any, error) {
	snap, c, err := s.client()
	if err != nil {
		return nil, err
	}
	if !snap.Access.Network {
		return nil, errors.New("network requests are disabled")
	}
	account, err := c.Identity(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"connected": true, "account": account}, nil
}
func (s *Service) Status() map[string]any {
	snap := s.current()
	used, _ := s.storageBytes()
	return map[string]any{"enabled": snap.Config.Enabled, "read_only": snap.Config.ReadOnly, "auth_mode": snap.Config.AuthMode, "configured": snap.Config.Token != "" || snap.Config.LegacyKey != "", "storage_bytes": used, "max_storage_bytes": int64(snap.Config.MaxStorageMB) << 20, "local": snap.Config.Local, "policy": snap.Config, "access": map[string]bool{"network": snap.Access.Network, "write": snap.Access.Write, "python": snap.Access.Python, "docker": snap.Access.Docker}}
}

func (s *Service) Execute(ctx context.Context, request Request, access Access) (Result, error) {
	op, err := Lookup(request.Domain, request.Operation)
	if err != nil {
		return Result{}, err
	}
	params, _, err := Decode(op, request.Parameters)
	if err != nil {
		return Result{}, err
	}
	snap, c, err := s.client()
	if err != nil {
		return Result{}, err
	}
	if run, ok := ctx.Value(grantSnapshotKey{}).(config.KaggleConfig); ok {
		snap.Config = intersectConfig(snap.Config, run)
	}
	access = intersect(access, snap.Access)
	if !snap.Config.Enabled || !access.Network {
		return Result{}, errors.New("Kaggle is disabled or network access is denied")
	}
	if op.Grant == "read" {
		_, typed, _ := Decode(op, request.Parameters)
		b, e := c.Call(ctx, op, typed)
		return Result{Data: cleanOutput(b)}, e
	}
	if op.Name == "save_kernel" && params["kernelExecutionType"] == nil {
		params["kernelExecutionType"] = "QUICK_SAVE"
	}
	if err := CheckGrants(snap.Config, access, op, params); err != nil {
		return Result{}, err
	}
	account, err := c.Identity(ctx)
	if err != nil {
		return Result{}, err
	}
	if op.Name == "save_kernel" {
		if params["kernelExecutionType"] == nil {
			params["kernelExecutionType"] = "QUICK_SAVE"
		}
		if params["isPrivate"] == nil {
			params["isPrivate"] = true
		}
		if params["enableGpu"] == nil {
			params["enableGpu"] = false
		}
		if params["enableTpu"] == nil {
			params["enableTpu"] = false
		}
		if params["enableInternet"] == nil {
			params["enableInternet"] = false
		}
		maxSeconds := int64(snap.Config.RemoteTimeoutMinutes) * 60
		if n, ok := params["sessionTimeoutSeconds"].(json.Number); ok {
			v, _ := n.Int64()
			if v <= 0 || v > maxSeconds {
				return Result{}, errors.New("remote run exceeds configured timeout")
			}
		} else {
			params["sessionTimeoutSeconds"] = maxSeconds
		}
	}
	if op.Name == "create_dataset" || op.Name == "create_model" {
		if params["isPrivate"] == nil {
			params["isPrivate"] = true
		}
		if params["ownerSlug"] == nil {
			params["ownerSlug"] = account
		}
	}
	if err = CheckPolicy(snap.Config, access, op, params, account); err != nil {
		return Result{}, err
	}
	request.Parameters, _ = json.Marshal(params)
	if request.ID == "" {
		request.ID = newID()
	} else if !validID(request.ID) {
		return Result{}, errors.New("request ID must be 32 hexadecimal characters")
	}
	canonical, _ := json.Marshal(struct {
		Domain, Operation, Project, File string
		Parameters                       map[string]any
	}{request.Domain, request.Operation, request.ProjectID, request.File, params})
	hash := sha256.Sum256(canonical)
	encoded, _ := json.Marshal(request)
	s.mu.Lock()
	defer s.mu.Unlock()
	j, fresh, err := s.Store.admit(ctx, Job{ID: request.ID, Account: account, Domain: op.Domain, Operation: op.Name, Request: encoded, Hash: hex.EncodeToString(hash[:])})
	if err != nil {
		return Result{}, err
	}
	if !fresh {
		return Result{Job: &j}, nil
	}
	if op.Grant == "execute" || op.Name == "save_kernel" && stringValue(params, "kernelExecutionType") == "SAVE_AND_RUN_ALL" {
		if err := s.checkComputeSlots(ctx, snap.Config); err != nil {
			s.Store.update(ctx, j.ID, "failed", 0, nil, err.Error())
			j.Status, j.Error = "failed", err.Error()
			return Result{Job: &j}, nil
		}
	}
	jobctx, cancel := context.WithCancel(s.ctx)
	if s.BeginJob != nil {
		guarded, done, err := s.BeginJob(jobctx)
		if err != nil {
			cancel()
			s.Store.update(ctx, j.ID, "failed", 0, nil, PublicError(err))
			return Result{}, err
		}
		jobctx = guarded
		original := cancel
		cancel = func() { done(); original() }
	}
	s.active[j.ID] = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer cancel()
		defer func() { s.mu.Lock(); delete(s.active, j.ID); s.mu.Unlock() }()
		s.run(jobctx, j, request, op, params, access, snap.Config)
	}()
	return Result{Job: &j}, nil
}

func (s *Service) run(ctx context.Context, j Job, req Request, op Operation, params map[string]any, access Access, grants config.KaggleConfig) {
	var transferred int64
	lastProgress := time.Time{}
	ctx = context.WithValue(ctx, transferProgressKey{}, func(n int) {
		transferred += int64(n)
		if time.Since(lastProgress) >= 250*time.Millisecond {
			_ = s.Store.update(ctx, j.ID, "running", transferred, nil, "")
			lastProgress = time.Now()
		}
	})
	_ = s.Store.update(ctx, j.ID, "running", 0, nil, "")
	check := func() error {
		snap := s.current()
		return CheckPolicy(intersectConfig(snap.Config, grants), intersect(access, snap.Access), op, params, j.Account)
	}
	var data json.RawMessage
	err := check()
	if err == nil {
		snap, c, e := s.client()
		err = e
		if err == nil {
			account, e := c.Identity(ctx)
			err = e
			if err == nil && account != j.Account {
				err = errors.New("Kaggle account changed")
			}
			if err == nil {
				err = s.checkResource(ctx, c, intersectConfig(snap.Config, grants), op, params, j.Account)
			}
			if err == nil {
				err = check()
			}
			if err == nil {
				switch {
				case op.Grant == "download":
					data, err = s.download(ctx, c, op, params, req, check)
				case strings.Contains(op.Name, "upload"):
					data, err = s.upload(ctx, c, op, params, req, j.Account, check)
				default:
					err = s.resolveUploads(ctx, params, j.Account)
					if err == nil {
						raw, _ := json.Marshal(params)
						_, typed, e := Decode(op, raw)
						err = e
						if err == nil {
							data, err = c.Call(ctx, op, typed)
						}
					}
				}
			}
		}
	}
	status, message := "completed", ""
	if err != nil {
		status = "failed"
		message = PublicError(err)
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Uncertain {
			status = "unknown"
		}
	}
	if err == nil && (op.Name == "save_kernel" && stringValue(params, "kernelExecutionType") == "SAVE_AND_RUN_ALL" || op.Grant == "execute") {
		status = "remote_running"
	}
	if err == nil && op.Name == "batch_schedule_benchmark_task_runs" {
		status, message = benchmarkAcceptance(data)
	}
	if err == nil && op.Grant == "cancel" {
		status = "cancel_requested"
	}
	finalCtx, cancel := context.WithTimeout(context.WithoutCancel(s.ctx), 5*time.Second)
	defer cancel()
	_ = s.Store.update(finalCtx, j.ID, status, transferred, cleanOutput(data), message)
}

func (s *Service) checkResource(ctx context.Context, c *Client, cfg config.KaggleConfig, op Operation, p map[string]any, account string) error {
	if op.Grant == "read" || op.Grant == "download" || op.Grant == "submit" || strings.Contains(op.Name, "upload") {
		return nil
	}
	if op.Grant == "cancel" {
		return s.ownedCancellation(ctx, op, p, account)
	}
	var read Operation
	var query map[string]any
	var err error
	switch op.Domain {
	case "datasets", "models":
		owner := stringValue(p, "ownerSlug")
		if !ownerAllowed(cfg, account, owner) {
			return errors.New("Kaggle owner is missing or disallowed")
		}
		if op.Name == "create_dataset" || op.Name == "create_model" {
			return nil
		}
		resource := "dataset"
		if op.Domain == "models" {
			resource = "model"
		}
		slug := stringValue(p, resource+"Slug")
		if slug == "" {
			return errors.New("Kaggle resource slug is required")
		}
		read, err = Lookup(op.Domain, "get_"+resource)
		query = map[string]any{"ownerSlug": owner, resource + "Slug": slug}
	case "notebooks":
		owner, slug := stringValue(p, "userName"), stringValue(p, "kernelSlug")
		if op.Name == "save_kernel" {
			parts := strings.Split(stringValue(p, "slug"), "/")
			if len(parts) != 2 {
				return errors.New("notebook slug must be owner/name")
			}
			owner, slug = parts[0], parts[1]
		}
		if !ownerAllowed(cfg, account, owner) {
			return errors.New("notebook owner is disallowed")
		}
		read, err = Lookup("notebooks", "get_kernel")
		query = map[string]any{"userName": owner, "kernelSlug": slug}
	case "benchmarks":
		if op.Name == "batch_schedule_benchmark_task_runs" {
			return s.checkComputeSlots(ctx, cfg)
		}
		if op.Name == "create_benchmark_task" {
			parts := strings.Split(stringValue(p, "slug"), "/")
			if len(parts) != 2 || !ownerAllowed(cfg, account, parts[0]) {
				return errors.New("benchmark slug must name an allowed owner")
			}
			return nil
		}
		slug, ok := p["slug"].(map[string]any)
		if !ok || !ownerAllowed(cfg, account, stringValue(slug, "ownerSlug")) || stringValue(slug, "taskSlug") == "" {
			return errors.New("benchmark slug must name an allowed owner and task")
		}
		read, err = Lookup("benchmarks", "get_benchmark_task")
		query = map[string]any{"slug": slug}
	default:
		return errors.New("unsupported mutation policy")
	}
	if err != nil {
		return err
	}
	body, err := c.Call(ctx, read, query)
	if err != nil {
		var ae *APIError
		if op.Name == "save_kernel" && errors.As(err, &ae) && ae.Status == 404 && cfg.AllowCreate {
			return s.checkComputeSlotsIfRun(ctx, cfg, p)
		}
		return err
	}
	var existing any
	if err = json.Unmarshal(body, &existing); err != nil {
		return err
	}
	private, known := resourcePrivate(existing)
	if (!known || !private) && !cfg.AllowPublish {
		return errors.New("publication permission is required for public resources or unresolved visibility")
	}
	if op.Name == "save_kernel" {
		if !cfg.AllowUpdate {
			return errors.New("updating this existing notebook requires update permission")
		}
		return s.checkComputeSlotsIfRun(ctx, cfg, p)
	}
	return nil
}
func (s *Service) checkComputeSlotsIfRun(ctx context.Context, c config.KaggleConfig, p map[string]any) error {
	if stringValue(p, "kernelExecutionType") == "SAVE_AND_RUN_ALL" {
		return s.checkComputeSlots(ctx, c)
	}
	return nil
}
func (s *Service) checkComputeSlots(ctx context.Context, c config.KaggleConfig) error {
	jobs, err := s.Store.List(ctx)
	if err != nil {
		return err
	}
	n := 0
	for _, j := range jobs {
		if j.Status != "queued" && j.Status != "running" && j.Status != "unknown" && j.Status != "remote_running" && j.Status != "cancel_requested" {
			continue
		}
		var req Request
		var p map[string]any
		json.Unmarshal(j.Request, &req)
		json.Unmarshal(req.Parameters, &p)
		if j.Operation == "batch_schedule_benchmark_task_runs" || j.Operation == "save_kernel" && stringValue(p, "kernelExecutionType") == "SAVE_AND_RUN_ALL" {
			n++
		}
	}
	if n > c.MaxRemoteJobs {
		return errors.New("concurrent Kaggle compute limit reached")
	}
	return nil
}
func (s *Service) ownedCancellation(ctx context.Context, op Operation, p map[string]any, account string) error {
	jobs, err := s.Store.List(ctx)
	if err != nil {
		return err
	}
	ownedRuns := map[string]bool{}
	for _, j := range jobs {
		if j.Account != account {
			continue
		}
		var r map[string]any
		decodeResponse(j.Result, &r)
		if op.Name == "cancel_kernel_session" && fmt.Sprint(p["kernelSessionId"]) == fmt.Sprint(r["kernelSessionId"]) && r["kernelSessionId"] != nil {
			return nil
		}
		if op.Name == "api_cancel_benchmark_task_runs" && j.Operation == "batch_schedule_benchmark_task_runs" {
			for id := range benchmarkRunIDs(r) {
				ownedRuns[id] = true
			}
		}
	}
	if op.Name == "api_cancel_benchmark_task_runs" && p["taskSlugs"] == nil && p["modelVersion"] == nil && p["agentSlug"] == nil {
		wrapper, _ := p["runIds"].(map[string]any)
		ids, _ := wrapper["ids"].([]any)
		if len(ids) > 0 {
			for _, id := range ids {
				if !ownedRuns[fmt.Sprint(id)] {
					return errors.New("benchmark cancellation contains an untracked run")
				}
			}
			return nil
		}
	}
	return errors.New("cancellation requires an operation owned by this AuraGo account")
}
func (s *Service) Cancel(ctx context.Context, id string) error {
	j, err := s.Store.Get(ctx, id)
	if err != nil {
		return err
	}
	if j.Status == "cancel_requested" {
		return nil
	}
	if j.Status == "queued" || j.Status == "running" {
		s.mu.Lock()
		cancel := s.active[id]
		s.mu.Unlock()
		if cancel != nil {
			cancel()
			return nil
		}
	}
	if j.Status != "remote_running" {
		return errors.New("job is not active")
	}
	return s.cancelRemote(ctx, j)
}

func (s *Service) monitor() {
	defer s.wg.Done()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.pollRemote()
		}
	}
}

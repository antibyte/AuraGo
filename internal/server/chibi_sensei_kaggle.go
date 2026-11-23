package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"aurago/internal/chibisensei"
	"aurago/internal/kaggle"
)

// errChibiKaggleUnavailable marks a driver call that found no Kaggle service (it could not be opened).
var errChibiKaggleUnavailable = errors.New("the Kaggle service is unavailable")

// chibiKaggleEnv holds the Kaggle facts the preflight needs besides the driver calls.
type chibiKaggleEnv struct {
	VaultAvailable    bool
	CredentialsStored bool
	StorageUsedBytes  int64 // bytes in the Kaggle project storage (kaggle.max_storage_mb counts them)
}

// chibiKaggleEnvironment is implemented by the driver adapter and by the server test stub.
type chibiKaggleEnvironment interface {
	kaggleEnv() chibiKaggleEnv
}

// chibiKaggleDriver is chibisensei.KaggleDriver over the server's *kaggle.Service. Every call opens the service through
// ensureKaggleServices (cached there; a failure is errChibiKaggleUnavailable) and passes the current
// s.kaggleSnapshot().Access, so a config or Vault change applies to the next call. Gates:
//   - mutations (Execute of a non-read operation, EnsureProject, WriteFile) refuse a read-only desktop with
//     chibisensei.ErrDesktopReadOnly and check kaggle.CheckGrants before Service.Execute runs, so the host job gate
//     (BeginJob) never admits a failed job row for a refused attempt;
//   - ReadKernelLog refuses a read-only desktop the same way (its download grant needs filesystem write);
//   - Account needs kaggle.enabled;
//   - reads, Job, ActiveComputeJobs, OpenFile, RemoveFile (cleanup of files Chibi staged, A3) and Cancel (a stop) have no
//     desktop or enabled gate; Service.Execute still refuses reads while Kaggle is disabled or the network is denied.
type chibiKaggleDriver struct {
	s     *Server
	store *chibisensei.Store
	mu    sync.Mutex // serializes EnsureProject, so one installation creates one project
}

var (
	_ chibisensei.KaggleDriver = (*chibiKaggleDriver)(nil)
	_ chibiKaggleEnvironment   = (*chibiKaggleDriver)(nil)
)

func newChibiKaggleDriver(s *Server, store *chibisensei.Store) *chibiKaggleDriver {
	return &chibiKaggleDriver{s: s, store: store}
}

func (d *chibiKaggleDriver) service() (*kaggle.Service, error) {
	b, err := d.s.ensureKaggleServices()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", errChibiKaggleUnavailable, kaggle.PublicError(err))
	}
	if b == nil || b.KaggleBackend == nil || b.Platform == nil {
		return nil, errChibiKaggleUnavailable
	}
	return b.Platform, nil
}

func (d *chibiKaggleDriver) desktopReadOnly() bool {
	cfg := d.s.ConfigSnapshot()
	return cfg != nil && cfg.VirtualDesktop.ReadOnly
}

func (d *chibiKaggleDriver) Account(ctx context.Context) (string, error) {
	if !d.s.kaggleSnapshot().Config.Enabled {
		return "", errors.New("kaggle account: the Kaggle integration is disabled")
	}
	svc, err := d.service()
	if err != nil {
		return "", err
	}
	res, err := svc.Test(ctx)
	if err != nil {
		return "", fmt.Errorf("kaggle account: %w", err)
	}
	account, _ := res["account"].(string)
	if account == "" {
		return "", errors.New("kaggle account: the connection test named no account")
	}
	return account, nil
}

// admit checks a mutation before Service.Execute sees it (see the type comment).
func (d *chibiKaggleDriver) admit(op kaggle.Operation, req kaggle.Request) error {
	if d.desktopReadOnly() {
		return fmt.Errorf("kaggle %s: %w", op.Name, chibisensei.ErrDesktopReadOnly)
	}
	params, _, err := kaggle.Decode(op, req.Parameters)
	if err != nil {
		return fmt.Errorf("kaggle %s: %w", op.Name, err)
	}
	if op.Name == "save_kernel" && params["kernelExecutionType"] == nil {
		params["kernelExecutionType"] = "QUICK_SAVE" // the service applies the same default
	}
	snap := d.s.kaggleSnapshot()
	if err := kaggle.CheckGrants(snap.Config, snap.Access, op, params); err != nil {
		return fmt.Errorf("kaggle %s refused: %w: %w", op.Name, chibisensei.ErrKaggleRefused, err)
	}
	return nil
}

func (d *chibiKaggleDriver) Execute(ctx context.Context, req kaggle.Request) (kaggle.Result, error) {
	op, err := kaggle.Lookup(req.Domain, req.Operation)
	if err != nil {
		return kaggle.Result{}, err
	}
	if op.Grant != "read" {
		if err := d.admit(op, req); err != nil {
			return kaggle.Result{}, err
		}
	}
	svc, err := d.service()
	if err != nil {
		return kaggle.Result{}, err
	}
	return svc.Execute(ctx, req, d.s.kaggleSnapshot().Access)
}

func (d *chibiKaggleDriver) Job(ctx context.Context, id string) (kaggle.Job, error) {
	svc, err := d.service()
	if err != nil {
		return kaggle.Job{}, err
	}
	return svc.Store.Get(ctx, id)
}

func (d *chibiKaggleDriver) ActiveComputeJobs(ctx context.Context) ([]kaggle.Job, error) {
	svc, err := d.service()
	if err != nil {
		return nil, err
	}
	jobs, err := svc.Store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list Kaggle jobs: %w", err)
	}
	out := []kaggle.Job{}
	for _, j := range jobs {
		if chibisensei.KaggleJobActive(j.Status) && chibiComputeJob(j) {
			out = append(out, j)
		}
	}
	return out, nil
}

// chibiComputeJob mirrors checkComputeSlots (internal/kaggle/service.go:413-435): a benchmark batch, or save_kernel with
// kernelExecutionType SAVE_AND_RUN_ALL in its stored request.
func chibiComputeJob(j kaggle.Job) bool {
	if j.Operation == "batch_schedule_benchmark_task_runs" {
		return true
	}
	if j.Operation != "save_kernel" {
		return false
	}
	var req kaggle.Request
	var p map[string]any
	if json.Unmarshal(j.Request, &req) != nil || json.Unmarshal(req.Parameters, &p) != nil {
		return false
	}
	kind, _ := p["kernelExecutionType"].(string)
	return kind == "SAVE_AND_RUN_ALL"
}

// EnsureProject returns the local Kaggle project of Chibi Sensei. The id lives in the Chibi setting kaggle_project_id;
// a project that no longer resolves (row or directory gone) is created again (A28).
func (d *chibiKaggleDriver) EnsureProject(ctx context.Context) (string, error) {
	if d.desktopReadOnly() {
		return "", fmt.Errorf("kaggle project: %w", chibisensei.ErrDesktopReadOnly)
	}
	snap := d.s.kaggleSnapshot()
	if !snap.Config.Enabled || !snap.Access.Write {
		return "", errors.New("kaggle project: Kaggle is disabled or filesystem writes are not allowed")
	}
	svc, err := d.service()
	if err != nil {
		return "", err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	id, found, err := d.store.GetSetting(ctx, chibisensei.SettingKaggleProjectID)
	if err != nil {
		return "", fmt.Errorf("kaggle project setting: %w", err)
	}
	if found && id != "" {
		ok, err := chibiKaggleProjectExists(ctx, svc, id)
		if err != nil {
			return "", err
		}
		if ok {
			return id, nil
		}
	}
	p, err := svc.CreateProject(ctx, chibisensei.KaggleProjectTitle)
	if err != nil {
		return "", fmt.Errorf("create the Chibi Sensei Kaggle project: %s", kaggle.PublicError(err))
	}
	if err := d.store.SetSetting(ctx, chibisensei.SettingKaggleProjectID, p.ID); err != nil {
		return "", fmt.Errorf("store the Kaggle project id: %w", err)
	}
	return p.ID, nil
}

// chibiKaggleProjectExists reports whether the project row and its directory exist. A store error is returned, so a
// transient failure never creates a second project.
func chibiKaggleProjectExists(ctx context.Context, svc *kaggle.Service, id string) (bool, error) {
	projects, err := svc.Store.Projects(ctx)
	if err != nil {
		return false, fmt.Errorf("list Kaggle projects: %w", err)
	}
	for _, p := range projects {
		if p.ID == id {
			_, ferr := svc.Files(id)
			return ferr == nil, nil
		}
	}
	return false, nil
}

func (d *chibiKaggleDriver) WriteFile(ctx context.Context, project, name string, r io.Reader) (int64, error) {
	if d.desktopReadOnly() {
		return 0, fmt.Errorf("write %s: %w", name, chibisensei.ErrDesktopReadOnly)
	}
	svc, err := d.service()
	if err != nil {
		return 0, err
	}
	return svc.WriteFile(ctx, project, name, r, nil) // nil: the service checks enabled and write access per chunk
}

func (d *chibiKaggleDriver) OpenFile(project, name string) (*os.File, error) {
	svc, err := d.service()
	if err != nil {
		return nil, err
	}
	return svc.OpenFile(project, name)
}

func (d *chibiKaggleDriver) RemoveFile(project, name string) error {
	svc, err := d.service()
	if err != nil {
		return err
	}
	return svc.RemoveFile(project, name)
}

func (d *chibiKaggleDriver) ReadKernelLog(ctx context.Context, owner, slug string, version int, max int64) ([]byte, error) {
	if d.desktopReadOnly() {
		return nil, fmt.Errorf("read the Kaggle log: %w", chibisensei.ErrDesktopReadOnly)
	}
	svc, err := d.service()
	if err != nil {
		return nil, err
	}
	return svc.ReadKernelLog(ctx, d.s.kaggleSnapshot().Access, owner, slug, version, max)
}

func (d *chibiKaggleDriver) Cancel(ctx context.Context, jobID string) error {
	svc, err := d.service()
	if err != nil {
		return err
	}
	return svc.Cancel(ctx, jobID)
}

func (d *chibiKaggleDriver) kaggleEnv() chibiKaggleEnv {
	snap := d.s.kaggleSnapshot()
	env := chibiKaggleEnv{VaultAvailable: d.s.Vault != nil, CredentialsStored: snap.Config.Token != "" || snap.Config.LegacyKey != ""}
	if snap.Config.Enabled { // a disabled integration is never opened just for the preflight
		if svc, err := d.service(); err == nil {
			if used, ok := svc.Status()["storage_bytes"].(int64); ok {
				env.StorageUsedBytes = used
			}
		}
	}
	return env
}

// chibiKaggleReadTTL is how long the preflight reuses the Kaggle-side reads. The Training step loads three preset cards
// and has a refresh button; within the TTL they share one round of calls.
const chibiKaggleReadTTL = 20 * time.Second

// chibiKaggleRead is one round of Kaggle-side reads.
type chibiKaggleRead struct {
	ConnectionOK bool
	Account      string
	QuotaRead    bool
	QuotaKnown   bool
	QuotaHours   float64
	JobsRead     bool
	ComputeJobs  int
	UnknownJobs  int
	At           time.Time
}

// chibiKaggleReads caches one chibiKaggleRead per runtime. The key covers what changes the answers (chibiKaggleReadKey),
// so a credential change misses; the enable route calls invalidate.
type chibiKaggleReads struct {
	mu  sync.Mutex
	key string
	val chibiKaggleRead
}

func (c *chibiKaggleReads) invalidate() {
	c.mu.Lock()
	c.key = ""
	c.mu.Unlock()
}

// get returns the cached reads for key while they are younger than the TTL, unless fresh is set (prepare and start).
// It holds the lock while it reads, so concurrent preflights wait for one round instead of starting more.
func (c *chibiKaggleReads) get(ctx context.Context, key string, fresh bool, now time.Time, drv chibisensei.KaggleDriver) chibiKaggleRead {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !fresh && c.key != "" && c.key == key && now.Sub(c.val.At) < chibiKaggleReadTTL {
		return c.val
	}
	v := chibiReadKaggle(ctx, drv)
	v.At = now
	if ctx.Err() == nil {
		c.key, c.val = key, v
	}
	return v
}

// chibiReadKaggle runs the connection test, then (only when it passed) the quota read, then the compute-job list.
func chibiReadKaggle(ctx context.Context, drv chibisensei.KaggleDriver) chibiKaggleRead {
	var v chibiKaggleRead
	if account, err := drv.Account(ctx); err == nil && account != "" {
		v.ConnectionOK, v.Account = true, account
	}
	if v.ConnectionOK {
		if req, err := chibisensei.NewKaggleRequest("", "get_accelerator_quota_statistics", "", "", nil); err == nil {
			if res, err := drv.Execute(ctx, req); err == nil {
				v.QuotaRead = true
				v.QuotaHours, v.QuotaKnown = chibisensei.ParseGPUQuota(res.Data)
			}
		}
	}
	if jobs, err := drv.ActiveComputeJobs(ctx); err == nil {
		v.JobsRead, v.ComputeJobs = true, len(jobs)
		for _, j := range jobs {
			if j.Status == chibisensei.KaggleJobUnknown {
				v.UnknownJobs++
			}
		}
	}
	return v
}

// chibiKaggleReadKey hashes what changes the Kaggle reads. Secrets only enter the hash, never a log.
func chibiKaggleReadKey(snap kaggle.Snapshot) string {
	h := sha256.New()
	c := snap.Config
	fmt.Fprintf(h, "%t|%t|%s|%s|%t|%t|", c.Enabled, c.ReadOnly, c.AuthMode, c.Username, snap.Access.Network, snap.Access.Write)
	io.WriteString(h, c.Token)
	io.WriteString(h, "|")
	io.WriteString(h, c.LegacyKey)
	return hex.EncodeToString(h.Sum(nil))
}

package chibisensei

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/kaggle"
)

var _ KaggleDriver = (*fakeKaggle)(nil)

// fakeProjectID is the id EnsureProject returns.
const fakeProjectID = "0123456789abcdef0123456789abcdef"

// fakeKaggle is an in-memory KaggleDriver that models kaggle.Service for the run state-machine suite:
//   - Execute validates the request with kaggle.Lookup and kaggle.Decode, like the real service.
//   - Reads answer at once (Result.Data, no job); see readLocked for the defaults.
//   - A mutation is admitted as a job in status queued and returned at once. The job moves only when it is
//     observed: each Job(id) read applies one step (Polls reads answer running, then the final status of the
//     operation's script; the default script is "completed").
//   - A known request ID returns the stored job unchanged (a replay, Fresh=false); a known ID with another
//     canonical request is an error. Only fresh admissions are side effects (Mutations).
//   - save_kernel with SAVE_AND_RUN_ALL completes as remote_running. Each observation (a Job read of that job or
//     a default get_kernel_session_status read of its slug) applies the next scripted Kaggle status (default
//     RUNNING, COMPLETE). A terminal Kaggle status turns the job completed, failed or cancelled; the last status
//     of the script repeats.
//   - A compute job over maxRemoteJobs (default 1, foreign jobs included) is admitted already failed.
//   - RestartService turns queued and running jobs into unknown, as kaggle.NewService does.
//   - A scripted outcome with Status unknown and Applied true applies the operation's side effect first and
//     still reports unknown (the request reached Kaggle but the answer was lost); without Applied nothing happened.
//   - Files live in a temporary directory, so OpenFile returns a real *os.File.
type fakeKaggle struct {
	t   *testing.T
	mu  sync.Mutex
	dir string

	account           string
	accountErr        error
	projectErr        error
	maxRemoteJobs     int
	maxSessionSeconds int64

	scripts    map[string][]fakeOutcome
	reads      map[string][]fakeRead
	logs       []fakeLog
	cancelErrs []error
	writeErrs  map[string]error
	outputs    map[string][]byte

	jobs      map[string]*fakeJob
	order     []string
	seq       int
	kernelSeq int
	datasets  map[string]*fakeDataset
	kernels   map[string]*fakeKernel
	uploads   map[string]fakeUpload
	calls     []fakeCall
}

// fakeOutcome scripts one fresh admission of an operation.
type fakeOutcome struct {
	Status           string         // final status: "" or completed (default), failed, unknown
	Error            string         // Job.Error for failed and unknown (a default text when empty)
	Result           map[string]any // merged into the default result of a completed job
	Polls            int            // Job reads that answer running before the final status
	Remote           []string       // save_kernel SAVE_AND_RUN_ALL: Kaggle statuses per observation (default RUNNING, COMPLETE)
	ExecErr          error          // Execute returns this error; nothing is admitted ...
	AdmitBeforeError bool           // ... unless set: then the job is admitted as failed first (the host gate)
	Applied          bool           // Status unknown: the side effect happened on Kaggle before the answer was lost
}

// fakeRead scripts one answer of a read operation.
type fakeRead struct {
	Data any   // marshalled to JSON; []byte, json.RawMessage and string are used as they are
	Err  error // for example fakeNotFound()
}

// fakeLog scripts one ReadKernelLog answer.
type fakeLog struct {
	Body []byte
	Err  error
}

// fakeCall records one driver call.
type fakeCall struct {
	Method  string         // Account | Execute | Job | ActiveComputeJobs | EnsureProject | WriteFile | OpenFile | RemoveFile | ReadKernelLog | Cancel
	Op      string         // Execute: the operation
	ID      string         // Execute: the request (job) ID; Job and Cancel: the job ID
	Project string         // Execute, WriteFile, OpenFile, RemoveFile
	File    string         // Execute, WriteFile, OpenFile, RemoveFile; ReadKernelLog: owner/slug
	Version int            // ReadKernelLog
	Max     int64          // ReadKernelLog
	Params  map[string]any // Execute: decoded parameters (numbers are json.Number)
	Fresh   bool           // Execute of a mutation: a new job was admitted
	Err     error
}

// fakeUpload is a completed upload receipt.
type fakeUpload struct {
	JobID, Name, SHA256 string
	Bytes               int64
}

type fakeJob struct {
	job     kaggle.Job
	hash    string
	op      kaggle.Operation
	params  map[string]any
	project string
	file    string
	outcome fakeOutcome
	polls   int
	compute bool
	kernel  string // save_kernel: owner/slug
}

type fakeDataset struct {
	ref     string
	files   []string
	deleted bool
}

type fakeKernel struct {
	ref     string
	jobID   string
	version int
	remote  []string
	status  string
	deleted bool
}

func newFakeKaggle(t *testing.T) *fakeKaggle {
	t.Helper()
	return &fakeKaggle{t: t, dir: t.TempDir(), account: "tester", maxRemoteJobs: 1, maxSessionSeconds: 720 * 60,
		scripts: map[string][]fakeOutcome{}, reads: map[string][]fakeRead{}, writeErrs: map[string]error{},
		outputs: map[string][]byte{}, jobs: map[string]*fakeJob{}, datasets: map[string]*fakeDataset{},
		kernels: map[string]*fakeKernel{}, uploads: map[string]fakeUpload{}}
}

// ---- scripting -------------------------------------------------------------------------------------------

// Script appends outcomes for the next fresh admissions of op, consumed in order; afterwards the default applies.
func (f *fakeKaggle) Script(op string, outcomes ...fakeOutcome) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.scripts[op] = append(f.scripts[op], outcomes...)
}

// ScriptRead appends answers for a read operation; each call takes the next, the last one repeats.
func (f *fakeKaggle) ScriptRead(op string, answers ...fakeRead) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads[op] = append(f.reads[op], answers...)
}

// ScriptLogs appends ReadKernelLog answers; each call takes the next, the last one repeats. Without a script the
// log is empty ("no log yet").
func (f *fakeKaggle) ScriptLogs(logs ...fakeLog) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, logs...)
}

// SetOutputs replaces the kernel output files that download_kernel_output and list_kernel_session_output serve.
func (f *fakeKaggle) SetOutputs(files map[string][]byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.outputs = map[string][]byte{}
	for k, v := range files {
		f.outputs[k] = append([]byte(nil), v...)
	}
}

// SetOutput sets one kernel output file.
func (f *fakeKaggle) SetOutput(name string, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.outputs[name] = append([]byte(nil), body...)
}

// SetAccount sets the account and an error for Account and for every mutation (Identity fails before admission).
func (f *fakeKaggle) SetAccount(account string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.account, f.accountErr = account, err
}

// FailEnsureProject makes EnsureProject return err until reset with nil.
func (f *fakeKaggle) FailEnsureProject(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.projectErr = err
}

// FailWrite makes the next WriteFile of name return err (once).
func (f *fakeKaggle) FailWrite(name string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writeErrs[name] = err
}

// FailCancel makes the next Cancel calls return these errors in order, whatever the job status.
func (f *fakeKaggle) FailCancel(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelErrs = append(f.cancelErrs, errs...)
}

// SetMaxRemoteJobs sets kaggle.max_remote_jobs (default 1).
func (f *fakeKaggle) SetMaxRemoteJobs(n int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.maxRemoteJobs = n
}

// SetMaxSessionSeconds sets kaggle.remote_timeout_minutes×60 (default 720×60).
func (f *fakeKaggle) SetMaxSessionSeconds(n int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.maxSessionSeconds = n
}

// SetQuota scripts the answer of get_accelerator_quota_statistics (raw JSON or a value); it replaces earlier answers.
func (f *fakeKaggle) SetQuota(data any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads["get_accelerator_quota_statistics"] = []fakeRead{{Data: data}}
}

// AddForeignComputeJob adds a compute job of another app (not a Chibi mutation) in the given status.
func (f *fakeKaggle) AddForeignComputeJob(status string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := fakeRandomID()
	f.seq++
	op, _ := kaggle.Lookup("notebooks", "save_kernel")
	f.jobs[id] = &fakeJob{job: kaggle.Job{ID: id, Account: f.account, Domain: "notebooks", Operation: "save_kernel",
		Status: status, Created: fakeStamp(f.seq), Updated: fakeStamp(f.seq)}, op: op, compute: true}
	f.order = append(f.order, id)
	return id
}

// SetJobStatus forces a job status (and Job.Error) without running anything.
func (f *fakeKaggle) SetJobStatus(id, status, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if j := f.jobs[id]; j != nil {
		f.setLocked(j, status, message, nil)
	}
}

// RestartService models an AuraGo restart of the Kaggle service: queued and running jobs become unknown;
// remote_running and cancel_requested stay.
func (f *fakeKaggle) RestartService() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, id := range f.order {
		j := f.jobs[id]
		if j.job.Status == KaggleJobQueued || j.job.Status == KaggleJobRunning {
			f.setLocked(j, KaggleJobUnknown, "AuraGo restarted; no mutation was replayed", nil)
		}
	}
}

// ---- recorded calls and state ----------------------------------------------------------------------------

// Calls returns every recorded call in order.
func (f *fakeKaggle) Calls() []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeCall(nil), f.calls...)
}

func (f *fakeKaggle) filter(keep func(fakeCall) bool) []fakeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []fakeCall
	for _, c := range f.calls {
		if keep(c) {
			out = append(out, c)
		}
	}
	return out
}

// CallsTo returns the calls of one driver method.
func (f *fakeKaggle) CallsTo(method string) []fakeCall {
	return f.filter(func(c fakeCall) bool { return c.Method == method })
}

// ExecCalls returns every Execute of op (reads, admissions, replays and refusals).
func (f *fakeKaggle) ExecCalls(op string) []fakeCall {
	return f.filter(func(c fakeCall) bool { return c.Method == "Execute" && c.Op == op })
}

// Mutations returns the fresh admissions of op ("" = every operation): the real side effects.
func (f *fakeKaggle) Mutations(op string) []fakeCall {
	return f.filter(func(c fakeCall) bool { return c.Method == "Execute" && c.Fresh && (op == "" || c.Op == op) })
}

// Replays returns the Executes of op that hit a known ID and returned the stored receipt.
func (f *fakeKaggle) Replays(op string) []fakeCall {
	return f.filter(func(c fakeCall) bool {
		return c.Method == "Execute" && !c.Fresh && c.Err == nil && c.Op == op && c.ID != "" && f.isMutation(c.Op)
	})
}

func (f *fakeKaggle) isMutation(op string) bool {
	o, err := kaggle.Lookup(kaggleOpDomains[op], op)
	return err == nil && o.Grant != "read"
}

// PeekJob returns a job without observing (advancing) it.
func (f *fakeKaggle) PeekJob(id string) (kaggle.Job, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	j, ok := f.jobs[id]
	if !ok {
		return kaggle.Job{}, false
	}
	return j.job, true
}

// JobsOf returns the jobs of op in admission order, without observing them.
func (f *fakeKaggle) JobsOf(op string) []kaggle.Job {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []kaggle.Job
	for _, id := range f.order {
		if j := f.jobs[id]; j.job.Operation == op {
			out = append(out, j.job)
		}
	}
	return out
}

// Upload returns the receipt of a completed upload job.
func (f *fakeKaggle) Upload(jobID string) (fakeUpload, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.uploads[jobID]
	return u, ok
}

// Dataset returns the file names of a dataset (owner/slug) and whether it exists and is not deleted.
func (f *fakeKaggle) Dataset(ref string) ([]string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := f.datasets[ref]
	if d == nil || d.deleted {
		return nil, false
	}
	return append([]string(nil), d.files...), true
}

// Kernel returns the current Kaggle status and version of a notebook (owner/slug) and whether it exists.
func (f *fakeKaggle) Kernel(ref string) (status string, version int, live bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := f.kernels[ref]
	if k == nil || k.deleted {
		return "", 0, false
	}
	return k.status, k.version, true
}

// ProjectFiles lists the files in the staging project, sorted.
func (f *fakeKaggle) ProjectFiles() []string {
	entries, _ := os.ReadDir(filepath.Join(f.dir, fakeProjectID))
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// assertMutations checks the number of fresh admissions per operation; an operation not listed must have none.
func assertMutations(t *testing.T, f *fakeKaggle, want map[string]int) {
	t.Helper()
	got := map[string]int{}
	for _, c := range f.Mutations("") {
		got[c.Op]++
	}
	for op, n := range want {
		if got[op] != n {
			t.Errorf("%s admitted %d times, want %d", op, got[op], n)
		}
	}
	for op, n := range got {
		if _, ok := want[op]; !ok {
			t.Errorf("%s admitted %d times, want none", op, n)
		}
	}
}

// ---- KaggleDriver ----------------------------------------------------------------------------------------

func (f *fakeKaggle) record(c fakeCall) { f.calls = append(f.calls, c) }

func (f *fakeKaggle) Account(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(fakeCall{Method: "Account", Err: f.accountErr})
	if f.accountErr != nil {
		return "", f.accountErr
	}
	return f.account, nil
}

func (f *fakeKaggle) Execute(ctx context.Context, req kaggle.Request) (kaggle.Result, error) {
	if err := ctx.Err(); err != nil {
		return kaggle.Result{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fakeCall{Method: "Execute", Op: req.Operation, ID: req.ID, Project: req.ProjectID, File: req.File}
	op, err := kaggle.Lookup(req.Domain, req.Operation)
	if err == nil {
		call.Params, _, err = kaggle.Decode(op, req.Parameters)
	}
	if err != nil {
		call.Err = err
		f.record(call)
		return kaggle.Result{}, err
	}
	if op.Grant == "read" {
		data, err := f.readLocked(op.Name, call.Params)
		call.Err = err
		f.record(call)
		return kaggle.Result{Data: data}, err
	}
	res, fresh, err := f.admitLocked(op, req, call.Params)
	call.Fresh, call.Err = fresh, err
	if res.Job != nil {
		call.ID = res.Job.ID
	}
	f.record(call)
	return res, err
}

func (f *fakeKaggle) admitLocked(op kaggle.Operation, req kaggle.Request, params map[string]any) (kaggle.Result, bool, error) {
	if f.accountErr != nil {
		return kaggle.Result{}, false, f.accountErr
	}
	if op.Name == "save_kernel" {
		if n, ok := params["sessionTimeoutSeconds"].(json.Number); ok {
			if v, _ := n.Int64(); v <= 0 || v > f.maxSessionSeconds {
				return kaggle.Result{}, false, errors.New("remote run exceeds configured timeout")
			}
		}
	}
	id := req.ID
	if id == "" {
		id = fakeRandomID()
	} else if !fakeValidID(id) {
		return kaggle.Result{}, false, errors.New("request ID must be 32 hexadecimal characters")
	}
	canonical, _ := json.Marshal(struct {
		Domain, Operation, Project, File string
		Parameters                       map[string]any
	}{op.Domain, op.Name, req.ProjectID, req.File, params})
	sum := sha256.Sum256(canonical)
	hash := hex.EncodeToString(sum[:])
	if j, ok := f.jobs[id]; ok {
		if j.hash != hash {
			return kaggle.Result{}, false, errors.New("request ID already belongs to a different account or request")
		}
		job := j.job
		return kaggle.Result{Job: &job}, false, nil
	}
	var out fakeOutcome
	if queue := f.scripts[op.Name]; len(queue) > 0 {
		out, f.scripts[op.Name] = queue[0], queue[1:]
	}
	if out.ExecErr != nil && !out.AdmitBeforeError {
		return kaggle.Result{}, false, out.ExecErr
	}
	f.seq++
	j := &fakeJob{job: kaggle.Job{ID: id, Account: f.account, Domain: op.Domain, Operation: op.Name,
		Status: KaggleJobQueued, Created: fakeStamp(f.seq), Updated: fakeStamp(f.seq)},
		hash: hash, op: op, params: params, project: req.ProjectID, file: req.File, outcome: out, polls: out.Polls,
		compute: op.Name == "save_kernel" && fakeString(params, "kernelExecutionType") == "SAVE_AND_RUN_ALL"}
	f.jobs[id] = j
	f.order = append(f.order, id)
	if out.ExecErr != nil {
		f.setLocked(j, KaggleJobFailed, out.ExecErr.Error(), nil)
		return kaggle.Result{}, true, out.ExecErr
	}
	if j.compute && f.activeComputeLocked() > f.maxRemoteJobs {
		f.setLocked(j, KaggleJobFailed, "concurrent Kaggle compute limit reached", nil)
	}
	job := j.job
	return kaggle.Result{Job: &job}, true, nil
}

func (f *fakeKaggle) activeComputeLocked() int {
	n := 0
	for _, j := range f.jobs {
		if j.compute && KaggleJobActive(j.job.Status) {
			n++
		}
	}
	return n
}

func (f *fakeKaggle) setLocked(j *fakeJob, status, message string, result json.RawMessage) {
	j.job.Status, j.job.Error = status, message
	if result != nil {
		j.job.Result = result
	}
	f.seq++
	j.job.Updated = fakeStamp(f.seq)
}

func (f *fakeKaggle) Job(ctx context.Context, id string) (kaggle.Job, error) {
	if err := ctx.Err(); err != nil {
		return kaggle.Job{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(fakeCall{Method: "Job", ID: id})
	j, ok := f.jobs[id]
	if !ok {
		return kaggle.Job{}, sql.ErrNoRows
	}
	f.observeLocked(j)
	return j.job, nil
}

func (f *fakeKaggle) observeLocked(j *fakeJob) {
	switch j.job.Status {
	case KaggleJobQueued, KaggleJobRunning:
		if j.polls > 0 {
			j.polls--
			f.setLocked(j, KaggleJobRunning, "", nil)
			return
		}
		f.finishLocked(j)
	case KaggleJobRemoteRunning, KaggleJobCancelRequested:
		if k := f.kernels[j.kernel]; k != nil && k.jobID == j.job.ID {
			f.advanceKernelLocked(k)
		}
	}
}

func (f *fakeKaggle) finishLocked(j *fakeJob) {
	out := j.outcome
	switch out.Status {
	case KaggleJobUnknown:
		if out.Applied {
			_, _, _ = f.applyLocked(j) // the request reached Kaggle; only the answer was lost
		}
		msg := out.Error
		if msg == "" {
			msg = "Kaggle outcome is unknown; this mutation was not retried"
		}
		f.setLocked(j, KaggleJobUnknown, msg, nil)
		return
	case KaggleJobFailed:
		msg := out.Error
		if msg == "" {
			msg = "Kaggle request failed (HTTP 500)"
		}
		f.setLocked(j, KaggleJobFailed, msg, nil)
		return
	case "", KaggleJobCompleted:
	default:
		f.t.Errorf("fakeOutcome.Status %q is not completed, failed or unknown", out.Status)
		f.setLocked(j, KaggleJobFailed, "bad fake script", nil)
		return
	}
	result, status, err := f.applyLocked(j)
	if err != nil {
		f.setLocked(j, KaggleJobFailed, err.Error(), nil)
		return
	}
	for k, v := range out.Result {
		result[k] = v
	}
	b, _ := json.Marshal(result)
	f.setLocked(j, status, "", b)
}

// fakeNotFound is what real Kaggle answers for a dataset, notebook or session that does not exist under the
// caller's own account: HTTP 403 PERMISSION_DENIED, not 404 (measured 2026-11-25).
func fakeNotFound() error { return &kaggle.APIError{Status: 403} }

func (f *fakeKaggle) applyLocked(j *fakeJob) (map[string]any, string, error) {
	p := j.params
	switch j.op.Name {
	case "upload_dataset_file":
		if j.project != fakeProjectID {
			return nil, "", errors.New("invalid project directory")
		}
		b, err := os.ReadFile(filepath.Join(f.dir, j.project, j.file))
		if err != nil {
			return nil, "", fmt.Errorf("open %s: file does not exist", j.file)
		}
		sum := sha256.Sum256(b)
		f.uploads[j.job.ID] = fakeUpload{JobID: j.job.ID, Name: j.file, SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(b))}
		return map[string]any{"upload_ref": "upload:" + j.job.ID, "bytes": len(b), "file": j.file}, KaggleJobCompleted, nil
	case "create_dataset":
		owner := fakeString(p, "ownerSlug")
		if owner == "" {
			owner = f.account
		}
		ref := owner + "/" + fakeString(p, "slug")
		if d := f.datasets[ref]; d != nil && !d.deleted {
			return nil, "", errors.New("Kaggle rejected the requested operation")
		}
		files, _ := p["files"].([]any)
		names := []string{}
		for _, item := range files {
			m, _ := item.(map[string]any)
			token := fakeString(m, "token")
			up, ok := f.uploads[strings.TrimPrefix(token, "upload:")]
			if !strings.HasPrefix(token, "upload:") || !ok {
				return nil, "", errors.New("Kaggle rejected the requested operation")
			}
			names = append(names, up.Name)
		}
		f.datasets[ref] = &fakeDataset{ref: ref, files: names}
		return map[string]any{"ref": ref, "url": KaggleDatasetURL(ref), "status": "Ok"}, KaggleJobCompleted, nil
	case "save_kernel":
		ref := fakeString(p, "slug")
		if strings.Count(ref, "/") != 1 {
			return nil, "", errors.New("notebook slug must be owner/name")
		}
		version := 1
		if k := f.kernels[ref]; k != nil && !k.deleted {
			version = k.version + 1
		}
		k := &fakeKernel{ref: ref, jobID: j.job.ID, version: version}
		status := KaggleJobCompleted
		if j.compute {
			status = KaggleJobRemoteRunning
			k.remote = append([]string(nil), j.outcome.Remote...)
			if len(k.remote) == 0 {
				k.remote = []string{"RUNNING", "COMPLETE"}
			}
		} else {
			k.status = "COMPLETE"
		}
		f.kernels[ref] = k
		j.kernel = ref
		f.kernelSeq++
		return map[string]any{"ref": ref, "url": KaggleNotebookURL(ref), "versionNumber": version,
			"kernelId": 9000 + f.kernelSeq}, status, nil
	case "download_kernel_output":
		if j.project == "" {
			return nil, "", errors.New("download requires a destination project")
		}
		if j.project != fakeProjectID {
			return nil, "", errors.New("invalid project directory")
		}
		name := j.file
		if name == "" {
			name = j.job.ID + ".zip"
		}
		body, ok := f.outputs[fakeString(p, "filePath")]
		if !ok {
			// A missing output file is no dataset, notebook or session; its real answer is not measured, so it keeps 404.
			return nil, "", &kaggle.APIError{Status: 404}
		}
		dest := filepath.Join(f.dir, j.project, name)
		if _, err := os.Lstat(dest); err == nil {
			return nil, "", kaggle.ErrFileConflict
		}
		if err := os.WriteFile(dest, body, 0o600); err != nil {
			return nil, "", err
		}
		return map[string]any{"project_id": j.project, "file": name, "bytes": len(body)}, KaggleJobCompleted, nil
	case "delete_dataset":
		d := f.datasets[fakeString(p, "ownerSlug")+"/"+fakeString(p, "datasetSlug")]
		if d == nil || d.deleted {
			return nil, "", fakeNotFound()
		}
		d.deleted = true
	case "delete_kernel":
		k := f.kernels[fakeString(p, "userName")+"/"+fakeString(p, "kernelSlug")]
		if k == nil || k.deleted {
			return nil, "", fakeNotFound()
		}
		k.deleted = true
	}
	return map[string]any{}, KaggleJobCompleted, nil
}

func (f *fakeKaggle) advanceKernelLocked(k *fakeKernel) {
	if len(k.remote) > 0 {
		k.status = k.remote[0]
		if len(k.remote) > 1 {
			k.remote = k.remote[1:]
		}
	}
	j := f.jobs[k.jobID]
	if j == nil || (j.job.Status != KaggleJobRemoteRunning && j.job.Status != KaggleJobCancelRequested) {
		return
	}
	result := map[string]any{}
	_ = json.Unmarshal(j.job.Result, &result)
	result["remote_status"] = k.status
	b, _ := json.Marshal(result)
	status := j.job.Status
	if final := fakeRemoteState(k.status); final != "" {
		status = final
	}
	f.setLocked(j, status, j.job.Error, b)
}

// readLocked answers a read: the scripted answer first; otherwise the modelled default:
//   - get_dataset, get_dataset_status: a live dataset → {"ref","isPrivate":true} / {"status":"ready"}; else 403 (as real Kaggle).
//     PROVISIONAL status vocabulary until the proof run.
//   - get_kernel: a live notebook → {"ref","isPrivate":true}; else 403.
//   - get_kernel_session_status: a live notebook (and matching versionLabel) → advances it one step and answers
//     {"status": <current>}; else 403.
//   - list_kernel_session_output: {"files":[{"fileName","size"}],"nextPageToken":""} from SetOutputs (PROVISIONAL shape).
//   - get_accelerator_quota_statistics: fakeQuotaJSON(27.5) (PROVISIONAL shape).
//   - anything else: {}.
func (f *fakeKaggle) readLocked(name string, p map[string]any) (json.RawMessage, error) {
	if answers := f.reads[name]; len(answers) > 0 {
		a := answers[0]
		if len(answers) > 1 {
			f.reads[name] = answers[1:]
		}
		if a.Err != nil {
			return nil, a.Err
		}
		return fakeJSON(a.Data), nil
	}
	switch name {
	case "get_dataset", "get_dataset_status":
		ref := fakeString(p, "ownerSlug") + "/" + fakeString(p, "datasetSlug")
		if d := f.datasets[ref]; d == nil || d.deleted {
			return nil, fakeNotFound()
		}
		if name == "get_dataset" {
			return fakeJSON(map[string]any{"ref": ref, "isPrivate": true}), nil
		}
		return fakeJSON(map[string]any{"status": "ready"}), nil
	case "get_kernel", "get_kernel_session_status":
		ref := fakeString(p, "userName") + "/" + fakeString(p, "kernelSlug")
		k := f.kernels[ref]
		if k == nil || k.deleted {
			return nil, fakeNotFound()
		}
		if name == "get_kernel" {
			return fakeJSON(map[string]any{"ref": ref, "isPrivate": true}), nil
		}
		if v := fakeString(p, "versionLabel"); v != "" && v != fmt.Sprint(k.version) {
			return nil, fakeNotFound()
		}
		f.advanceKernelLocked(k)
		status := k.status
		if status == "" {
			status = "QUEUED"
		}
		return fakeJSON(map[string]any{"status": status}), nil
	case "list_kernel_session_output":
		names := make([]string, 0, len(f.outputs))
		for n := range f.outputs {
			names = append(names, n)
		}
		sort.Strings(names)
		files := []map[string]any{}
		for _, n := range names {
			files = append(files, map[string]any{"fileName": n, "size": len(f.outputs[n])})
		}
		return fakeJSON(map[string]any{"files": files, "nextPageToken": ""}), nil
	case "get_accelerator_quota_statistics":
		return fakeJSON(fakeQuotaJSON(27.5)), nil
	}
	return json.RawMessage(`{}`), nil
}

func (f *fakeKaggle) ActiveComputeJobs(ctx context.Context) ([]kaggle.Job, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(fakeCall{Method: "ActiveComputeJobs"})
	out := []kaggle.Job{}
	for _, id := range f.order {
		if j := f.jobs[id]; j.compute && KaggleJobActive(j.job.Status) {
			out = append(out, j.job)
		}
	}
	return out, nil
}

func (f *fakeKaggle) EnsureProject(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(fakeCall{Method: "EnsureProject", Err: f.projectErr})
	if f.projectErr != nil {
		return "", f.projectErr
	}
	if err := os.MkdirAll(filepath.Join(f.dir, fakeProjectID), 0o700); err != nil {
		return "", err
	}
	return fakeProjectID, nil
}

func (f *fakeKaggle) WriteFile(ctx context.Context, project, name string, r io.Reader) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	body, readErr := io.ReadAll(r) // read before locking: r may be a pipe fed by the caller
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fakeCall{Method: "WriteFile", Project: project, File: name}
	fail := func(err error) (int64, error) {
		call.Err = err
		f.record(call)
		return 0, err
	}
	if err, ok := f.writeErrs[name]; ok {
		delete(f.writeErrs, name)
		return fail(err)
	}
	if readErr != nil {
		return fail(fmt.Errorf("read upload source: %w", readErr))
	}
	if project != fakeProjectID {
		return fail(errors.New("invalid project directory"))
	}
	if !fakeValidFile(name) {
		return fail(errors.New("invalid project filename"))
	}
	dest := filepath.Join(f.dir, project, name)
	if err := os.WriteFile(dest+".tmp", body, 0o600); err != nil {
		return fail(err)
	}
	if err := os.Rename(dest+".tmp", dest); err != nil {
		return fail(err)
	}
	f.record(call)
	return int64(len(body)), nil
}

func (f *fakeKaggle) OpenFile(project, name string) (*os.File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fakeCall{Method: "OpenFile", Project: project, File: name}
	var file *os.File
	var err error
	switch {
	case project != fakeProjectID:
		err = errors.New("invalid project directory")
	case !fakeValidFile(name):
		err = errors.New("invalid project filename")
	default:
		file, err = os.Open(filepath.Join(f.dir, project, name))
	}
	call.Err = err
	f.record(call)
	return file, err
}

func (f *fakeKaggle) RemoveFile(project, name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fakeCall{Method: "RemoveFile", Project: project, File: name}
	var err error
	switch {
	case project != fakeProjectID:
		err = errors.New("invalid project directory")
	case !fakeValidFile(name):
		err = errors.New("invalid project filename")
	default:
		if e := os.Remove(filepath.Join(f.dir, project, name)); e != nil && !errors.Is(e, fs.ErrNotExist) {
			err = e
		}
	}
	call.Err = err
	f.record(call)
	return err
}

func (f *fakeKaggle) ReadKernelLog(ctx context.Context, owner, slug string, version int, max int64) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fakeCall{Method: "ReadKernelLog", File: owner + "/" + slug, Version: version, Max: max}
	if len(f.logs) == 0 {
		f.record(call)
		return nil, nil
	}
	l := f.logs[0]
	if len(f.logs) > 1 {
		f.logs = f.logs[1:]
	}
	call.Err = l.Err
	f.record(call)
	if l.Err != nil {
		return nil, l.Err
	}
	b := l.Body
	if max > 0 && int64(len(b)) > max {
		b = b[int64(len(b))-max:]
	}
	return append([]byte(nil), b...), nil
}

func (f *fakeKaggle) Cancel(ctx context.Context, jobID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	call := fakeCall{Method: "Cancel", ID: jobID}
	err := f.cancelLocked(jobID)
	call.Err = err
	f.record(call)
	return err
}

func (f *fakeKaggle) cancelLocked(id string) error {
	if len(f.cancelErrs) > 0 {
		err := f.cancelErrs[0]
		f.cancelErrs = f.cancelErrs[1:]
		return err
	}
	j, ok := f.jobs[id]
	if !ok {
		return sql.ErrNoRows
	}
	switch j.job.Status {
	case KaggleJobCancelRequested:
		return nil
	case KaggleJobQueued, KaggleJobRunning:
		f.setLocked(j, KaggleJobFailed, "context canceled", nil)
		return nil
	case KaggleJobRemoteRunning:
		r := map[string]any{}
		_ = json.Unmarshal(j.job.Result, &r)
		if r["kernelSessionId"] == nil {
			return errors.New("Kaggle did not return a verifiable session ID; cancel this run on Kaggle")
		}
		f.setLocked(j, KaggleJobCancelRequested, "", nil)
		return nil
	}
	return errors.New("job is not active")
}

// ---- helpers ---------------------------------------------------------------------------------------------

func fakeRandomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func fakeValidID(s string) bool {
	if len(s) != 32 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// fakeValidFile copies kaggle.validFile (transfers.go:37-43).
func fakeValidFile(name string) bool {
	stem := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if stem == "CON" || stem == "PRN" || stem == "AUX" || stem == "NUL" || len(stem) == 4 &&
		(strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '0' && stem[3] <= '9' {
		return false
	}
	return name != "" && name != "." && name != ".." && len(name) < 240 && !strings.ContainsAny(name, "/\\:\x00") &&
		!strings.HasPrefix(name, ".") && strings.TrimSpace(name) == name && !strings.HasSuffix(name, ".")
}

// fakeRemoteState copies kaggle.remoteState (remote_jobs.go:142-153).
func fakeRemoteState(v string) string {
	v = strings.ToUpper(v)
	switch {
	case v == "COMPLETE" || v == "COMPLETED" || strings.HasSuffix(v, "_SUCCEEDED") || strings.HasSuffix(v, "_COMPLETED"):
		return KaggleJobCompleted
	case v == "ERROR" || v == "FAILED" || strings.HasSuffix(v, "_FAILED") || strings.HasSuffix(v, "_ERRORED"):
		return KaggleJobFailed
	case v == "CANCELLED" || v == "CANCELED" || v == "CANCEL_ACKNOWLEDGED" || strings.HasSuffix(v, "_CANCELLED"):
		return KaggleJobCancelled
	}
	return ""
}

func fakeStamp(n int) string {
	return time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC).Add(time.Duration(n) * time.Second).Format(time.RFC3339Nano)
}

func fakeString(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func fakeJSON(v any) json.RawMessage {
	switch x := v.(type) {
	case json.RawMessage:
		return x
	case []byte:
		return x
	case string:
		return json.RawMessage(x)
	}
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// ---- builders --------------------------------------------------------------------------------------------

// fakeQuotaJSON is a PROVISIONAL quota answer with the given remaining GPU hours (replace it with the proof
// run's real shape).
func fakeQuotaJSON(hours float64) map[string]any {
	used := 108000 - int(hours*3600)
	return map[string]any{"acceleratorQuotaStatistics": []any{
		map[string]any{"acceleratorType": "GPU", "quotaLimit": "108000s", "quotaUsed": fmt.Sprintf("%ds", used)},
		map[string]any{"acceleratorType": "TPU", "quotaLimit": "72000s", "quotaUsed": "0s"},
	}}
}

// fakeLogLine is one trainer line: "@@chibi " + {"v":1,"seq","ts","t", fields…}.
func fakeLogLine(seq int, t string, fields map[string]any) string {
	m := map[string]any{"v": 1, "seq": seq, "ts": fakeStamp(seq), "t": t}
	for k, v := range fields {
		m[k] = v
	}
	b, _ := json.Marshal(m)
	return "@@chibi " + string(b)
}

// fakeKernelLogJSON wraps lines in the JSON-array log form [{"stream_name","time","data"}].
func fakeKernelLogJSON(lines ...string) []byte {
	entries := make([]map[string]any, 0, len(lines))
	for i, l := range lines {
		entries = append(entries, map[string]any{"stream_name": "stdout", "time": 1.5 + float64(i), "data": l + "\n"})
	}
	b, _ := json.Marshal(entries)
	return b
}

// fakeKernelLogText is the plain-text log form.
func fakeKernelLogText(lines ...string) []byte { return []byte(strings.Join(lines, "\n") + "\n") }

// fakeTrainLog is a train stage start plus a metric line every 10 steps up to lastStep.
func fakeTrainLog(lastStep, maxSteps int) []string {
	lines := []string{fakeLogLine(1, "stage", map[string]any{"stage": "train", "state": "start"})}
	seq := 2
	for s := 10; s <= lastStep; s += 10 {
		frac := float64(s) / float64(maxSteps)
		lines = append(lines, fakeLogLine(seq, "metric", map[string]any{"step": s, "max_steps": maxSteps, "epoch": frac,
			"loss": 2.0 - frac, "lr": 0.0002, "grad_norm": 1.0, "tok_s": 2400.0, "elapsed_s": float64(s),
			"eta_s": float64(maxSteps - s), "vram_gb": 6.1}))
		seq++
	}
	return lines
}

// Defaults of fakeOutputs for the manifest header (a valid UUID, profile id and sha256).
const (
	fakeDefaultRunID     = "11111111-2222-4333-8444-555555555555"
	fakeDefaultProfileID = "fake-profile"
)

var fakeDefaultProfileSHA = func() string { s := sha256.Sum256([]byte(fakeDefaultProfileID)); return hex.EncodeToString(s[:]) }()

// fakeNull as a fakeSample answer emits JSON null (the trainer produced no answer; its timing is null too).
const fakeNull = "\x00null"

// fakeRunOutputs describes a set of kernel outputs.
type fakeRunOutputs struct {
	RunID, ProfileID, ProfileSHA string // defaults: fakeDefaultRunID, fakeDefaultProfileID, its sha256
	Failed                       bool   // status failed with Code and Message; no GGUF, samples or adapter
	Code, Message                string
	StopReason                   string // default "done"
	Steps                        int    // default 40
	Arch                         string // GGUF architecture, default "gemma3"
	BaseGGUF                     bool   // also chibi-base-q8_0.gguf
	Adapter                      bool   // also chibi-adapter-model.safetensors and chibi-adapter-config.json
	Samples                      []fakeSample
	WrongSHA                     string         // manifest path whose sha256 is replaced by 64 zeros
	Extra                        map[string]any // merged into the status object (for example "warning_code")
}

// fakeSample is one eval sample; Base or Tuned may be fakeNull.
type fakeSample struct{ Expected, Base, Tuned string }

// fakeOutputs builds the output files of one run, keyed by file name, with a consistent manifest.
func fakeOutputs(o fakeRunOutputs) map[string][]byte {
	if o.RunID == "" {
		o.RunID = fakeDefaultRunID
	}
	if o.ProfileID == "" {
		o.ProfileID = fakeDefaultProfileID
	}
	if o.ProfileSHA == "" {
		o.ProfileSHA = fakeDefaultProfileSHA
	}
	if o.StopReason == "" {
		o.StopReason = "done"
	}
	if o.Steps == 0 {
		o.Steps = 40
	}
	if o.Arch == "" {
		o.Arch = "gemma3"
	}
	if len(o.Samples) == 0 {
		o.Samples = []fakeSample{{"pos", "neg", "pos"}, {"neg", "neg", "neg"}, {"pos", "pos", "pos"}}
	}
	files, kinds := map[string][]byte{}, map[string]string{}
	add := func(name, kind string, b []byte) { files[name], kinds[name] = b, kind }
	add(OutMetrics, "metrics", fakeMetricsJSONL(o.Steps))
	if !o.Failed {
		add(OutEvalSamples, "eval_samples", fakeEvalSamplesJSONL(o.Samples))
		add(OutModelGGUF, "gguf_tuned", fakeGGUF(o.Arch))
		if o.BaseGGUF {
			add(OutBaseGGUF, "gguf_base", fakeGGUF(o.Arch))
		}
		if o.Adapter {
			add(OutAdapter, "adapter", []byte("fake-safetensors"))
			add(OutAdapterConfig, "adapter_config", []byte(`{"r":16,"lora_alpha":32}`))
		}
	}
	add(OutStatus, "status", fakeStatusJSON(o))
	files[OutManifest] = fakeManifestJSON(o, files, kinds)
	return files
}

func fakeStatusJSON(o fakeRunOutputs) []byte {
	train := map[string]any{"steps": o.Steps, "epochs": 2.0, "train_loss": 0.92, "eval_loss_base": 2.31,
		"eval_loss_tuned": 1.12, "stop_reason": o.StopReason, "truncated_rows": 0}
	st := map[string]any{"v": 1, "run_id": o.RunID, "state": "succeeded", "stage": "manifest", "code": "", "message": "",
		"warning_code": "", "started": "2026-11-23T10:00:00Z", "finished": "2026-11-23T10:30:00Z",
		"gpu":      map[string]any{"name": "Tesla T4", "capability": "7.5", "vram_gb": 15.0},
		"versions": map[string]any{"python": "3.11.13", "torch": "2.6.0", "transformers": "4.57.0", "peft": "0.17.1", "llama_cpp": "b9994"},
		"train":    train,
		"discovery": map[string]any{"gpu_name": "Tesla T4", "lora_targets": []string{"q_proj", "v_proj"},
			"converter_env": "system"}}
	if o.Failed {
		st["state"], st["stage"], st["code"], st["message"] = "failed", "train", o.Code, o.Message
		train["eval_loss_tuned"] = nil
	}
	for k, v := range o.Extra {
		st[k] = v
	}
	b, _ := json.Marshal(st)
	return b
}

func fakeManifestJSON(o fakeRunOutputs, files map[string][]byte, kinds map[string]string) []byte {
	names := make([]string, 0, len(kinds))
	for n := range kinds {
		names = append(names, n)
	}
	sort.Strings(names)
	list := []map[string]any{}
	for _, n := range names {
		sum := sha256.Sum256(files[n])
		sha := hex.EncodeToString(sum[:])
		if n == o.WrongSHA {
			sha = strings.Repeat("0", 64)
		}
		list = append(list, map[string]any{"path": n, "kind": kinds[n], "bytes": len(files[n]), "sha256": sha})
	}
	b, _ := json.Marshal(map[string]any{"v": 1, "run_id": o.RunID, "profile_id": o.ProfileID,
		"profile_sha": o.ProfileSHA, "files": list})
	return b
}

// fakeMetricsJSONL writes train metric objects (the log objects without the prefix), one every 10 steps.
func fakeMetricsJSONL(steps int) []byte {
	var buf bytes.Buffer
	for _, line := range fakeTrainLog(steps, steps)[1:] {
		buf.WriteString(strings.TrimPrefix(line, "@@chibi "))
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// fakeEvalSamplesJSONL writes one line per sample; ids are "eval-1", "eval-2", …; an answer of fakeNull is JSON
// null together with its timing.
func fakeEvalSamplesJSONL(samples []fakeSample) []byte {
	var buf bytes.Buffer
	answer := func(s string) (any, any) {
		if s == fakeNull {
			return nil, nil
		}
		return s, 110
	}
	for i, s := range samples {
		base, baseMS := answer(s.Base)
		tuned, tunedMS := answer(s.Tuned)
		b, _ := json.Marshal(map[string]any{"id": fmt.Sprintf("eval-%d", i+1),
			"prompt_messages": []map[string]string{{"role": "user", "content": fmt.Sprintf("question %d", i+1)}},
			"expected":        s.Expected, "base_output": base, "tuned_output": tuned, "base_ms": baseMS, "tuned_ms": tunedMS,
			"truncated": false})
		buf.Write(b)
		buf.WriteByte('\n')
	}
	return buf.Bytes()
}

// fakeGGUF is a minimal valid GGUF v3 file: magic, version 3, one tensor, two string keys
// (general.architecture = arch, general.name), one tensor info and 32-byte aligned tensor data.
func fakeGGUF(arch string) []byte {
	var b bytes.Buffer
	w := func(v any) { _ = binary.Write(&b, binary.LittleEndian, v) }
	str := func(s string) { w(uint64(len(s))); b.WriteString(s) }
	b.WriteString("GGUF")
	w(uint32(3)) // version
	w(uint64(1)) // tensor count
	w(uint64(2)) // metadata key-value count
	str("general.architecture")
	w(uint32(8)) // GGUF_TYPE_STRING
	str(arch)
	str("general.name")
	w(uint32(8))
	str("chibi-fake")
	str("t")     // tensor name
	w(uint32(1)) // n_dims
	w(uint64(1)) // dim 0
	w(uint32(0)) // GGML_TYPE_F32
	w(uint64(0)) // offset
	for b.Len()%32 != 0 {
		b.WriteByte(0)
	}
	w(float32(0))
	return b.Bytes()
}

// ---- self-tests ------------------------------------------------------------------------------------------

func fakeExec(t *testing.T, f *fakeKaggle, id, op, project, file string, params any) (kaggle.Result, error) {
	t.Helper()
	req, err := NewKaggleRequest(id, op, project, file, params)
	if err != nil {
		t.Fatal(err)
	}
	return f.Execute(context.Background(), req)
}

func TestFakeKaggleReplayUnknownAndSlots(t *testing.T) {
	ctx := context.Background()
	f := newFakeKaggle(t)
	proj, _ := f.EnsureProject(ctx)
	if _, err := f.WriteFile(ctx, proj, "r1-train.jsonl", strings.NewReader("{}\n")); err != nil {
		t.Fatal(err)
	}
	f.Script("upload_dataset_file", fakeOutcome{Status: KaggleJobUnknown})
	id := KaggleRequestID("run", "upload_train", 0)
	res, err := fakeExec(t, f, id, "upload_dataset_file", proj, "r1-train.jsonl", nil)
	if err != nil || res.Job.Status != KaggleJobQueued {
		t.Fatalf("admit: %+v %v", res.Job, err)
	}
	if j, _ := f.Job(ctx, id); j.Status != KaggleJobUnknown {
		t.Fatalf("status %q, want unknown", j.Status)
	}
	res, err = fakeExec(t, f, id, "upload_dataset_file", proj, "r1-train.jsonl", nil)
	if err != nil || res.Job.Status != KaggleJobUnknown || len(f.Replays("upload_dataset_file")) != 1 {
		t.Fatalf("replay: %+v %v", res.Job, err)
	}
	if _, err := fakeExec(t, f, id, "upload_dataset_file", proj, "other.jsonl", nil); err == nil {
		t.Fatal("same ID with another request was accepted")
	}
	f.AddForeignComputeJob(KaggleJobUnknown)
	res, _ = fakeExec(t, f, KaggleRequestID("run", "save_kernel", 0), "save_kernel", "", "", map[string]any{
		"slug": "tester/chibi-x", "kernelExecutionType": "SAVE_AND_RUN_ALL", "enableGpu": true})
	if res.Job.Status != KaggleJobFailed || res.Job.Error != "concurrent Kaggle compute limit reached" {
		t.Fatalf("slot: %+v", res.Job)
	}
	assertMutations(t, f, map[string]int{"upload_dataset_file": 1, "save_kernel": 1})
}

func fakeSaveRun(t *testing.T, f *fakeKaggle, step, slug string) kaggle.Job {
	t.Helper()
	res, err := fakeExec(t, f, KaggleRequestID("run", step, 0), "save_kernel", "", "", map[string]any{
		"slug": slug, "kernelExecutionType": "SAVE_AND_RUN_ALL", "enableGpu": true})
	if err != nil {
		t.Fatal(err)
	}
	return *res.Job
}

func fakeRemoteStatus(t *testing.T, j kaggle.Job) string {
	t.Helper()
	var r map[string]any
	_ = json.Unmarshal(j.Result, &r)
	s, _ := r["remote_status"].(string)
	return s
}

func TestFakeKaggleRemoteSequenceAndCancel(t *testing.T) {
	ctx := context.Background()
	f := newFakeKaggle(t)
	f.SetMaxRemoteJobs(5)
	f.Script("save_kernel",
		fakeOutcome{Remote: []string{"QUEUED", "RUNNING", "RUNNING", "COMPLETE"}},
		fakeOutcome{Remote: []string{"RUNNING"}},
		fakeOutcome{Remote: []string{"CANCEL_ACKNOWLEDGED"}, Result: map[string]any{"kernelSessionId": 7}})

	j1 := fakeSaveRun(t, f, "a", "tester/chibi-x")
	if j1.Status != KaggleJobQueued {
		t.Fatalf("admitted as %q", j1.Status)
	}
	got, _ := f.Job(ctx, j1.ID)
	if got.Status != KaggleJobRemoteRunning || fakeRemoteStatus(t, got) != "" {
		t.Fatalf("first read: %s %q", got.Status, fakeRemoteStatus(t, got))
	}
	if _, v, live := f.Kernel("tester/chibi-x"); !live || v != 1 {
		t.Fatalf("kernel version %d live %v", v, live)
	}
	for _, want := range []string{"QUEUED", "RUNNING", "RUNNING"} {
		got, _ = f.Job(ctx, j1.ID)
		if got.Status != KaggleJobRemoteRunning || fakeRemoteStatus(t, got) != want {
			t.Fatalf("read: %s %q, want remote_running %s", got.Status, fakeRemoteStatus(t, got), want)
		}
	}
	got, _ = f.Job(ctx, j1.ID)
	if got.Status != KaggleJobCompleted || fakeRemoteStatus(t, got) != "COMPLETE" {
		t.Fatalf("final: %s %q", got.Status, fakeRemoteStatus(t, got))
	}
	if got, _ = f.Job(ctx, j1.ID); got.Status != KaggleJobCompleted {
		t.Fatalf("a terminal job moved to %s", got.Status)
	}

	// No session ID: the cancel fails and the job keeps running.
	j2 := fakeSaveRun(t, f, "b", "tester/chibi-y")
	f.Job(ctx, j2.ID)
	if err := f.Cancel(ctx, j2.ID); err == nil || !strings.Contains(err.Error(), "verifiable session ID") {
		t.Fatalf("cancel without session: %v", err)
	}
	if j, _ := f.PeekJob(j2.ID); j.Status != KaggleJobRemoteRunning {
		t.Fatalf("job %s after failed cancel", j.Status)
	}

	// With a session ID: cancel_requested, then the scripted acknowledgement cancels the job.
	j3 := fakeSaveRun(t, f, "c", "tester/chibi-z")
	f.Job(ctx, j3.ID)
	if err := f.Cancel(ctx, j3.ID); err != nil {
		t.Fatal(err)
	}
	if j, _ := f.PeekJob(j3.ID); j.Status != KaggleJobCancelRequested {
		t.Fatalf("job %s, want cancel_requested", j.Status)
	}
	if got, _ = f.Job(ctx, j3.ID); got.Status != KaggleJobCancelled || fakeRemoteStatus(t, got) != "CANCEL_ACKNOWLEDGED" {
		t.Fatalf("after ack: %s %q", got.Status, fakeRemoteStatus(t, got))
	}
	if err := f.Cancel(ctx, j3.ID); err == nil {
		t.Fatal("cancel of a cancelled job succeeded")
	}

	f.FailCancel(errors.New("x"))
	if err := f.Cancel(ctx, j2.ID); err == nil || err.Error() != "x" {
		t.Fatalf("scripted cancel error: %v", err)
	}

	// Restart: a queued upload becomes unknown; the remote job stays.
	proj, _ := f.EnsureProject(ctx)
	if _, err := f.WriteFile(ctx, proj, "r1-a.jsonl", strings.NewReader("{}\n")); err != nil {
		t.Fatal(err)
	}
	up, err := fakeExec(t, f, KaggleRequestID("run", "up", 0), "upload_dataset_file", proj, "r1-a.jsonl", nil)
	if err != nil {
		t.Fatal(err)
	}
	f.RestartService()
	if j, _ := f.PeekJob(up.Job.ID); j.Status != KaggleJobUnknown {
		t.Fatalf("upload %s after restart, want unknown", j.Status)
	}
	if j, _ := f.PeekJob(j2.ID); j.Status != KaggleJobRemoteRunning {
		t.Fatalf("remote job %s after restart, want remote_running", j.Status)
	}
}

func TestFakeKaggleDatasetDownloadAndBuilders(t *testing.T) {
	ctx := context.Background()
	f := newFakeKaggle(t)
	proj, _ := f.EnsureProject(ctx)
	var refs []map[string]any
	for _, name := range []string{"r1-train.jsonl", "r1-job.json"} {
		if _, err := f.WriteFile(ctx, proj, name, strings.NewReader("{}\n")); err != nil {
			t.Fatal(err)
		}
		res, err := fakeExec(t, f, KaggleRequestID("run", "up-"+name, 0), "upload_dataset_file", proj, name, nil)
		if err != nil {
			t.Fatal(err)
		}
		j, _ := f.Job(ctx, res.Job.ID)
		if j.Status != KaggleJobCompleted {
			t.Fatalf("upload %s: %s %s", name, j.Status, j.Error)
		}
		refs = append(refs, map[string]any{"token": "upload:" + j.ID})
	}
	res, err := fakeExec(t, f, KaggleRequestID("run", "ds", 0), "create_dataset", "", "", map[string]any{
		"slug": "chibi-data-x", "title": "chibi data x", "files": refs})
	if err != nil {
		t.Fatal(err)
	}
	if j, _ := f.Job(ctx, res.Job.ID); j.Status != KaggleJobCompleted {
		t.Fatalf("create_dataset: %s %s", j.Status, j.Error)
	}
	if names, ok := f.Dataset("tester/chibi-data-x"); !ok || len(names) != 2 || names[0] != "r1-train.jsonl" || names[1] != "r1-job.json" {
		t.Fatalf("dataset %v %v", names, ok)
	}
	r, err := fakeExec(t, f, "", "get_dataset_status", "", "", map[string]any{"ownerSlug": "tester", "datasetSlug": "chibi-data-x"})
	if err != nil || strings.ReplaceAll(string(r.Data), " ", "") != `{"status":"ready"}` {
		t.Fatalf("get_dataset_status: %s %v", r.Data, err)
	}
	if _, err := fakeExec(t, f, "", "get_dataset", "", "", map[string]any{"ownerSlug": "tester", "datasetSlug": "nope"}); !IsKaggleNotFound(err) {
		t.Fatalf("get_dataset of an unknown slug: %v", err)
	}

	// Downloads.
	f.SetOutputs(fakeOutputs(fakeRunOutputs{WrongSHA: OutModelGGUF}))
	download := func(step, output, file string) kaggle.Job {
		res, err := fakeExec(t, f, KaggleRequestID("run", step, 0), "download_kernel_output", proj, file, map[string]any{
			"ownerSlug": "tester", "kernelSlug": "chibi-x", "filePath": output})
		if err != nil {
			t.Fatal(err)
		}
		j, _ := f.Job(ctx, res.Job.ID)
		return j
	}
	if j := download("d1", OutStatus, "r1-chibi-status.json"); j.Status != KaggleJobCompleted {
		t.Fatalf("download: %s %s", j.Status, j.Error)
	}
	file, err := f.OpenFile(proj, "r1-chibi-status.json")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(file)
	file.Close()
	if !bytes.Equal(body, fakeOutputs(fakeRunOutputs{})[OutStatus]) {
		t.Fatalf("downloaded body differs: %s", body)
	}
	if j := download("d2", OutStatus, "r1-chibi-status.json"); j.Status != KaggleJobFailed || !strings.Contains(j.Error, "project file revision changed") {
		t.Fatalf("second download: %s %q", j.Status, j.Error)
	}
	if j := download("d3", "chibi-missing.json", "r1-missing.json"); j.Status != KaggleJobFailed || j.Error != "Kaggle request failed (HTTP 404)" {
		t.Fatalf("missing output: %s %q", j.Status, j.Error)
	}

	// Builders against the production parsers.
	out := fakeOutputs(fakeRunOutputs{WrongSHA: OutModelGGUF, BaseGGUF: true, Adapter: true})
	m, err := ParseManifest(out[OutManifest])
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if len(m.Files) != 7 {
		t.Fatalf("manifest lists %d files, want 7", len(m.Files))
	}
	for _, e := range m.Files {
		sum := sha256.Sum256(out[e.Path])
		want := hex.EncodeToString(sum[:])
		if e.Path == OutModelGGUF {
			want = strings.Repeat("0", 64)
		}
		if e.SHA256 != want || e.Bytes != int64(len(out[e.Path])) || ManifestKindFile[e.Kind] != e.Path {
			t.Errorf("manifest entry %+v does not match (want sha %s)", e, want)
		}
	}
	st, err := ParseStatus(out[OutStatus])
	if err != nil || !st.Succeeded() || st.Train.Epochs != 2 || st.Train.EvalLossTuned == nil || len(st.Discovery) == 0 {
		t.Fatalf("status: %+v %v", st, err)
	}
	if st, err = ParseStatus(fakeOutputs(fakeRunOutputs{Failed: true, Code: "out_of_memory", Message: "boom",
		Extra: map[string]any{"warning_code": "slow_gpu"}})[OutStatus]); err != nil || st.Succeeded() || st.Code != "out_of_memory" ||
		st.WarningCode != "slow_gpu" || st.Train.EvalLossTuned != nil {
		t.Fatalf("failed status: %+v %v", st, err)
	}
	if failed := fakeOutputs(fakeRunOutputs{Failed: true, Code: "x_y"}); failed[OutModelGGUF] != nil || failed[OutEvalSamples] != nil {
		t.Fatal("a failed run carries a GGUF or samples")
	} else if _, err := ParseManifest(failed[OutManifest]); err != nil {
		t.Fatalf("failed manifest: %v", err)
	}
	pts, err := ParseMetricsFile(bytes.NewReader(out[OutMetrics]))
	if err != nil || len(pts) != 4 || pts[0].Step != 10 || pts[3].Step != 40 || pts[0].Kind != "train" || pts[3].Loss != 1 {
		t.Fatalf("metrics: %+v %v", pts, err)
	}
	samples, err := ParseEvalSamples(bytes.NewReader(out[OutEvalSamples]), 0)
	if err != nil || len(samples) != 3 || samples[0].ID != "eval-1" || samples[0].Expected != "pos" ||
		samples[0].BaseOutput == nil || *samples[0].BaseOutput != "neg" || samples[0].TunedOutput == nil || *samples[0].TunedOutput != "pos" {
		t.Fatalf("samples: %+v %v", samples, err)
	}
	nulls, err := ParseEvalSamples(bytes.NewReader(fakeEvalSamplesJSONL([]fakeSample{{"pos", fakeNull, "pos"}, {"neg", "neg", fakeNull}})), 0)
	if err != nil || len(nulls) != 2 || nulls[0].BaseOutput != nil || nulls[0].BaseMS != nil || nulls[0].TunedOutput == nil ||
		nulls[1].TunedOutput != nil || nulls[1].TunedMS != nil || nulls[1].BaseOutput == nil {
		t.Fatalf("null samples: %+v %v", nulls, err)
	}

	// Log builders against ParseKernelLog.
	lines := fakeTrainLog(20, 40)
	for name, body := range map[string][]byte{"json": fakeKernelLogJSON(lines...), "text": fakeKernelLogText(lines...)} {
		evs := ParseKernelLog(body)
		if len(evs) != 3 {
			t.Fatalf("%s log: %d events, want 3", name, len(evs))
		}
		if s, ok := evs[0].Stage(); !ok || s.Stage != "train" || s.State != "start" {
			t.Errorf("%s log: stage event %+v %v", name, s, ok)
		}
		if mt, ok := evs[2].Metric(); !ok || mt.Step != 20 || mt.MaxSteps != 40 {
			t.Errorf("%s log: metric event %+v %v", name, mt, ok)
		}
	}
	for i, stage := range TrainerStages {
		if evs := ParseKernelLog([]byte(fakeLogLine(i+1, "stage", map[string]any{"stage": stage, "state": "done"}))); len(evs) != 1 {
			t.Errorf("stage %s is not accepted by the log parser", stage)
		}
	}

	// GGUF.
	g := fakeGGUF("gemma3")
	if string(g[:4]) != "GGUF" || binary.LittleEndian.Uint32(g[4:8]) != 3 || len(g)%4 != 0 ||
		binary.LittleEndian.Uint64(g[8:16]) != 1 || binary.LittleEndian.Uint64(g[16:24]) != 2 {
		t.Fatalf("bad GGUF header % x", g[:24])
	}
	if n := binary.LittleEndian.Uint64(g[24:32]); string(g[32:32+n]) != "general.architecture" {
		t.Fatalf("first key %q", g[32:32+n])
	}
	if !bytes.Contains(g, []byte("gemma3")) || !bytes.Contains(fakeGGUF("qwen3"), []byte("qwen3")) {
		t.Fatal("architecture value missing")
	}

	// Logs and quota.
	f.ScriptLogs(fakeLog{Body: []byte("abcdef")})
	if b, err := f.ReadKernelLog(ctx, "tester", "chibi-x", 1, 3); err != nil || string(b) != "def" {
		t.Fatalf("ReadKernelLog: %q %v", b, err)
	}
	if h, ok := ParseGPUQuota(fakeJSON(fakeQuotaJSON(27.5))); !ok || h != 27.5 {
		t.Fatalf("quota: %v %v", h, ok)
	}
	if q, err := fakeExec(t, f, "", "get_accelerator_quota_statistics", "", "", nil); err != nil {
		t.Fatal(err)
	} else if h, ok := ParseGPUQuota(q.Data); !ok || h != 27.5 {
		t.Fatalf("default quota read: %v %v", h, ok)
	}
}

func TestFakeKaggleUnknownAppliedOrNot(t *testing.T) {
	ctx := context.Background()
	for _, applied := range []bool{true, false} {
		f := newFakeKaggle(t)
		proj, _ := f.EnsureProject(ctx)
		if _, err := f.WriteFile(ctx, proj, "r1-a.jsonl", strings.NewReader("{}\n")); err != nil {
			t.Fatal(err)
		}
		up, _ := fakeExec(t, f, KaggleRequestID("run", "up", 0), "upload_dataset_file", proj, "r1-a.jsonl", nil)
		j, _ := f.Job(ctx, up.Job.ID)
		if j.Status != KaggleJobCompleted {
			t.Fatalf("upload %s", j.Status)
		}
		datasetParams := map[string]any{"slug": "chibi-data-x", "title": "chibi data x",
			"files": []any{map[string]any{"token": "upload:" + j.ID}}}
		f.Script("create_dataset", fakeOutcome{Status: KaggleJobUnknown, Applied: applied})
		res, err := fakeExec(t, f, KaggleRequestID("run", "ds", 0), "create_dataset", "", "", datasetParams)
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := f.Job(ctx, res.Job.ID); got.Status != KaggleJobUnknown {
			t.Fatalf("applied=%v: dataset job %s, want unknown", applied, got.Status)
		}
		if _, ok := f.Dataset("tester/chibi-data-x"); ok != applied {
			t.Fatalf("applied=%v: dataset exists=%v", applied, ok)
		}
		_, readErr := fakeExec(t, f, "", "get_dataset", "", "", map[string]any{"ownerSlug": "tester", "datasetSlug": "chibi-data-x"})
		if applied == IsKaggleNotFound(readErr) {
			t.Fatalf("applied=%v: get_dataset error %v", applied, readErr)
		}

		f.Script("save_kernel", fakeOutcome{Status: KaggleJobUnknown, Applied: applied})
		res, err = fakeExec(t, f, KaggleRequestID("run", "nb", 0), "save_kernel", "", "", map[string]any{"slug": "tester/chibi-nb"})
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := f.Job(ctx, res.Job.ID); got.Status != KaggleJobUnknown {
			t.Fatalf("applied=%v: notebook job %s, want unknown", applied, got.Status)
		}
		if _, v, live := f.Kernel("tester/chibi-nb"); live != applied || (applied && v != 1) {
			t.Fatalf("applied=%v: notebook live=%v version=%d", applied, live, v)
		}
		if _, err := fakeExec(t, f, "", "get_kernel", "", "", map[string]any{"userName": "tester", "kernelSlug": "chibi-nb"}); applied == IsKaggleNotFound(err) {
			t.Fatalf("applied=%v: get_kernel error %v", applied, err)
		}
		// A replay of the same ID still answers unknown and admits nothing new.
		if again, _ := fakeExec(t, f, KaggleRequestID("run", "nb", 0), "save_kernel", "", "", map[string]any{"slug": "tester/chibi-nb"}); again.Job.Status != KaggleJobUnknown {
			t.Fatalf("replay %s", again.Job.Status)
		}
		assertMutations(t, f, map[string]int{"upload_dataset_file": 1, "create_dataset": 1, "save_kernel": 1})
	}
}

func TestFakeKaggleScriptedErrorsAndReads(t *testing.T) {
	ctx := context.Background()
	f := newFakeKaggle(t)
	f.SetAccount("", errors.New("no credentials"))
	if _, err := f.Account(ctx); err == nil {
		t.Fatal("Account ignored its error")
	}
	f.SetAccount("tester", nil)
	f.Script("save_kernel", fakeOutcome{ExecErr: errors.New("refused")}, fakeOutcome{ExecErr: errors.New("gate"), AdmitBeforeError: true})
	params := map[string]any{"slug": "tester/chibi-x"}
	id := KaggleRequestID("run", "nb", 0)
	if _, err := fakeExec(t, f, id, "save_kernel", "", "", params); err == nil || len(f.Mutations("save_kernel")) != 0 {
		t.Fatalf("refused request admitted a job: %v", err)
	}
	if _, err := fakeExec(t, f, id, "save_kernel", "", "", params); err == nil || len(f.Mutations("save_kernel")) != 1 {
		t.Fatalf("host gate: %v", err)
	}
	if j, _ := f.PeekJob(id); j.Status != KaggleJobFailed {
		t.Fatalf("gated job %s", j.Status)
	}
	f.SetMaxSessionSeconds(60)
	if _, err := fakeExec(t, f, KaggleRequestID("run", "nb", 1), "save_kernel", "", "", map[string]any{
		"slug": "tester/chibi-x", "sessionTimeoutSeconds": 120}); err == nil {
		t.Fatal("a session timeout over the bound was admitted")
	}
	f.ScriptRead("get_kernel_session_status", fakeRead{Data: map[string]any{"status": "RUNNING"}}, fakeRead{Err: fakeNotFound()})
	read := map[string]any{"userName": "tester", "kernelSlug": "chibi-x"}
	if r, err := fakeExec(t, f, "", "get_kernel_session_status", "", "", read); err != nil || !strings.Contains(string(r.Data), "RUNNING") {
		t.Fatalf("scripted read: %s %v", r.Data, err)
	}
	if _, err := fakeExec(t, f, "", "get_kernel_session_status", "", "", read); !IsKaggleNotFound(err) {
		t.Fatalf("second scripted read: %v", err)
	}
	f.FailWrite("r1-a.jsonl", errors.New("disk full"))
	proj, _ := f.EnsureProject(ctx)
	if _, err := f.WriteFile(ctx, proj, "r1-a.jsonl", strings.NewReader("x")); err == nil {
		t.Fatal("FailWrite ignored")
	}
	if _, err := f.WriteFile(ctx, proj, "r1-a.jsonl", strings.NewReader("x")); err != nil {
		t.Fatalf("FailWrite is not once: %v", err)
	}
	if files := f.ProjectFiles(); len(files) != 1 || files[0] != "r1-a.jsonl" {
		t.Fatalf("project files %v", files)
	}
	if err := f.RemoveFile(proj, "r1-a.jsonl"); err != nil || f.RemoveFile(proj, "r1-a.jsonl") != nil {
		t.Fatal("RemoveFile must be idempotent")
	}
	if _, err := f.OpenFile(proj, "r1-a.jsonl"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("OpenFile of a removed file: %v", err)
	}
	f.FailEnsureProject(errors.New("kaggle disabled"))
	if _, err := f.EnsureProject(ctx); err == nil {
		t.Fatal("FailEnsureProject ignored")
	}
}

package chibisensei

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Job states.
const (
	JobQueued    = "queued"
	JobRunning   = "running"
	JobSucceeded = "succeeded"
	JobFailed    = "failed"
	JobCancelled = "cancelled"
)

const (
	progressInterval = 500 * time.Millisecond
	cancelWait       = 10 * time.Second
	maxListedJobs    = 50
)

// Progress is the progress of a job.
type Progress struct {
	Done  int    `json:"done"`
	Total int    `json:"total"`
	Phase string `json:"phase"`
}

// JobResult is the outcome of a job; unused fields stay empty.
type JobResult struct {
	RowsAdded   int     `json:"rows_added"`
	RowsSkipped int     `json:"rows_skipped"`
	BatchID     string  `json:"batch_id,omitempty"`
	Issues      []Issue `json:"issues,omitempty"`
	Suggestions int     `json:"suggestions,omitempty"`
	Withheld    int     `json:"withheld,omitempty"`
	Rows        int     `json:"rows,omitempty"`
	Path        string  `json:"path,omitempty"`
}

// Job is one background job of a project.
type Job struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"project_id"`
	Kind      string          `json:"kind"`
	State     string          `json:"state"`
	Progress  Progress        `json:"progress"`
	Result    JobResult       `json:"result"`
	ErrorCode string          `json:"error_code"`
	Error     string          `json:"error"`
	Created   string          `json:"created"`
	Updated   string          `json:"updated"`
	Params    json.RawMessage `json:"-"`
}

func (j Job) terminal() bool {
	return j.State == JobSucceeded || j.State == JobFailed || j.State == JobCancelled
}

// JobError is a job failure with a stable code for the UI.
type JobError struct{ Code, Message string }

func (e *JobError) Error() string { return e.Code + ": " + e.Message }

// JobFunc is the body of a job. It must stop promptly when ctx is cancelled and may return a partial result with
// the context error. report must not be called while a transaction or cursor of the store is open.
type JobFunc func(ctx context.Context, report func(Progress)) (JobResult, error)

type activeJob struct {
	project string
	cancel  context.CancelFunc
	done    chan struct{}
}

// Runner runs the jobs of all projects.
type Runner struct {
	st  *Store
	hub *Hub

	mu     sync.Mutex
	active map[string]*activeJob
	wg     sync.WaitGroup
	closed bool
	log    *slog.Logger
}

// NewRunner returns a runner; hub may be nil.
func NewRunner(st *Store, hub *Hub) *Runner {
	return &Runner{st: st, hub: hub, active: map[string]*activeJob{}, log: slog.Default()}
}

// SetLogger sets the logger that receives the causes of failed jobs (they are never stored).
func (r *Runner) SetLogger(l *slog.Logger) {
	if l == nil {
		return
	}
	r.mu.Lock()
	r.log = l
	r.mu.Unlock()
}

func (r *Runner) publish(j Job) {
	if r.hub != nil {
		r.hub.Publish(j.ProjectID, Event{Name: "job", Data: j})
	}
}

// Recover marks every queued or running job failed with error_code "interrupted" (called once at runtime open).
func (r *Runner) Recover(ctx context.Context) error {
	_, err := r.st.db.ExecContext(ctx, `UPDATE jobs SET state=?, error_code='interrupted', error=?, updated=? WHERE state IN (?,?)`,
		JobFailed, "The job was interrupted by a restart.", r.st.stamp(), JobQueued, JobRunning)
	return err
}

// Start inserts a running job and runs fn in a goroutine. See the plan for the outcome rules.
func (r *Runner) Start(parent context.Context, release func(), projectID, kind string, params any, total int, fn JobFunc) (Job, error) {
	fail := func(err error) (Job, error) {
		if release != nil {
			release()
		}
		return Job{}, err
	}
	pj, err := json.Marshal(params)
	if err != nil {
		return fail(err)
	}
	if params == nil {
		pj = []byte("{}")
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return fail(errors.New("chibisensei: job runner closed"))
	}
	var n int
	if err := r.st.db.QueryRowContext(parent, `SELECT COUNT(*) FROM jobs WHERE project_id=? AND kind=? AND state IN (?,?)`,
		projectID, kind, JobQueued, JobRunning).Scan(&n); err != nil {
		r.mu.Unlock()
		return fail(err)
	}
	if n > 0 {
		r.mu.Unlock()
		return fail(ErrJobActive)
	}
	now := r.st.stamp()
	job := Job{ID: uuid.NewString(), ProjectID: projectID, Kind: kind, State: JobRunning,
		Progress: Progress{Total: total}, Created: now, Updated: now, Params: pj}
	progJSON, _ := json.Marshal(job.Progress)
	if _, err := r.st.db.ExecContext(parent, `INSERT INTO jobs(id,project_id,kind,state,params_json,progress_json,result_json,error_code,error,created,updated)
		VALUES(?,?,?,?,?,?,'{}','','',?,?)`, job.ID, projectID, kind, job.State, string(pj), string(progJSON), now, now); err != nil {
		r.mu.Unlock()
		return fail(err)
	}
	ctx, cancel := context.WithCancel(parent)
	aj := &activeJob{project: projectID, cancel: cancel, done: make(chan struct{})}
	r.active[job.ID] = aj
	r.wg.Add(1)
	r.mu.Unlock()
	r.publish(job)
	go r.run(ctx, aj, release, job, fn)
	return job, nil
}

func (r *Runner) run(ctx context.Context, aj *activeJob, release func(), job Job, fn JobFunc) {
	defer r.wg.Done()
	defer close(aj.done)
	defer func() {
		r.mu.Lock()
		delete(r.active, job.ID)
		r.mu.Unlock()
	}()
	defer aj.cancel()
	if release != nil {
		defer release()
	}

	var pmu sync.Mutex
	last := job.Progress
	lastWrite := time.Now()
	report := func(p Progress) {
		pmu.Lock()
		last = p
		if time.Since(lastWrite) < progressInterval {
			pmu.Unlock()
			return
		}
		lastWrite = time.Now()
		pmu.Unlock()
		r.persistProgress(job, p)
	}

	var res JobResult
	var err error
	func() {
		defer func() {
			if v := recover(); v != nil {
				err = fmt.Errorf("job panicked: %v", v)
			}
		}()
		res, err = fn(ctx, report)
	}()

	pmu.Lock()
	final := last
	pmu.Unlock()
	job.Progress = final
	job.Result = res
	switch {
	case err == nil:
		job.State = JobSucceeded
	case ctx.Err() != nil || errors.Is(err, context.Canceled):
		job.State = JobCancelled
	default:
		job.State = JobFailed
		var je *JobError
		if errors.As(err, &je) {
			job.ErrorCode, job.Error = je.Code, je.Message
		} else {
			job.ErrorCode, job.Error = "internal", "The job failed."
			r.mu.Lock()
			l := r.log
			r.mu.Unlock()
			l.Warn("Chibi Sensei job failed", "job", job.ID, "kind", job.Kind, "error", err)
		}
	}
	job.Updated = r.st.stamp()
	fctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	resJSON, _ := json.Marshal(res)
	progJSON, _ := json.Marshal(final)
	if _, uerr := r.st.db.ExecContext(fctx, `UPDATE jobs SET state=?, progress_json=?, result_json=?, error_code=?, error=?, updated=? WHERE id=?`,
		job.State, string(progJSON), string(resJSON), job.ErrorCode, job.Error, job.Updated, job.ID); uerr != nil {
		r.mu.Lock()
		l := r.log
		r.mu.Unlock()
		l.Warn("Chibi Sensei job state not stored", "job", job.ID, "error", uerr)
	}
	r.publish(job)
}

func (r *Runner) persistProgress(job Job, p Progress) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pj, _ := json.Marshal(p)
	now := r.st.stamp()
	if _, err := r.st.db.ExecContext(ctx, `UPDATE jobs SET progress_json=?, updated=? WHERE id=? AND state=?`, string(pj), now, job.ID, JobRunning); err != nil {
		return
	}
	job.Progress, job.Updated = p, now
	r.publish(job)
}

func waitDone(ch chan struct{}) {
	select {
	case <-ch:
	case <-time.After(cancelWait):
	}
}

// Cancel stops a job and returns it; a terminal job is returned as is. It waits until the job has finished.
func (r *Runner) Cancel(ctx context.Context, jobID string) (Job, error) {
	j, err := r.st.GetJob(ctx, jobID)
	if err != nil {
		return Job{}, err
	}
	if j.terminal() {
		return j, nil
	}
	r.mu.Lock()
	aj := r.active[jobID]
	r.mu.Unlock()
	if aj != nil {
		aj.cancel()
		waitDone(aj.done)
	} else {
		// no goroutine owns this job (left over from a crash): close it directly
		if _, err := r.st.db.ExecContext(ctx, `UPDATE jobs SET state=?, updated=? WHERE id=? AND state IN (?,?)`,
			JobCancelled, r.st.stamp(), jobID, JobQueued, JobRunning); err != nil {
			return Job{}, err
		}
	}
	return r.st.GetJob(ctx, jobID)
}

// CancelProject cancels every job of a project and waits for them (before the project is deleted).
func (r *Runner) CancelProject(projectID string) {
	r.mu.Lock()
	var list []*activeJob
	for _, aj := range r.active {
		if aj.project == projectID {
			list = append(list, aj)
		}
	}
	r.mu.Unlock()
	for _, aj := range list {
		aj.cancel()
	}
	for _, aj := range list {
		waitDone(aj.done)
	}
}

// Close cancels all jobs and waits for their goroutines.
func (r *Runner) Close() {
	r.mu.Lock()
	r.closed = true
	for _, aj := range r.active {
		aj.cancel()
	}
	r.mu.Unlock()
	r.wg.Wait()
}

const jobCols = `id,project_id,kind,state,params_json,progress_json,result_json,error_code,error,created,updated`

func scanJob(sc rowScanner) (Job, error) {
	var j Job
	var params, prog, res string
	if err := sc.Scan(&j.ID, &j.ProjectID, &j.Kind, &j.State, &params, &prog, &res, &j.ErrorCode, &j.Error, &j.Created, &j.Updated); err != nil {
		return Job{}, err
	}
	j.Params = json.RawMessage(params)
	_ = json.Unmarshal([]byte(prog), &j.Progress)
	_ = json.Unmarshal([]byte(res), &j.Result)
	return j, nil
}

// GetJob returns one job or ErrNotFound.
func (s *Store) GetJob(ctx context.Context, id string) (Job, error) {
	j, err := scanJob(s.db.QueryRowContext(ctx, `SELECT `+jobCols+` FROM jobs WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, ErrNotFound
	}
	return j, err
}

// ListJobs lists the newest jobs of a project (at most 50), optionally only queued and running ones.
func (s *Store) ListJobs(ctx context.Context, projectID string, activeOnly bool) ([]Job, error) {
	q := `SELECT ` + jobCols + ` FROM jobs WHERE project_id=?`
	args := []any{projectID}
	if activeOnly {
		q += ` AND state IN (?,?)`
		args = append(args, JobQueued, JobRunning)
	}
	q += ` ORDER BY created DESC, rowid DESC LIMIT ?`
	rows, err := s.db.QueryContext(ctx, q, append(args, maxListedJobs)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

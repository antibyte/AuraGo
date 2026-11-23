package chibisensei

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path"
	"path/filepath"
	"strings"
)

// jsonText marshals without HTML escaping and fails on values JSON cannot carry (NaN, Inf).
func jsonText(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}

type runVals struct {
	hyper, job, remote, leftovers string
	cancel                        int
}

func runValues(r Run) (runVals, error) {
	var v runVals
	var err error
	if v.hyper, err = jsonText(r.Hyper); err != nil {
		return v, fmt.Errorf("encode hyper: %w", err)
	}
	if v.remote, err = jsonText(r.Remote); err != nil {
		return v, fmt.Errorf("encode remote: %w", err)
	}
	left := r.Leftovers
	if left == nil {
		left = []Leftover{}
	}
	if v.leftovers, err = jsonText(left); err != nil {
		return v, fmt.Errorf("encode leftovers: %w", err)
	}
	if len(r.JobJSON) > 0 && !json.Valid(r.JobJSON) {
		return v, errors.New("job_json is not valid JSON")
	}
	v.job = string(r.JobJSON)
	if r.CancelRequested {
		v.cancel = 1
	}
	return v, nil
}

const runCols = `id, COALESCE(project_id,''), COALESCE(backend,''), COALESCE(state,''), COALESCE(step_attempt,0),
	COALESCE(profile_id,''), COALESCE(profile_sha,''), COALESCE(preset,''), COALESCE(hyper_json,'{}'), COALESCE(job_json,''),
	COALESCE(data_sha,''), COALESCE(remote_json,'{}'), COALESCE(deadline,''), COALESCE(cancel_requested,0),
	COALESCE(error_code,''), COALESCE(error,''), COALESCE(leftovers_json,'[]'), COALESCE(created,''), COALESCE(updated,'')`

func scanRun(sc rowScanner) (Run, error) {
	var r Run
	var hyper, job, remote, leftovers string
	var cancel int
	if err := sc.Scan(&r.ID, &r.ProjectID, &r.Backend, &r.State, &r.StepAttempt, &r.ProfileID, &r.ProfileSHA, &r.Preset,
		&hyper, &job, &r.DataSHA, &remote, &r.Deadline, &cancel, &r.ErrorCode, &r.Error, &leftovers, &r.Created, &r.Updated); err != nil {
		return Run{}, err
	}
	if err := json.Unmarshal([]byte(hyper), &r.Hyper); err != nil {
		return Run{}, fmt.Errorf("decode run %s hyper: %w", r.ID, err)
	}
	if job != "" {
		r.JobJSON = []byte(job)
	}
	if err := json.Unmarshal([]byte(remote), &r.Remote); err != nil {
		return Run{}, fmt.Errorf("decode run %s remote: %w", r.ID, err)
	}
	if err := json.Unmarshal([]byte(leftovers), &r.Leftovers); err != nil {
		return Run{}, fmt.Errorf("decode run %s leftovers: %w", r.ID, err)
	}
	if r.Leftovers == nil {
		r.Leftovers = []Leftover{}
	}
	if r.Remote.Live == "" {
		r.Remote.Live = LiveUnknown
	}
	r.CancelRequested = cancel != 0
	return r, nil
}

func loadRun(ctx context.Context, q queryer, id string) (Run, error) {
	r, err := scanRun(q.QueryRowContext(ctx, `SELECT `+runCols+` FROM runs WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	return r, err
}

// CreateRun inserts a new run in state queued and its first "state" event. It refuses with ErrRunActive while any
// run of the installation is not terminal (D13) and with ErrNotFound when the project does not exist. The single
// store connection serializes the check and the insert.
func (s *Store) CreateRun(ctx context.Context, r Run) error {
	if !ValidID(r.ID) || !ValidID(r.ProjectID) {
		return errors.New("create run: invalid run or project id")
	}
	if r.State == "" {
		r.State = RunQueued
	}
	if r.State != RunQueued {
		return fmt.Errorf("create run: a new run starts queued, not %q", r.State)
	}
	if r.Backend == "" {
		r.Backend = BackendKaggle
	}
	if r.Remote.Live == "" {
		r.Remote.Live = LiveUnknown
	}
	if r.Remote.LastState == "" {
		r.Remote.LastState = RunQueued
	}
	r.Created = s.stamp()
	r.Updated = r.Created
	vals, err := runValues(r)
	if err != nil {
		return fmt.Errorf("create run: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE id=?`, r.ProjectID).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM runs WHERE state NOT IN `+terminalStatesSQL).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrRunActive
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO runs(id,project_id,backend,state,step_attempt,profile_id,profile_sha,preset,
		hyper_json,job_json,data_sha,remote_json,deadline,cancel_requested,error_code,error,leftovers_json,created,updated)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, r.ID, r.ProjectID, r.Backend, r.State, r.StepAttempt, r.ProfileID,
		r.ProfileSHA, r.Preset, vals.hyper, vals.job, r.DataSHA, vals.remote, r.Deadline, vals.cancel, r.ErrorCode, r.Error,
		vals.leftovers, r.Created, r.Updated); err != nil {
		return fmt.Errorf("create run: %w", err)
	}
	if _, err := s.addRunEvent(ctx, tx, r.ID, NewStateRunEvent(RunQueued)); err != nil {
		return err
	}
	return tx.Commit()
}

// GetRun returns one run or ErrNotFound.
func (s *Store) GetRun(ctx context.Context, id string) (Run, error) { return loadRun(ctx, s.db, id) }

func (s *Store) queryRuns(ctx context.Context, query string, args ...any) ([]Run, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+runCols+` FROM runs `+query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListRuns lists the runs of a project, newest first; limit defaults to 20 and is capped at 100.
func (s *Store) ListRuns(ctx context.Context, projectID string, limit int) ([]Run, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.queryRuns(ctx, `WHERE project_id=? ORDER BY created DESC, rowid DESC LIMIT ?`, projectID, limit)
}

// ActiveRuns lists every non-terminal run, oldest first (the orchestrator resumes them at boot).
func (s *Store) ActiveRuns(ctx context.Context) ([]Run, error) {
	return s.queryRuns(ctx, `WHERE state NOT IN `+terminalStatesSQL+` ORDER BY created, rowid`)
}

// ListRunIDs lists the IDs of every run of a project (the project-delete file hook).
func (s *Store) ListRunIDs(ctx context.Context, projectID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM runs WHERE project_id=? ORDER BY created, rowid`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// UpdateRun loads the run, calls fn, stores it and, when ev.Kind is not empty, appends ev, all in one transaction.
// fn cannot change ID, ProjectID, Backend or Created; an error from fn rolls everything back. It publishes nothing;
// the caller publishes after the commit.
func (s *Store) UpdateRun(ctx context.Context, id string, ev RunEvent, fn func(*Run) error) (Run, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Run{}, err
	}
	defer tx.Rollback()
	cur, err := loadRun(ctx, tx, id)
	if err != nil {
		return Run{}, err
	}
	next := cur
	if err := fn(&next); err != nil {
		return Run{}, err
	}
	next.ID, next.ProjectID, next.Backend, next.Created = cur.ID, cur.ProjectID, cur.Backend, cur.Created
	if !validRunState(next.State) {
		return Run{}, fmt.Errorf("update run %s: unknown state %q", id, next.State)
	}
	if next.Leftovers == nil {
		next.Leftovers = []Leftover{}
	}
	if next.Remote.Live == "" {
		next.Remote.Live = LiveUnknown
	}
	next.Updated = s.stamp()
	vals, err := runValues(next)
	if err != nil {
		return Run{}, fmt.Errorf("update run %s: %w", id, err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE runs SET state=?, step_attempt=?, profile_id=?, profile_sha=?, preset=?,
		hyper_json=?, job_json=?, data_sha=?, remote_json=?, deadline=?, cancel_requested=?, error_code=?, error=?,
		leftovers_json=?, updated=? WHERE id=?`, next.State, next.StepAttempt, next.ProfileID, next.ProfileSHA, next.Preset,
		vals.hyper, vals.job, next.DataSHA, vals.remote, next.Deadline, vals.cancel, next.ErrorCode, next.Error,
		vals.leftovers, next.Updated, id); err != nil {
		return Run{}, fmt.Errorf("update run %s: %w", id, err)
	}
	if ev.Kind != "" {
		if _, err := s.addRunEvent(ctx, tx, id, ev); err != nil {
			return Run{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Run{}, err
	}
	return next, nil
}

// DeleteRun deletes a terminal run with its events, metrics, reports and artifact rows. It returns ErrNotFound or
// ErrRunNotTerminal. Files are not touched: the caller then calls RemoveRunFiles (B4, artifacts.go).
func (s *Store) DeleteRun(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var state string
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(state,'') FROM runs WHERE id=?`, id).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !RunTerminal(state) {
		return ErrRunNotTerminal
	}
	for _, q := range []string{
		`DELETE FROM run_events WHERE run_id=?`,
		`DELETE FROM metrics WHERE run_id=?`,
		`DELETE FROM eval_reports WHERE run_id=?`,
		`DELETE FROM artifacts WHERE run_id=?`,
		`DELETE FROM runs WHERE id=?`,
	} {
		if _, err := tx.ExecContext(ctx, q, id); err != nil {
			return fmt.Errorf("delete run %s: %w", id, err)
		}
	}
	return tx.Commit()
}

// addRunEvent inserts one event and prunes the run to the newest maxRunEvents events.
func (s *Store) addRunEvent(ctx context.Context, q queryer, runID string, ev RunEvent) (int64, error) {
	if ev.Kind == "" {
		return 0, errors.New("run event: empty kind")
	}
	data := ev.Data
	if len(data) == 0 {
		data = json.RawMessage(`{}`)
	}
	if !json.Valid(data) {
		return 0, errors.New("run event: data is not valid JSON")
	}
	ts := ev.TS
	if ts == "" {
		ts = s.stamp()
	}
	res, err := q.ExecContext(ctx, `INSERT INTO run_events(run_id,ts,kind,data_json) VALUES(?,?,?,?)`, runID, ts, ev.Kind, string(data))
	if err != nil {
		return 0, fmt.Errorf("add run event: %w", err)
	}
	id, _ := res.LastInsertId()
	if _, err := q.ExecContext(ctx, `DELETE FROM run_events WHERE run_id=? AND id <= (SELECT id FROM run_events WHERE run_id=?
		ORDER BY id DESC LIMIT 1 OFFSET ?)`, runID, runID, maxRunEvents); err != nil {
		return 0, fmt.Errorf("prune run events: %w", err)
	}
	return id, nil
}

// AddRunEvent appends one event and keeps the newest 5,000 per run.
func (s *Store) AddRunEvent(ctx context.Context, runID string, ev RunEvent) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := s.addRunEvent(ctx, tx, runID, ev); err != nil {
		return err
	}
	return tx.Commit()
}

// ListRunEvents returns the events with id > afterID in id order; limit defaults to 200, at most 1000.
func (s *Store) ListRunEvents(ctx context.Context, runID string, afterID int64, limit int) ([]RunEvent, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, COALESCE(ts,''), COALESCE(kind,''), COALESCE(data_json,'{}')
		FROM run_events WHERE run_id=? AND id>? ORDER BY id LIMIT ?`, runID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []RunEvent{}
	for rows.Next() {
		var e RunEvent
		var data string
		if err := rows.Scan(&e.ID, &e.TS, &e.Kind, &data); err != nil {
			return nil, err
		}
		e.Data = json.RawMessage(data)
		out = append(out, e)
	}
	return out, rows.Err()
}

func finiteOr0(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return v
}

// insertMetrics upserts points; a point with an unknown kind, a negative step or a non-finite loss is skipped.
func insertMetrics(ctx context.Context, tx *sql.Tx, runID string, m []Metric) error {
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO metrics(run_id,step,kind,epoch,loss,lr,grad_norm,tok_s,ts)
		VALUES(?,?,?,?,?,?,?,?,?) ON CONFLICT(run_id,step,kind) DO UPDATE SET epoch=excluded.epoch, loss=excluded.loss,
		lr=excluded.lr, grad_norm=excluded.grad_norm, tok_s=excluded.tok_s, ts=excluded.ts`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, p := range m {
		if (p.Kind != "train" && p.Kind != "eval") || p.Step < 0 || math.IsNaN(p.Loss) || math.IsInf(p.Loss, 0) {
			continue
		}
		if _, err := stmt.ExecContext(ctx, runID, p.Step, p.Kind, finiteOr0(p.Epoch), p.Loss, finiteOr0(p.LR),
			finiteOr0(p.GradNorm), finiteOr0(p.TokS), p.TS); err != nil {
			return fmt.Errorf("store metric: %w", err)
		}
	}
	return nil
}

// UpsertMetrics adds or replaces live points by (step, kind).
func (s *Store) UpsertMetrics(ctx context.Context, runID string, m []Metric) error {
	if len(m) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertMetrics(ctx, tx, runID, m); err != nil {
		return err
	}
	return tx.Commit()
}

// ReplaceMetrics replaces every point of the run (chibi-metrics.jsonl after the run).
func (s *Store) ReplaceMetrics(ctx context.Context, runID string, m []Metric) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM metrics WHERE run_id=?`, runID); err != nil {
		return err
	}
	if err := insertMetrics(ctx, tx, runID, m); err != nil {
		return err
	}
	return tx.Commit()
}

const maxListedMetrics = 20000

// ListMetrics returns the points with step >= sinceStep (inclusive) ordered by step and kind, at most 20,000.
func (s *Store) ListMetrics(ctx context.Context, runID string, sinceStep int) ([]Metric, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT step, COALESCE(kind,''), COALESCE(epoch,0), COALESCE(loss,0), COALESCE(lr,0),
		COALESCE(grad_norm,0), COALESCE(tok_s,0), COALESCE(ts,'') FROM metrics WHERE run_id=? AND step>=?
		ORDER BY step, kind LIMIT ?`, runID, sinceStep, maxListedMetrics)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Metric{}
	for rows.Next() {
		var m Metric
		if err := rows.Scan(&m.Step, &m.Kind, &m.Epoch, &m.Loss, &m.LR, &m.GradNorm, &m.TokS, &m.TS); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func validArtifactPath(rel string) bool {
	return rel != "" && !strings.Contains(rel, `\`) && filepath.IsLocal(filepath.FromSlash(rel))
}

// AddArtifact registers (or, for the same ID, replaces) a file. ID defaults to ArtifactID(RunID, Kind), Created to now.
func (s *Store) AddArtifact(ctx context.Context, a Artifact) error {
	if !validArtifactKind(a.Kind) {
		return fmt.Errorf("add artifact: unknown kind %q", a.Kind)
	}
	if !ValidID(a.RunID) || !ValidID(a.ProjectID) {
		return errors.New("add artifact: invalid run or project id")
	}
	if !validArtifactPath(a.RelPath) {
		return fmt.Errorf("add artifact: invalid path %q", a.RelPath)
	}
	if a.ID == "" {
		a.ID = ArtifactID(a.RunID, a.Kind)
	}
	if a.Created == "" {
		a.Created = s.stamp()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO artifacts(id,run_id,project_id,kind,rel_path,bytes,sha256,created)
		VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET run_id=excluded.run_id, project_id=excluded.project_id,
		kind=excluded.kind, rel_path=excluded.rel_path, bytes=excluded.bytes, sha256=excluded.sha256`,
		a.ID, a.RunID, a.ProjectID, a.Kind, a.RelPath, a.Bytes, a.SHA256, a.Created)
	if err != nil {
		return fmt.Errorf("add artifact: %w", err)
	}
	return nil
}

const artifactCols = `id, COALESCE(run_id,''), COALESCE(project_id,''), COALESCE(kind,''), COALESCE(rel_path,''),
	COALESCE(bytes,0), COALESCE(sha256,''), COALESCE(created,'')`

func scanArtifact(sc rowScanner) (Artifact, error) {
	var a Artifact
	if err := sc.Scan(&a.ID, &a.RunID, &a.ProjectID, &a.Kind, &a.RelPath, &a.Bytes, &a.SHA256, &a.Created); err != nil {
		return Artifact{}, err
	}
	a.Name = path.Base(a.RelPath)
	return a, nil
}

// ListArtifacts lists the artifacts of a run in registration order.
func (s *Store) ListArtifacts(ctx context.Context, runID string) ([]Artifact, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+artifactCols+` FROM artifacts WHERE run_id=? ORDER BY created, rowid`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Artifact{}
	for rows.Next() {
		a, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetArtifact returns one artifact or ErrNotFound.
func (s *Store) GetArtifact(ctx context.Context, id string) (Artifact, error) {
	a, err := scanArtifact(s.db.QueryRowContext(ctx, `SELECT `+artifactCols+` FROM artifacts WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Artifact{}, ErrNotFound
	}
	return a, err
}

// SaveReport stores (or replaces) the report of a run; kind is auto or judge.
func (s *Store) SaveReport(ctx context.Context, runID, kind string, result any) error {
	if kind != "auto" && kind != "judge" {
		return fmt.Errorf("save report: unknown kind %q", kind)
	}
	body, err := jsonText(result)
	if err != nil {
		return fmt.Errorf("save report: %w", err)
	}
	if body == "null" {
		return errors.New("save report: empty report")
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO eval_reports(run_id,kind,result_json,created) VALUES(?,?,?,?)
		ON CONFLICT(run_id,kind) DO UPDATE SET result_json=excluded.result_json, created=excluded.created`,
		runID, kind, body, s.stamp())
	if err != nil {
		return fmt.Errorf("save report: %w", err)
	}
	return nil
}

// GetReport returns the stored report JSON and whether one exists.
func (s *Store) GetReport(ctx context.Context, runID, kind string) (json.RawMessage, bool, error) {
	var body string
	err := s.db.QueryRowContext(ctx, `SELECT result_json FROM eval_reports WHERE run_id=? AND kind=?`, runID, kind).Scan(&body)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return json.RawMessage(body), true, nil
}

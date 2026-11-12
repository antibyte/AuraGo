package flows

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/fileutil"
)

type PurgeStep string

const (
	PurgeStepBackup    PurgeStep = "backup"
	PurgeStepVersions  PurgeStep = "versions"
	PurgeStepRedact    PurgeStep = "redact"
	PurgeStepCompact   PurgeStep = "compact"
	PurgeStepSnapshots PurgeStep = "snapshots"
	PurgeStepVerify    PurgeStep = "verify"
)

var (
	ErrPurgeBusy        = errors.New("a credential cleanup is already running")
	ErrPurgeRunsActive  = errors.New("a flow run is running; try again when it has finished")
	ErrPurgeNoKey       = errors.New("the master key is not available, so the encrypted safety backup cannot be written")
	ErrPurgeNoSpace     = errors.New("there is not enough free disk space for the cleanup")
	ErrPurgeUnavailable = errors.New("the flows database is not stored in a file")
	ErrPurgeSnapshot    = errors.New("a chosen snapshot does not exist")
)

type PurgeRemaining struct {
	Where string `json:"where"`
	Count int    `json:"count"`
}

// PurgeState is what GET /credentials/purge answers. Status is idle, running, done or failed. It never
// holds a credential value.
type PurgeState struct {
	ID               string           `json:"id"`
	Status           string           `json:"status"`
	Step             PurgeStep        `json:"step,omitempty"`
	FailedStep       PurgeStep        `json:"failed_step,omitempty"`
	Error            string           `json:"error,omitempty"`
	StartedAt        time.Time        `json:"started_at"`
	FinishedAt       time.Time        `json:"finished_at"`
	Backup           string           `json:"backup,omitempty"`
	VersionsDeleted  int              `json:"versions_deleted"`
	VersionsPinned   int              `json:"versions_pinned"`
	RowsRedacted     int              `json:"rows_redacted"`
	SnapshotsDeleted []string         `json:"snapshots_deleted"`
	Clean            bool             `json:"clean"`
	Remaining        []PurgeRemaining `json:"remaining"`
}

type purgeState struct {
	mu    sync.Mutex
	state PurgeState
	// stepHook, set only by tests, runs when the job enters a step.
	stepHook func(PurgeStep)
}

func (s *Service) CredentialPurgeState() PurgeState {
	p := &s.purge
	p.mu.Lock()
	defer p.mu.Unlock()
	st := p.state
	if st.Status == "" {
		st.Status = "idle"
	}
	st.SnapshotsDeleted = append([]string{}, st.SnapshotsDeleted...)
	st.Remaining = append([]PurgeRemaining{}, st.Remaining...)
	return st
}

func (s *Service) updatePurge(change func(*PurgeState)) {
	s.purge.mu.Lock()
	change(&s.purge.state)
	s.purge.mu.Unlock()
}

// validateSnapshotNames accepts only names of existing migration snapshot directories.
func validateSnapshotNames(dir string, names []string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	for _, name := range names {
		if !snapshotNamePattern.MatchString(name) {
			return nil, ErrPurgeSnapshot
		}
		if info, err := os.Lstat(filepath.Join(dir, name)); err != nil || !info.IsDir() {
			return nil, ErrPurgeSnapshot
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out, nil
}

// StartCredentialPurge checks everything that can be checked first, holds the runner and starts the job.
// It refuses while a run is running (ErrPurgeRunsActive), without a master key, without free disk space
// and while another job runs. New runs wait in the queue until the job ends; waiting runs do not block it.
func (s *Service) StartCredentialPurge(snapshots []string) (PurgeState, error) {
	key := ""
	if s.cfg.PurgeKey != nil {
		key = strings.TrimSpace(s.cfg.PurgeKey())
	}
	if key == "" {
		return PurgeState{}, ErrPurgeNoKey
	}
	dbPath, err := s.store.dbPath()
	if err != nil {
		return PurgeState{}, err
	}
	if dbPath == "" {
		return PurgeState{}, ErrPurgeUnavailable
	}
	dir := filepath.Dir(dbPath)
	names, err := validateSnapshotNames(dir, snapshots)
	if err != nil {
		return PurgeState{}, err
	}
	var size int64
	for _, suffix := range []string{"", "-wal"} {
		if info, err := os.Stat(dbPath + suffix); err == nil {
			size += info.Size()
		}
	}
	if free, err := fileutil.FreeDiskBytes(dir); err == nil && free < 2*size+16<<20 {
		return PurgeState{}, ErrPurgeNoSpace
	}
	p := &s.purge
	p.mu.Lock()
	if p.state.Status == "running" {
		p.mu.Unlock()
		return PurgeState{}, ErrPurgeBusy
	}
	release, err := s.runner.HoldDispatch()
	if err != nil {
		p.mu.Unlock()
		if errors.Is(err, ErrRunsActive) {
			return PurgeState{}, ErrPurgeRunsActive
		}
		if errors.Is(err, ErrDispatchHeld) {
			return PurgeState{}, ErrPurgeBusy
		}
		return PurgeState{}, err
	}
	id := make([]byte, 6)
	_, _ = rand.Read(id)
	p.state = PurgeState{ID: hex.EncodeToString(id), Status: "running", Step: PurgeStepBackup, StartedAt: s.now(),
		SnapshotsDeleted: []string{}, Remaining: []PurgeRemaining{}}
	state := p.state
	p.mu.Unlock()
	go s.runPurge(dbPath, dir, key, names, release)
	return state, nil
}

func (s *Service) runPurge(dbPath, dir, key string, snapshots []string, release func()) {
	defer release()
	ctx, cancel := s.stopContext()
	defer cancel()
	step := PurgeStepBackup
	fail := func(err error) {
		s.logger.Warn("credential cleanup stopped", "step", string(step), "error", err)
		s.updatePurge(func(st *PurgeState) {
			st.Status, st.FailedStep, st.Error, st.FinishedAt = "failed", step, err.Error(), s.now()
		})
	}
	defer func() {
		if r := recover(); r != nil {
			fail(fmt.Errorf("internal error: %v", r))
		}
	}()
	enter := func(next PurgeStep) {
		step = next
		s.updatePurge(func(st *PurgeState) { st.Step = next })
		if hook := s.purge.stepHook; hook != nil {
			hook(next)
		}
	}

	enter(PurgeStepBackup)
	backup, err := s.store.writePurgeBackup(ctx, dir, key, s.now())
	if err != nil {
		fail(err)
		return
	}
	s.updatePurge(func(st *PurgeState) { st.Backup = backup })

	enter(PurgeStepVersions)
	scan, err := s.store.scanCredentials(ctx) // before anything is deleted: the moved tokens are still in the old versions
	if err != nil {
		fail(err)
		return
	}
	literals := scan.allLiterals()
	for flowID, revisions := range scan.purge {
		if err := s.store.MarkVersionsForPurge(ctx, flowID, revisions, s.now()); err != nil {
			fail(err)
			return
		}
	}
	deleted, err := s.store.PrunePurgedVersions(ctx, "")
	if err != nil {
		fail(err)
		return
	}
	var pinned int
	if err := s.store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM flow_version_purge`).Scan(&pinned); err != nil {
		fail(err)
		return
	}
	s.updatePurge(func(st *PurgeState) { st.VersionsDeleted, st.VersionsPinned = int(deleted), pinned })

	enter(PurgeStepRedact)
	conn, err := s.store.db.Conn(ctx)
	if err != nil {
		fail(err)
		return
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `PRAGMA secure_delete = ON`); err != nil {
		fail(err)
		return
	}
	defer func() { _, _ = conn.ExecContext(context.WithoutCancel(ctx), `PRAGMA secure_delete = OFF`) }()
	redacted, err := s.store.redactStoredRows(ctx, conn, literals)
	if err != nil {
		fail(err)
		return
	}
	s.updatePurge(func(st *PurgeState) { st.RowsRedacted = redacted })

	enter(PurgeStepCompact)
	if err := compactDatabase(ctx, conn); err != nil {
		fail(err)
		return
	}

	enter(PurgeStepSnapshots)
	gone, err := deleteSnapshots(dir, snapshots)
	if err != nil {
		fail(err)
		return
	}
	s.updatePurge(func(st *PurgeState) { st.SnapshotsDeleted = gone })

	enter(PurgeStepVerify)
	remaining, err := s.store.verifyPurge(ctx, dbPath, dir, literals)
	if err != nil {
		fail(err)
		return
	}
	if _, err := s.computeCredentialScan(ctx); err != nil {
		s.logger.Warn("the credential scan after the cleanup failed", "error", err)
	}
	s.updatePurge(func(st *PurgeState) {
		st.Status, st.Step, st.Remaining, st.Clean, st.FinishedAt = "done", "", remaining, len(remaining) == 0, s.now()
	})
	s.logger.Info("credential cleanup finished", slog.Int("versions_deleted", int(deleted)), slog.Int("rows_redacted", redacted), slog.Int("remaining", len(remaining)))
}

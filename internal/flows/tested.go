package flows

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"
)

// The "tested" marker. A test run or a simulation of the draft that finishes successfully marks
// that draft revision as tested; any later save creates a new revision, which is untested.
//
//   - A simulation is not stored, and retention prunes old test runs. So a successful
//     simulation, and a test run that ends with success (Service.onRunFinished), write the
//     revision they ran into flows_meta under the key testedMetaKey(flowID): the existing
//     key/value table, so there is no schema change. Only a higher revision replaces the value.
//   - Test runs are also stored with the draft revision they ran (flow_runs.revision, mode
//     "test", status "success"); IsTested reads them as well, which covers the moment between
//     the run's row being saved and the hook writing the marker, and runs from before this code.
//
// A failed test or simulation, one that was cancelled, and a simulation that did not run
// (an error from Simulate) mark nothing.

// testedMetaKey is the flows_meta key that holds the last successfully simulated draft
// revision of a flow or block.
func testedMetaKey(flowID string) string { return "tested:" + flowID }

// markTestedSQL writes the revision only when it is higher than the stored one, so a slow
// simulation of an old revision cannot erase the mark of a newer one.
const markTestedSQL = `INSERT INTO flows_meta (key, value) VALUES (?, ?)
	ON CONFLICT(key) DO UPDATE SET value = excluded.value
	WHERE CAST(flows_meta.value AS INTEGER) < CAST(excluded.value AS INTEGER)`

func (s *Store) markSimulated(ctx context.Context, flowID string, revision int) error {
	_, err := s.db.ExecContext(ctx, markTestedSQL, testedMetaKey(flowID), strconv.Itoa(revision))
	return err
}

func (s *Store) simulatedRevision(ctx context.Context, flowID string) (int, bool, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM flows_meta WHERE key = ?`, testedMetaKey(flowID)).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	revision, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, nil
	}
	return revision, true, nil
}

// hasSuccessfulTestRun reports whether a test run of exactly this draft revision finished
// with success. The query reads the flow's rows through idx_flow_runs_flow_started.
func (s *Store) hasSuccessfulTestRun(ctx context.Context, flowID string, revision int) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM flow_runs WHERE flow_id = ? AND mode = 'test' AND revision = ? AND status = 'success' LIMIT 1`,
		flowID, revision).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// markTested records a successful simulation of the draft revision. It must not fail the
// simulation: a store error is logged and the answer goes out unmarked.
func (s *Service) markTested(ctx context.Context, flowID string, revision int) {
	if revision <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := s.store.markSimulated(ctx, flowID, revision); err != nil {
		s.logger.Warn("flow simulation could not be marked as tested", "flow", truncateForError(flowID), "revision", revision, "error", err)
	}
}

// IsTested reports whether a test run or a simulation of exactly this draft revision of the
// flow (or block) finished successfully. It is read-only and takes no flow lock. A store error
// counts as "not tested".
func (s *Service) IsTested(ctx context.Context, flowID string, draftRevision int) bool {
	if draftRevision <= 0 {
		return false
	}
	if revision, ok, err := s.store.simulatedRevision(ctx, flowID); err == nil && ok && revision == draftRevision {
		return true
	}
	ok, err := s.store.hasSuccessfulTestRun(ctx, flowID, draftRevision)
	return err == nil && ok
}

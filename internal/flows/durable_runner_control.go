package flows

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

const durableLease = 30 * time.Second

// durableControl binds checkpoint writes to one run's generation and owner.
// Its callbacks run serially on the run's coordinator.
func durableControl(store *Store, keys CheckpointKeyProvider, runID, owner string, generation int64, now func() time.Time) *durableRunControl {
	control := &durableRunControl{}
	var mu sync.Mutex
	control.renew = func(ctx context.Context) error {
		mu.Lock()
		defer mu.Unlock()
		at := now()
		res, err := store.db.ExecContext(ctx, `UPDATE flow_checkpoints SET lease_until=?,updated_at=? WHERE run_id=? AND generation=? AND state='claimed' AND lease_owner=? AND lease_until>? AND deadline>? AND EXISTS (SELECT 1 FROM flow_runs WHERE id=? AND status='running' AND durable=1)`, formatTime(at.Add(durableLease)), formatTime(at), runID, generation, owner, formatTime(at), formatTime(at), runID)
		if err != nil {
			return err
		}
		return requireRow(res, errCheckpointConflict)
	}
	control.save = func(ctx context.Context, state durableCheckpointState, step StepRecord) error {
		mu.Lock()
		defer mu.Unlock()
		plain, err := encodeDurableState(state)
		if err != nil {
			return err
		}
		defer clear(plain)
		next := generation + 1
		err = withCheckpointKeyMode(ctx, store.db, keys, generation == 0, func(tx *sql.Tx, key []byte) error {
			if generation > 0 {
				if err := authenticateOwnedCheckpointTx(ctx, tx, runID, generation, owner, now(), key); err != nil {
					return err
				}
			}
			sealed, err := sealCheckpoint(key, runID, uint64(next), plain)
			if err != nil {
				return err
			}
			rec := durableCheckpointRecord{RunID: runID, Generation: next, WakeAt: state.WakeAt, Deadline: state.Deadline, Ciphertext: sealed}
			return store.saveWaitingCheckpointTx(ctx, tx, rec, generation, owner, now(), state.EventSeq, step)
		})
		if err == nil {
			generation = next
			control.persistedSeq = state.EventSeq
		}
		return err
	}
	control.checkpoint = func(ctx context.Context, state durableCheckpointState, step StepRecord) error {
		mu.Lock()
		defer mu.Unlock()
		plain, err := encodeDurableState(state)
		if err != nil {
			return err
		}
		defer clear(plain)
		next := generation + 1
		err = withCheckpointKeyMode(ctx, store.db, keys, false, func(tx *sql.Tx, key []byte) error {
			if err := authenticateOwnedCheckpointTx(ctx, tx, runID, generation, owner, now(), key); err != nil {
				return err
			}
			sealed, err := sealCheckpoint(key, runID, uint64(next), plain)
			if err != nil {
				return err
			}
			at := now()
			rec := durableCheckpointRecord{RunID: runID, Generation: next, Deadline: state.Deadline, Ciphertext: sealed}
			return store.saveActiveCheckpointTx(ctx, tx, rec, generation, owner, at, at.Add(durableLease), state.EventSeq, step)
		})
		if err == nil {
			generation = next
			control.persistedSeq = state.EventSeq
		}
		return err
	}
	control.finish = func(ctx context.Context, result RunResult, events []RunEvent) error {
		mu.Lock()
		defer mu.Unlock()
		err := withCheckpointKeyMode(ctx, store.db, keys, false, func(tx *sql.Tx, key []byte) error {
			if err := authenticateOwnedCheckpointTx(ctx, tx, runID, generation, owner, now(), key); err != nil {
				return err
			}
			for _, event := range events {
				if event.Step == nil {
					continue
				}
				if err := store.saveStepOn(ctx, tx, runID, event.Seq, *event.Step, false); err != nil {
					return fmt.Errorf("save terminal step: %w", err)
				}
			}
			return store.finishDurableCheckpointTx(ctx, tx, runID, generation, owner, now(), result)
		})
		if err != nil {
			return err
		}
		if len(events) > 0 {
			control.persistedSeq = events[len(events)-1].Seq
		}
		return nil
	}
	return control
}

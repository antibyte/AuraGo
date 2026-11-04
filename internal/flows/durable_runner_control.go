package flows

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const durableLease = 30 * time.Second

// durableControl binds checkpoint writes to one run's generation and owner.
// Its callbacks run serially on the run's coordinator.
func durableControl(store *Store, keys CheckpointKeyProvider, runID, owner string, generation int64, now func() time.Time) *durableRunControl {
	control := &durableRunControl{}
	control.save = func(ctx context.Context, state durableCheckpointState, step StepRecord) error {
		plain, err := encodeDurableState(state)
		if err != nil {
			return err
		}
		defer clear(plain)
		next := generation + 1
		err = withCheckpointKey(ctx, store.db, keys, func(tx *sql.Tx, key []byte) error {
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
		plain, err := encodeDurableState(state)
		if err != nil {
			return err
		}
		defer clear(plain)
		next := generation + 1
		err = withCheckpointKey(ctx, store.db, keys, func(tx *sql.Tx, key []byte) error {
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
		tx, err := store.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		for _, event := range events {
			if event.Step == nil {
				continue
			}
			if err := store.saveStepOn(ctx, tx, runID, event.Seq, *event.Step, false); err != nil {
				return fmt.Errorf("save terminal step: %w", err)
			}
		}
		if err := store.finishDurableCheckpointTx(ctx, tx, runID, generation, owner, now(), result); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		if len(events) > 0 {
			control.persistedSeq = events[len(events)-1].Seq
		}
		return nil
	}
	return control
}

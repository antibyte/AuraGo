package flows

import (
	"context"
	"time"
)

// The SQLite journal remains authoritative. Reconciliation never resumes work
// or invokes the ordinary mission completion chain, even when processing is off.
func (s *Service) startDurableHistoryReconcile() {
	r := s.runner
	if r.cfg.DurableHistory == nil {
		return
	}
	r.mu.Lock()
	if r.closed || r.durableFaulted {
		r.mu.Unlock()
		return
	}
	r.wg.Add(1)
	r.mu.Unlock()
	go func() {
		defer r.wg.Done()
		defer func() {
			if recover() != nil {
				r.faultDurableRuntime("history_reconcile")
			}
		}()
		cursor := ""
		for {
			if r.durableRuntimeFaulted() || r.durableRuntimeCtx.Err() != nil {
				return
			}
			portion, cancel := context.WithTimeout(r.durableRuntimeCtx, 3*time.Second)
			previous := cursor
			page, err := s.ListDurableProjections(portion, cursor, 100)
			complete := false
			if err == nil {
				for _, entry := range page.Items {
					if portion.Err() != nil {
						break
					}
					if err := s.projectDurableHistory(portion, entry.RunID); err != nil && portion.Err() == nil {
						r.logger.Warn("durable history projection unavailable", "run", entry.RunID)
					}
					if portion.Err() != nil {
						break
					}
					cursor = entry.RunID
				}
				complete = portion.Err() == nil && page.NextCursor == ""
			}
			interrupted := portion.Err() != nil
			cancel()
			wait := 20 * time.Millisecond
			if err != nil || (interrupted && previous == cursor) {
				wait = 30 * time.Second
			}
			if complete {
				cursor = ""
				wait = 5 * time.Minute
			}
			select {
			case <-r.durableRuntimeCtx.Done():
				return
			case <-s.services.clock().After(wait):
			}
		}
	}()
}

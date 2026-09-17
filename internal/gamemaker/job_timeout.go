package gamemaker

import (
	"context"
	"fmt"
	"time"
)

// The configured budget remains the limit for stalled jobs. Productive jobs
// receive one finishing window, never a sliding or repeatedly renewed deadline.
func jobTimeoutGrace(budget time.Duration) time.Duration {
	return min(budget/2, 15*time.Minute)
}

func (s *Service) jobContext(job Job, project Project) (context.Context, context.CancelFunc) {
	budget := s.opts.JobTimeout
	return newGameJobContext(budget, func(ctx context.Context) bool {
		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if !s.recentCompiledProgress(checkCtx, job.ID, project.Dimension, time.Now().Add(-min(budget/3, 10*time.Minute))) {
			return false
		}
		grace := jobTimeoutGrace(budget)
		_, _ = s.emit(checkCtx, job.ProjectID, job.ID, "diagnostic", map[string]any{
			"level": "info", "code": "job_time_extended",
			"message":       fmt.Sprintf("Recent source changes compiled successfully. Game creation has one additional %s to finish; validation remains required.", grace),
			"extra_seconds": grace.Seconds(), "total_seconds": (budget + grace).Seconds(),
		})
		return true
	})
}

func newGameJobContext(budget time.Duration, extend func(context.Context) bool) (context.Context, context.CancelFunc) {
	baseDeadline := time.Now().Add(budget)
	lifetime, cancelCause := context.WithCancelCause(context.Background())
	ctx, cancelDeadline := context.WithDeadline(lifetime, baseDeadline.Add(jobTimeoutGrace(budget)))
	go func() {
		timer := time.NewTimer(time.Until(baseDeadline))
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if ctx.Err() != nil {
				return
			}
			if !extend(ctx) {
				cancelCause(context.DeadlineExceeded)
				cancelDeadline()
			}
		}
	}()
	return ctx, func() {
		cancelCause(context.Canceled)
		cancelDeadline()
	}
}

func (s *Service) recentCompiledProgress(ctx context.Context, jobID, dimension string, since time.Time) bool {
	s.mu.RLock()
	// buildJob clears this on every build, and installs it only after success.
	compiled := s.previewCheck != nil && s.previewCheck.JobID == jobID
	accepted := s.acceptedPlans[jobID]
	s.mu.RUnlock()
	if !accepted || !compiled {
		return false
	}
	var changedAt time.Time
	if err := s.db.QueryRowContext(ctx, `SELECT created_at FROM gm_events
		WHERE job_id=? AND event_type='file_changed' ORDER BY id DESC LIMIT 1`, jobID).Scan(&changedAt); err != nil || changedAt.Before(since) {
		return false
	}
	unchanged, err := s.unchangedGameStarter(ctx, jobID, dimension)
	return err == nil && !unchanged && ctx.Err() == nil
}

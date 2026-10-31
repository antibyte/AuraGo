package kaggle

import (
	"context"
	"errors"
	"time"
)

var ErrJobStateConflict = errors.New("only an unknown job can be manually resolved")

// ResolveUnknown records an administrator's external check, not a remote outcome.
// Authorization and explicit confirmation belong to the HTTP owner, never a tool.
func (s *Store) ResolveUnknown(ctx context.Context, id string) (Job, error) {
	if !validID(id) {
		return Job{}, errors.New("invalid job ID")
	}
	result, err := s.db.ExecContext(ctx, `UPDATE jobs SET status='resolved_unknown',updated=? WHERE id=? AND status='unknown'`, time.Now().UTC().Format(time.RFC3339Nano), id)
	if err != nil {
		return Job{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return Job{}, err
	}
	job, err := s.Get(ctx, id)
	if err != nil {
		return Job{}, err
	}
	if job.Status != "resolved_unknown" {
		return Job{}, ErrJobStateConflict
	}
	if changed == 1 {
		_, err = s.Emit(ctx, "job", job)
	}
	return job, err
}

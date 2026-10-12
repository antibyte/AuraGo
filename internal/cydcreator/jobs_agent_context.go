package cydcreator

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// maxAgentContextBytes caps one private conversation checkpoint.
const maxAgentContextBytes = 16 << 20

// SaveAgentContext stores the private, provider-native conversation of the
// running job (cc_agent_context) so a resumed job can continue it. It is
// never shown in the chat, exported or used as general memory.
func (s *Service) SaveAgentContext(ctx context.Context, jobID string, data []byte) error {
	if len(data) > maxAgentContextBytes || !json.Valid(data) {
		return NewError(CodeBadRequest, "invalid or oversized agent conversation", nil)
	}
	run := s.activeRun()
	if run == nil || run.jobID != jobID {
		return NewError(CodeJobState, "this job is not running", map[string]any{"job_id": jobID})
	}
	if _, err := s.db.ExecContext(ctx, `INSERT INTO cc_agent_context(job_id,project_id,data,updated_at) VALUES(?,?,?,?)
		ON CONFLICT(job_id) DO UPDATE SET data=excluded.data, updated_at=excluded.updated_at`,
		jobID, run.projectID, data, formatTime(s.now())); err != nil {
		return s.internalError("save agent conversation", err)
	}
	return nil
}

// LoadAgentContext returns the job's conversation, else the conversation of
// the draft it resumes, else nil.
func (s *Service) LoadAgentContext(ctx context.Context, jobID string) ([]byte, error) {
	job, err := s.loadJob(ctx, jobID)
	if err != nil {
		return nil, s.internalError("load agent conversation", err)
	}
	for _, id := range []string{job.ID, job.ResumeOf} {
		if id == "" {
			continue
		}
		var data []byte
		err := s.db.QueryRowContext(ctx, `SELECT data FROM cc_agent_context WHERE job_id=? AND project_id=?`, id, job.ProjectID).Scan(&data)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, s.internalError("load agent conversation", err)
		}
		return data, nil
	}
	return nil, nil
}

// projectJobIDs lists the ids of every job of a project.
func (s *Service) projectJobIDs(ctx context.Context, projectID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM cc_jobs WHERE project_id=?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

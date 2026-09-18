package gamemaker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AgentConversation is private execution state, never a Studio message, asset,
// project file or export. The server owns the provider-native message format.
type AgentConversation struct {
	JobID      string
	ProviderID string
	Model      string
	Messages   json.RawMessage
}

func (s *Service) SaveAgentConversation(ctx context.Context, jobID, providerID, model string, messages json.RawMessage) error {
	if len(messages) > 16*1024*1024 || !json.Valid(messages) {
		return fmt.Errorf("invalid or oversized agent continuation")
	}
	s.mu.RLock()
	active := s.activeJobID == jobID
	s.mu.RUnlock()
	if !active {
		return ErrBusy
	}
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO gm_agent_context(project_id,job_id,provider_id,model,messages,updated_at)
		VALUES(?,?,?,?,?,?) ON CONFLICT(project_id) DO UPDATE SET job_id=excluded.job_id,
		provider_id=excluded.provider_id,model=excluded.model,messages=excluded.messages,updated_at=excluded.updated_at`,
		job.ProjectID, job.ID, providerID, model, []byte(messages), time.Now().UTC())
	if err != nil {
		return fmt.Errorf("save game maker conversation: %w", err)
	}
	return nil
}

func (s *Service) LoadAgentConversation(ctx context.Context, jobID string) (AgentConversation, error) {
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return AgentConversation{}, err
	}
	var c AgentConversation
	// A revision restore must not resurrect a conversation about discarded work.
	err = s.db.QueryRowContext(ctx, `SELECT c.job_id,c.provider_id,c.model,c.messages
		FROM gm_agent_context c JOIN gm_jobs j ON j.id=c.job_id
		WHERE c.project_id=? AND (j.id=? OR
		(j.status='ready' AND j.result_revision=?) OR (j.status!='ready' AND j.base_revision=?))`,
		job.ProjectID, jobID, job.BaseRevision, job.BaseRevision).Scan(&c.JobID, &c.ProviderID, &c.Model, &c.Messages)
	if errors.Is(err, sql.ErrNoRows) {
		return AgentConversation{}, nil
	}
	if err != nil {
		return AgentConversation{}, fmt.Errorf("load game maker conversation: %w", err)
	}
	return c, nil
}

// User requests are independently retained even for projects predating private
// checkpoints, or when route-aware history fitting drops old tool rounds.
func (s *Service) PreviousJobRequests(ctx context.Context, jobID string) ([]string, error) {
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT prompt FROM (
		SELECT prompt,created_at FROM gm_jobs WHERE project_id=? AND created_at<?
		ORDER BY created_at DESC LIMIT 16) ORDER BY created_at`, job.ProjectID, job.CreatedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var requests []string
	for rows.Next() {
		var prompt string
		if err := rows.Scan(&prompt); err != nil {
			return nil, err
		}
		requests = append(requests, prompt)
	}
	return requests, rows.Err()
}

func isContinuationPrompt(prompt string) bool {
	switch strings.ToLower(strings.Trim(strings.TrimSpace(prompt), ".!")) {
	case "continue", "retry", "try again", "resume", "weiter", "weiter machen", "weitermachen", "fortsetzen", "erneut versuchen", "versuche es erneut":
		return true
	}
	return false
}

func (s *Service) resumableJob(ctx context.Context, project Project) (string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,status,base_revision FROM gm_jobs WHERE project_id=? ORDER BY created_at DESC`, project.ID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var id, status string
		var base int64
		if err := rows.Scan(&id, &status, &base); err != nil {
			return "", err
		}
		if base != project.CurrentRevision || (status != "failed" && status != "cancelled" && status != "interrupted") {
			return "", nil
		}
		path := filepath.Join(s.stagingDir, id)
		if err := rejectSymlinkComponents(s.stagingDir, path); err != nil {
			return "", err
		}
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue // A failed copy must not hide the previous recoverable draft.
		} else if err != nil {
			return "", err
		}
		return id, nil
	}
	return "", rows.Err()
}

// Keep only the latest failed working copy per project. It remains private;
// published revisions and validation gates are independent of this checkpoint.
// The caller supplies the final status and retains the writer until cleanup ends.
func (s *Service) finishWorkingCopy(job Job) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if job.Status != "ready" {
		if _, err := os.Stat(filepath.Join(s.stagingDir, job.ID)); err != nil {
			return // Initialization failed: keep the previous working copy intact.
		}
	}
	ids, err := s.projectWorkingCopyIDs(ctx, job.ProjectID)
	if err != nil {
		return
	}
	for _, id := range ids {
		if id != job.ID || job.Status == "ready" {
			s.removeWorkingCopy(id)
		}
	}
}

func (s *Service) projectWorkingCopyIDs(ctx context.Context, projectID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM gm_jobs WHERE project_id=?`, projectID)
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

func (s *Service) removeWorkingCopy(id string) {
	if filepath.Base(id) != id || id == "." || id == ".." {
		return
	}
	path := filepath.Join(s.stagingDir, id)
	if err := rejectSymlinkComponents(s.stagingDir, path); err == nil {
		if err := os.RemoveAll(path); err != nil && s.opts.Logger != nil {
			s.opts.Logger.Warn("Failed to remove old Game Maker working copy", "job_id", id)
		}
	}
}

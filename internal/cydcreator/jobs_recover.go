package cydcreator

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// unfinishedJobIDs lists queued and running jobs before recoverStore marks
// them interrupted, so recoverJobs can announce them and keep their drafts.
func unfinishedJobIDs(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT id FROM cc_jobs WHERE status IN ('queued','running') ORDER BY created_at, id`)
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

// recoverJobs runs once in NewService after recoverStore, before the build
// worker starts and before any job can start: it finishes or rolls back
// interrupted publications, turns the working copies of interrupted jobs into
// drafts, announces those jobs, removes orphaned working copies and prunes
// unreferenced revision blobs.
//
// It never fails the start. A project whose state it cannot settle is fenced
// (every write CYD_BUSY, reason publication_review, until a restart resolves
// it) and keeps its working copies; when the receipts cannot be listed at all
// every project is fenced. Working copies are removed only when every
// remaining receipt and the job rows could be read, and never the stage of a
// receipt left for review.
func (s *Service) recoverJobs(ctx context.Context, interrupted []string) {
	s.recoverReceipts(ctx)
	keepStage, listed, complete := s.unresolvedReceiptStages()
	if !listed {
		s.fenceAllProjects(ctx)
	}
	for _, id := range interrupted {
		job, err := s.loadJob(ctx, id)
		if err != nil {
			if ErrorCode(err) != CodeJobNotFound {
				keepStage[id] = true
				s.logger.Error("CYD Creator interrupted job could not be read; its working copy is kept", "job_id", id, "error", err)
			}
			continue
		}
		if job.Status != JobInterrupted {
			continue
		}
		row, err := s.loadProjectRow(ctx, job.ProjectID)
		if err != nil {
			if ErrorCode(err) != CodeNotFound {
				keepStage[job.ID] = true
				s.fenceForRecovery(job, err)
			}
			continue
		}
		stage := s.stagingPath(job.ID)
		keep := false
		if dirExists(stage) {
			staged, err1 := treeFingerprint(stage)
			current, err2 := treeFingerprint(s.projectDir(row.Slug))
			keep = err1 != nil || err2 != nil || staged != current
		}
		draft := false
		switch {
		case keep:
			// Record the new draft before the older one is removed, so a
			// failed write loses neither.
			if _, err := s.db.ExecContext(ctx, `UPDATE cc_projects SET draft_job=? WHERE id=?`, job.ID, row.ID); err != nil {
				keepStage[job.ID] = true
				s.fenceForRecovery(job, err)
				break
			}
			draft = true
			if row.DraftJob != "" && row.DraftJob != job.ID {
				s.removeStaging(row.DraftJob)
				_, _ = s.db.ExecContext(ctx, `DELETE FROM cc_agent_context WHERE job_id=?`, row.DraftJob)
			}
		default:
			// No draft: the working copy goes with the sweep, and so does the
			// private conversation (finishJob does the same).
			_, _ = s.db.ExecContext(ctx, `DELETE FROM cc_agent_context WHERE job_id=?`, job.ID)
		}
		s.Emit(job.ProjectID, EventLockChanged, job.ID, "", map[string]any{"locked_by_job": ""})
		s.Emit(job.ProjectID, EventJobStatus, job.ID, "", map[string]any{"job": job})
		text := "The server restarted while the job was running. The project is unchanged."
		if s.publicationFence(row.ID) != "" {
			text = "The server restarted while the job was running. The project needs a review before it can be changed again; see the server log."
		}
		if draft {
			text += " The agent's work is kept as a draft you can resume or discard."
		}
		s.postMessage(job.ProjectID, job.ID, MessageRoleSystem, text)
	}
	if listed && complete {
		s.sweepWorkingCopies(ctx, keepStage)
	} else {
		s.logger.Warn("CYD Creator working copies were not swept: the receipts could not all be read")
	}
	if rows, err := s.db.QueryContext(ctx, `SELECT id, draft_job FROM cc_projects WHERE draft_job<>''`); err == nil {
		var stale []string
		for rows.Next() {
			var projectID, draft string
			if rows.Scan(&projectID, &draft) == nil {
				// Only a working copy that is certainly gone clears the draft.
				info, err := os.Stat(s.stagingPath(draft))
				if errors.Is(err, fs.ErrNotExist) || (err == nil && !info.IsDir()) {
					stale = append(stale, projectID)
				}
			}
		}
		_ = rows.Close()
		for _, projectID := range stale {
			_, _ = s.db.ExecContext(ctx, `UPDATE cc_projects SET draft_job='' WHERE id=?`, projectID)
		}
	}
	if err := s.pruneOrphanBlobs(ctx); err != nil {
		s.logger.Warn("CYD Creator revision blobs could not be pruned", "error", err)
	}
}

// fenceForRecovery fences the project of an interrupted job whose state could
// not be recorded at startup.
func (s *Service) fenceForRecovery(job Job, err error) {
	s.setPublicationFence(job.ProjectID, "recover_"+job.ID)
	s.logger.Error("CYD Creator could not recover an interrupted job; its project stays read-only until a restart",
		"job_id", job.ID, "project_id", job.ProjectID, "error", err)
}

// fenceAllProjects fences every project: the receipts could not be listed
// (or there are too many), so recoverReceipts resolved none and any project
// may be in the middle of a directory swap.
func (s *Service) fenceAllProjects(ctx context.Context) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM cc_projects`)
	if err != nil {
		s.logger.Error("CYD Creator receipts could not be checked and the projects could not be fenced", "error", err)
		return
	}
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	_ = rows.Close()
	for _, id := range ids {
		s.setPublicationFence(id, "receipts_unchecked")
	}
	s.logger.Error("CYD Creator receipts could not be checked; every project stays read-only until a restart resolves them",
		"projects", len(ids))
}

// unresolvedReceiptStages reads the receipts recoverReceipts left for review
// and returns their stage folders, which the working copy sweep must keep.
// listed is false when the receipts folder cannot be read or holds more than
// maxReceipts entries (recoverReceipts then resolved nothing); complete is
// false when a leftover entry names no valid stage.
func (s *Service) unresolvedReceiptStages() (stages map[string]bool, listed, complete bool) {
	stages = map[string]bool{}
	entries, err := os.ReadDir(s.receiptsDir())
	if errors.Is(err, fs.ErrNotExist) {
		return stages, true, true
	}
	if err != nil || len(entries) > maxReceipts {
		return stages, false, false
	}
	complete = true
	for _, e := range entries {
		r, err := readReceiptFile(filepath.Join(s.receiptsDir(), e.Name()))
		if err != nil || !stagePattern.MatchString(r.Stage) {
			complete = false
			continue
		}
		stages[r.Stage] = true
	}
	return stages, true, complete
}

// readReceiptFile parses a receipt file with the limits recoverReceipt uses.
func readReceiptFile(p string) (publishReceipt, error) {
	info, err := os.Lstat(p)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16<<10 || !strings.HasSuffix(p, ".json") {
		return publishReceipt{}, errors.New("unrecognized receipt file")
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return publishReceipt{}, err
	}
	var r publishReceipt
	if err := json.Unmarshal(data, &r); err != nil {
		return publishReceipt{}, err
	}
	return r, nil
}

// sweepWorkingCopies removes the working copies nothing needs any more: only
// drafts, jobs awaiting confirmation and the names in keep (stages of
// unresolved receipts, working copies recoverJobs could not settle) stay. It
// removes nothing when the staging folder is a link or the job rows cannot be
// read.
func (s *Service) sweepWorkingCopies(ctx context.Context, keep map[string]bool) {
	if err := rejectLinkedComponents(s.workspaceDir, stagingDirName); err != nil {
		s.logger.Error("CYD Creator working copies were not swept", "error", err)
		return
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM cc_jobs WHERE status='awaiting_confirmation'
		UNION SELECT draft_job FROM cc_projects WHERE draft_job<>''`)
	if err != nil {
		s.logger.Error("CYD Creator working copies were not swept", "error", err)
		return
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			s.logger.Error("CYD Creator working copies were not swept", "error", err)
			return
		}
		keep[id] = true
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		s.logger.Error("CYD Creator working copies were not swept", "error", err)
		return
	}
	entries, err := os.ReadDir(s.stagingRoot())
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			s.logger.Error("CYD Creator working copies were not swept", "error", err)
		}
		return
	}
	for _, e := range entries {
		if keep[e.Name()] {
			continue
		}
		if err := os.RemoveAll(filepath.Join(s.stagingRoot(), e.Name())); err != nil {
			s.logger.Warn("CYD Creator orphaned working copy could not be removed", "name", e.Name(), "error", err)
		}
	}
}

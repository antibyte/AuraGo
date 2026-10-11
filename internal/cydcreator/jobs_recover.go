package cydcreator

import (
	"context"
	"database/sql"
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
// interrupted publications (and completes a committed job publication whose
// cleanup the crash cut short), turns the working copies of interrupted jobs
// into drafts, announces those jobs, removes orphaned working copies and
// prunes unreferenced revision blobs.
//
// It never fails the start. A project whose state it cannot settle is fenced
// (every write CYD_BUSY, reason publication_review, until the next start) and
// keeps its working copies; an interrupted job whose draft could not be
// recorded is settled again at the next start. When the receipts cannot all
// be checked every project is fenced. Working copies are removed only when
// every receipt and the job rows could be read, and never the stage of a
// receipt left for review.
func (s *Service) recoverJobs(ctx context.Context, interrupted []string) {
	rec := s.recoverReceipts(ctx)
	if !rec.Listed || !rec.Complete {
		s.fenceAllProjects(ctx)
	}
	for _, r := range rec.Committed {
		s.finishCommittedPublication(ctx, r)
	}
	keepStage := rec.Stages
	// Interrupted jobs of earlier starts whose draft write failed come back
	// here (they are the newest job of their project, not its draft, and
	// their working copy still exists); only the jobs of this start are
	// announced.
	unsettled, err := s.unsettledInterruptedJobs(ctx)
	sweep := rec.Listed && rec.Complete
	if err != nil {
		sweep = false
		s.logger.Error("CYD Creator interrupted jobs of earlier starts could not be listed; working copies are kept", "error", err)
	}
	announced := map[string]bool{}
	for _, id := range interrupted {
		announced[id] = true
		s.settleInterruptedJob(ctx, id, true, keepStage)
	}
	for _, id := range unsettled {
		if !announced[id] {
			s.settleInterruptedJob(ctx, id, false, keepStage)
		}
	}
	if sweep {
		s.sweepWorkingCopies(ctx, keepStage)
	} else {
		s.logger.Warn("CYD Creator working copies were not swept: the receipts or jobs could not all be read")
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

// settleInterruptedJob turns the working copy of an interrupted job into the
// project's draft when it holds work (interruptedWork), replacing an older
// draft only after the new one is recorded. announce sends job_status,
// lock_changed and the chat line (jobs interrupted by this start). A step
// that fails keeps the working copy (keepStage) and fences the project; the
// next start settles the job again.
func (s *Service) settleInterruptedJob(ctx context.Context, id string, announce bool, keepStage map[string]bool) {
	job, err := s.loadJob(ctx, id)
	if err != nil {
		if ErrorCode(err) != CodeJobNotFound {
			keepStage[id] = true
			s.logger.Error("CYD Creator interrupted job could not be read; its working copy is kept and the next start tries again",
				"job_id", id, "error", err)
		}
		return
	}
	if job.Status != JobInterrupted {
		return
	}
	row, err := s.loadProjectRow(ctx, job.ProjectID)
	if err != nil {
		if ErrorCode(err) != CodeNotFound {
			keepStage[job.ID] = true
			s.fenceForRecovery(job, err)
		}
		return
	}
	keep, err := s.interruptedWork(ctx, row, job)
	if err != nil {
		keepStage[job.ID] = true
		s.fenceForRecovery(job, err)
	}
	draft := false
	switch {
	case keep:
		// Record the new draft before the older one is removed, so a failed
		// write loses neither.
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
	case s.publicationFence(row.ID) == "":
		// No draft: the working copy goes with the sweep, and so does the
		// private conversation (finishJob does the same). A fenced project
		// keeps everything for the review.
		_, _ = s.db.ExecContext(ctx, `DELETE FROM cc_agent_context WHERE job_id=?`, job.ID)
	}
	if !announce {
		if draft {
			s.postMessage(job.ProjectID, job.ID, MessageRoleSystem, "The agent's work of the interrupted job is now kept as a draft you can resume or discard.")
		}
		return
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

// interruptedWork reports whether the working copy of an interrupted job is
// worth a draft: the job got past its preparation (not in phase queued, and
// its "before_job" revision exists, which a resume needs) and the working
// copy differs from the project. A job that crashed while it was being
// prepared holds at most a partial copy of the project or of the draft it
// resumes, so the older draft stays.
func (s *Service) interruptedWork(ctx context.Context, row projectRow, job Job) (bool, error) {
	stage := s.stagingPath(job.ID)
	if job.Phase == PhaseQueued || !dirExists(stage) {
		return false, nil
	}
	var one int
	err := s.db.QueryRowContext(ctx, `SELECT 1 FROM cc_revisions WHERE job_id=? AND reason=? LIMIT 1`, job.ID, RevisionBeforeJob).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	staged, err1 := treeFingerprint(stage)
	current, err2 := treeFingerprint(s.projectDir(row.Slug))
	return err1 != nil || err2 != nil || staged != current, nil
}

// unsettledInterruptedJobs lists interrupted jobs that are the newest job of
// their project (by insertion: rowid, not the clock), are not its draft, were
// not discarded (DiscardDraft clears staging_dir) and still have a working
// copy: a draft write that failed at an earlier start. While such a project
// was fenced no newer job could start, so the job is still the newest one.
func (s *Service) unsettledInterruptedJobs(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT j.id FROM cc_jobs j WHERE j.status='interrupted' AND j.staging_dir<>''
		AND j.rowid = (SELECT j2.rowid FROM cc_jobs j2 WHERE j2.project_id=j.project_id ORDER BY j2.rowid DESC LIMIT 1)
		AND j.id NOT IN (SELECT draft_job FROM cc_projects WHERE draft_job<>'')
		ORDER BY j.rowid`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	out := ids[:0]
	for _, id := range ids {
		if validElementID(id) && dirExists(s.stagingPath(id)) {
			out = append(out, id)
		}
	}
	return out, nil
}

// finishCommittedPublication completes what a job's publication does after
// its commit when a crash cut it short (recoverReceipts still found the
// receipt): the "job" revision, the end of the private conversations and the
// events. The commit itself stored the job as succeeded and released the
// lock. A crash after the receipt was removed leaves no trace to act on; that
// job simply has no "job" revision (like a revision write that fails). Only
// a project still at the receipt's new version is completed: a receipt that
// survived while the server kept running (its removal and the revision write
// both failed) meets a later project state, whose revision would be wrong and
// whose later conversations (a newer draft's) must stay; it is only logged
// (recoverReceipt has removed it as committed).
func (s *Service) finishCommittedPublication(ctx context.Context, r publishReceipt) {
	if r.Kind != receiptPublish {
		return
	}
	job, err := s.loadJob(ctx, strings.TrimPrefix(r.ID, receiptPublish+"_"))
	if err != nil || job.Status != JobSucceeded || job.ProjectID != r.ProjectID {
		return
	}
	row, err := s.loadProjectRow(ctx, r.ProjectID)
	if err != nil {
		s.logger.Warn("CYD Creator published job could not be completed after a restart", "job_id", job.ID, "error", err)
		return
	}
	if row.Version != r.NewVersion {
		s.logger.Warn("CYD Creator published job is not completed after a restart: the project changed since its publication",
			"job_id", job.ID, "project_id", row.ID, "published_version", r.NewVersion, "version", row.Version)
		return
	}
	var one int
	err = s.db.QueryRowContext(ctx, `SELECT 1 FROM cc_revisions WHERE job_id=? AND reason=? LIMIT 1`, job.ID, RevisionJob).Scan(&one)
	if err == nil {
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		s.logger.Warn("CYD Creator job revision could not be checked after a restart", "job_id", job.ID, "error", err)
		return
	}
	if root, err := s.openRoot(row, ""); err == nil {
		if _, err := s.createRevision(ctx, row.ID, root, job.Prompt, RevisionJob, job.ID, job.BuildID); err != nil {
			s.logger.Warn("CYD Creator job revision could not be stored", "job_id", job.ID, "error", err)
		}
		_ = root.Close()
	} else {
		s.logger.Warn("CYD Creator job revision could not be stored", "job_id", job.ID, "error", err)
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM cc_agent_context WHERE project_id=?`, row.ID)
	origin := JobOrigin(job.ID)
	s.Emit(row.ID, EventProjectChanged, job.ID, "", projectChange(row.Version, origin, "layout"))
	s.Emit(row.ID, EventProjectChanged, job.ID, "", projectChange(row.Version, origin, "files"))
	s.Emit(row.ID, EventLockChanged, job.ID, "", map[string]any{"locked_by_job": ""})
	s.Emit(row.ID, EventJobStatus, job.ID, "", map[string]any{"job": job})
	s.postMessage(row.ID, job.ID, MessageRoleSystem, "The app was built and published just before the server restarted.")
}

// fenceForRecovery fences the project of an interrupted job whose state could
// not be settled at startup; the next start settles the job again.
func (s *Service) fenceForRecovery(job Job, err error) {
	s.setPublicationFence(job.ProjectID, "recover_"+job.ID)
	s.logger.Error("CYD Creator could not settle an interrupted job; its project is read-only and its working copy kept until the next start retries",
		"job_id", job.ID, "project_id", job.ProjectID, "error", err)
}

// fenceAllProjects fences every project: the receipts could not be listed,
// there were too many, or one names no project, so any project may be in the
// middle of a directory swap.
func (s *Service) fenceAllProjects(ctx context.Context) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM cc_projects`)
	if err != nil {
		s.logger.Error("CYD Creator receipts could not all be checked and the projects could not be fenced", "error", err)
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
	s.logger.Error("CYD Creator receipts could not all be checked; every project is read-only until a restart resolves them (see internal/cydcreator/AGENTS.md)",
		"projects", len(ids))
}

// sweepWorkingCopies removes the working copies nothing needs any more: only
// drafts, jobs awaiting confirmation and the names in keep (stages of
// unresolved receipts, working copies recoverJobs could not settle) stay. It
// removes nothing when the staging folder is a link or the job rows cannot be
// read.
func (s *Service) sweepWorkingCopies(ctx context.Context, keep map[string]bool) {
	if err := s.checkStagingRoot(); err != nil {
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

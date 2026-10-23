package gamemaker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

const adoptPublicationSummary = "Checked workspace edits and published them."

// JobError carries a stable code. The message stays technical English.
type JobError struct {
	Code string
	Err  error
}

func (e *JobError) Error() string {
	if e == nil || e.Err == nil {
		return "game maker job failed"
	}
	return e.Err.Error()
}

func (e *JobError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func jobErrorCode(failure error) string {
	var coded *JobError
	if errors.As(failure, &coded) && coded != nil && strings.TrimSpace(coded.Code) != "" {
		return coded.Code
	}
	return "internal"
}

func validationFailureCode(result BuildResult) string {
	if result.RuntimeStatus == "unavailable" || result.GameplayStatus == "unavailable" {
		return "preview"
	}
	return "validation"
}

func checkCounts(checks []CheckResult) (*int, *int) {
	passed, failed := 0, 0
	for _, check := range checks {
		switch check.Status {
		case "passed":
			passed++
		case "failed":
			failed++
		}
	}
	return &passed, &failed
}

func intPtr(value int) *int { return &value }

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func rejectAdoptRequest(req StartJobRequest, project Project) error {
	if project.CurrentRevision <= 0 {
		return fmt.Errorf("workspace adoption requires a published revision")
	}
	if req.Resume || strings.TrimSpace(req.Prompt) != "" || len(req.AssetSelections) > 0 || len(req.AssetPackIDs) > 0 || len(req.ModelAssetIDs) > 0 || req.Presentation != nil || req.ImageGeneration != nil || req.MusicGeneration != nil || len(req.PreviewDiagnostics) > 0 {
		return fmt.Errorf("workspace adoption cannot include a prompt, resume, assets, or generation options")
	}
	return nil
}

func (s *Service) planningDrift(ctx context.Context, job Job, project Project) *WorkspaceDrift {
	if job.Kind == "adopt" || job.ResumeFrom != "" || job.BaseRevision <= 0 {
		return nil
	}
	drift, err := s.WorkspaceDrift(ctx, project)
	if err != nil || drift.Empty() {
		return nil
	}
	return &drift
}

func planSummaryPayload(plan *GamePlan) map[string]any {
	payload := map[string]any{
		"template":    "",
		"objective":   "",
		"perspective": "",
		"controls":    []string{},
		"assets":      []map[string]string{},
	}
	if plan == nil {
		return payload
	}
	objective := plan.Objective
	runes := []rune(objective)
	if len(runes) > 240 {
		objective = string(runes[:240])
	}
	keys := make([]string, 0, len(plan.Controls))
	for key := range plan.Controls {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) > 8 {
		keys = keys[:8]
	}
	assets := make([]map[string]string, 0, min(12, len(plan.Assets)))
	for i, asset := range plan.Assets {
		if i >= 12 {
			break
		}
		id := asset.AssetID
		if id == "" {
			id = asset.AssemblyID
		}
		assets = append(assets, map[string]string{"role": asset.Role, "id": id})
	}
	payload["template"] = plan.Template
	payload["objective"] = objective
	payload["perspective"] = plan.Perspective
	payload["controls"] = keys
	payload["assets"] = assets
	return payload
}

func applyJobOutcome(payload map[string]any, job Job) map[string]any {
	if payload == nil {
		payload = map[string]any{}
	}
	if job.ErrorCode != "" {
		payload["error_code"] = job.ErrorCode
	}
	if job.ChecksPassed != nil {
		payload["checks_passed"] = *job.ChecksPassed
	}
	if job.ChecksFailed != nil {
		payload["checks_failed"] = *job.ChecksFailed
	}
	if job.RepairPasses != nil {
		payload["repair_passes"] = *job.RepairPasses
	}
	if len(job.OpenChecks) > 0 {
		payload["open_checks"] = job.OpenChecks
	}
	if strings.TrimSpace(job.GameplayStatus) != "" {
		payload["gameplay_status"] = job.GameplayStatus
	}
	if job.Usage != nil && job.Usage.Calls > 0 {
		payload["usage"] = job.Usage
	}
	payload["job"] = job
	return payload
}

func (s *Service) finishFailure(job Job, ctx context.Context, failure error, result *BuildResult, repairPasses *int) {
	if result != nil {
		job.ChecksPassed, job.ChecksFailed = checkCounts(result.Checks)
		job.GameplayStatus = strings.TrimSpace(result.GameplayStatus)
		job.OpenChecks = openChecksFromResult(result)
	}
	job.RepairPasses = repairPasses
	s.terminateJob(job, ctx, failure)
}

func (s *Service) finishReady(job Job, project Project, revision Revision, result *BuildResult, repairPasses *int) {
	now := time.Now().UTC()
	job.Status = "ready"
	job.Phase = "ready"
	job.ResultRevision = revision.Number
	job.FinishedAt = &now
	job.ErrorCode = ""
	if result != nil {
		job.ChecksPassed, job.ChecksFailed = checkCounts(result.Checks)
		job.GameplayStatus = strings.TrimSpace(result.GameplayStatus)
	}
	job.RepairPasses = repairPasses
	s.finishWorkingCopy(job)
	s.mu.Lock()
	_, _ = s.db.Exec(`UPDATE gm_jobs SET status='ready',phase='ready',result_revision=?,finished_at=?,error_code='',gameplay_status=?,checks_passed=?,checks_failed=?,repair_passes=?,open_checks='' WHERE id=?`,
		revision.Number, now, job.GameplayStatus, nullableInt(job.ChecksPassed), nullableInt(job.ChecksFailed), nullableInt(job.RepairPasses), job.ID)
	s.releaseJobLocked(job.ID)
	s.mu.Unlock()
	s.mu.RLock()
	summary := s.jobSummaries[job.ID]
	s.mu.RUnlock()
	s.attachPromptUsage(&job)
	if strings.TrimSpace(summary) != "" {
		_, _ = s.appendMessage(context.Background(), project.ID, job.ID, "assistant", summary)
	}
	_, _ = s.emit(context.Background(), project.ID, job.ID, "preview_reload", map[string]any{"revision": revision.Number})
	_, _ = s.emit(context.Background(), project.ID, job.ID, "revision", map[string]any{"revision": revision})
	ready := map[string]any{"status": "ready"}
	if job.GameplayStatus != "" {
		ready["gameplay_status"] = job.GameplayStatus
	}
	_, _ = s.emit(context.Background(), project.ID, job.ID, "job_status", applyJobOutcome(ready, job))
}

func (s *Service) runAdoptJob(ctx context.Context, job Job, project Project, stage string) {
	if err := s.updateJobPhase(ctx, &job, "planning"); err != nil {
		s.terminateJob(job, ctx, &JobError{Code: "internal", Err: err})
		return
	}
	accepted, err := s.acceptPublishedPlan(ctx, job.ID)
	if err != nil {
		s.terminateJob(job, ctx, &JobError{Code: "planning", Err: err})
		return
	}
	_, _ = s.emit(ctx, project.ID, job.ID, "plan_summary", planSummaryPayload(accepted))
	if err := s.updateJobPhase(ctx, &job, "validating"); err != nil {
		s.terminateJob(job, ctx, &JobError{Code: "internal", Err: err})
		return
	}
	scope := "full"
	if project.Dimension == "3d" && !s.sceneBackedCurrent(ctx, job.ID, accepted) {
		scope = "startup"
	}
	result := s.validateForPublication(ctx, job.ID, scope)
	passes := intPtr(0)
	if !result.OK {
		s.finishFailure(job, ctx, &JobError{Code: validationFailureCode(result), Err: fmt.Errorf("game validation failed: %s", diagnosticsText(result.Diagnostics))}, &result, passes)
		return
	}
	revision, err := s.publishValidated(ctx, stage, project, job, result)
	if err != nil {
		s.finishFailure(job, ctx, &JobError{Code: "internal", Err: err}, &result, passes)
		return
	}
	s.finishReady(job, project, revision, &result, passes)
}

func (s *Service) LatestJob(ctx context.Context, projectID string) (Job, bool, error) {
	job, err := scanJob(s.db.QueryRowContext(ctx, `SELECT `+jobSelectColumns+` FROM gm_jobs WHERE project_id=? ORDER BY created_at DESC, rowid DESC LIMIT 1`, projectID))
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, false, nil
	}
	if err != nil {
		return Job{}, false, fmt.Errorf("get latest game maker job: %w", err)
	}
	s.attachPromptUsage(&job)
	return job, true, nil
}

func (s *Service) LatestPlanSummary(ctx context.Context, projectID string) (map[string]any, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM gm_events WHERE project_id=? AND event_type='plan_summary' ORDER BY id DESC LIMIT 1`, projectID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read game maker plan summary: %w", err)
	}
	payload := decodeEventPayload(raw)
	if len(payload) == 0 {
		return nil, nil
	}
	return payload, nil
}

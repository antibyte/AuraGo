package cydcreator

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Job modes, states and limits (overview "Job" JSON, spec §11).
const (
	JobModeNew          = "new"
	JobModeChange       = "change"
	JobModeGenerateCode = "generate_code"

	JobQueued               = "queued"
	JobRunning              = "running"
	JobAwaitingConfirmation = "awaiting_confirmation"
	JobSucceeded            = "succeeded"
	JobFailed               = "failed"
	JobCancelled            = "cancelled"
	JobInterrupted          = "interrupted"

	// MaxPromptRunes caps a job prompt and a chat message.
	MaxPromptRunes = 8000
	// MaxAttachmentBytes caps all attachments of one message (spec §11: 16 KiB).
	MaxAttachmentBytes = 16 << 10
	maxAttachments     = 4
	maxListedJobs      = 20
)

// Job phases (overview "Job" JSON). The Runner executes planning, layout,
// coding and repairing; queued, building and done belong to the orchestrator.
const (
	PhaseQueued    = "queued"
	PhasePlanning  = "planning"
	PhaseLayout    = "layout"
	PhaseCoding    = "coding"
	PhaseBuilding  = "building"
	PhaseRepairing = "repairing"
	PhaseDone      = "done"
)

// Job is one agent job (overview "Job" JSON). Prompt is the current request
// (an edited re-plan prompt replaces it) and is not part of the API answer;
// the chat history carries it as a user message.
type Job struct {
	ID            string   `json:"id"`
	ProjectID     string   `json:"project_id"`
	Mode          string   `json:"mode"`
	Status        string   `json:"status"`
	Phase         string   `json:"phase"`
	RepairAttempt int      `json:"repair_attempt"`
	Plan          *JobPlan `json:"plan"`
	ErrorCode     string   `json:"error_code"`
	ErrorMessage  string   `json:"error_message"`
	BuildID       string   `json:"build_id"`
	ResumeOf      string   `json:"resume_of"`
	CreatedAt     string   `json:"created_at"`
	StartedAt     string   `json:"started_at"`
	FinishedAt    string   `json:"finished_at"`
	Prompt        string   `json:"-"`
	StagingDir    string   `json:"-"`
}

// JobPlan is what planning produces for a new project (overview "Job" JSON).
type JobPlan struct {
	Summary     string   `json:"summary"`
	Screens     []string `json:"screens"`
	Widgets     []string `json:"widgets"`
	DataSources []string `json:"data_sources"`
	WiFi        string   `json:"wifi"`
	AuraGoLink  bool     `json:"aurago_link"`
	Libraries   []string `json:"libraries"`
}

// Attachment is extra data sent with a chat message; "log" is the only kind
// (selected serial monitor lines). The agent receives it as <external_data>.
type Attachment struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// StartJobInput is the body of POST /projects/{id}/jobs.
type StartJobInput struct {
	Prompt      string       `json:"prompt"`
	Mode        string       `json:"mode"`
	Resume      bool         `json:"resume"`
	Attachments []Attachment `json:"attachments"`
}

// isActiveJobStatus reports a job that holds the global agent slot and its
// project's lock.
func isActiveJobStatus(status string) bool {
	return status == JobQueued || status == JobRunning || status == JobAwaitingConfirmation
}

// validPrompt trims a prompt and checks its length and characters.
func validPrompt(prompt string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	if utf8.RuneCountInString(prompt) > MaxPromptRunes {
		return "", NewError(CodeBadRequest, fmt.Sprintf("the request may have at most %d characters", MaxPromptRunes), nil)
	}
	if !utf8.ValidString(prompt) || strings.ContainsRune(prompt, 0) {
		return "", NewError(CodeBadRequest, "the request must be UTF-8 text", nil)
	}
	return prompt, nil
}

// validAttachments checks kind, encoding and the 16 KiB total.
func validAttachments(list []Attachment) ([]Attachment, error) {
	if len(list) > maxAttachments {
		return nil, NewError(CodeBadRequest, fmt.Sprintf("at most %d attachments are allowed", maxAttachments), nil)
	}
	out := make([]Attachment, 0, len(list))
	total := 0
	for _, a := range list {
		if a.Kind != "log" {
			return nil, NewError(CodeBadRequest, fmt.Sprintf("unknown attachment kind %q", a.Kind), map[string]any{"kind": a.Kind})
		}
		if !utf8.ValidString(a.Text) || strings.ContainsRune(a.Text, 0) {
			return nil, NewError(CodeBadRequest, "attachments must be UTF-8 text", nil)
		}
		total += len(a.Text)
		if strings.TrimSpace(a.Text) == "" {
			continue
		}
		out = append(out, a)
	}
	if total > MaxAttachmentBytes {
		return nil, NewError(CodeBadRequest, fmt.Sprintf("attachments may have at most %d bytes", MaxAttachmentBytes),
			map[string]any{"max": MaxAttachmentBytes})
	}
	return out, nil
}

// Plan size limits: the decoded plan object, and the raw input before a
// string-wrapped plan is unwrapped (escaping at most doubles the object).
const (
	maxPlanBytes      = 32 << 10
	maxPlanInputBytes = 64 << 10
)

// planRuneNotAllowed reports a character plan text must not contain: control
// characters (newline only where allowNewline is set), format characters
// (bidirectional overrides, zero-width marks, BOM: the "Trojan Source"
// family) and Unicode line or paragraph separators. This is the set cComment
// folds; the plan rejects it instead, so the agent corrects its text.
func planRuneNotAllowed(r rune, allowNewline bool) bool {
	if r == '\n' {
		return !allowNewline
	}
	return unicode.IsControl(r) || unicode.In(r, unicode.Cf, unicode.Zl, unicode.Zp)
}

// planText checks one plan string entry.
func planText(field, value string, maxRunes int) error {
	if strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > maxRunes ||
		strings.IndexFunc(value, func(r rune) bool { return planRuneNotAllowed(r, false) }) >= 0 {
		return fmt.Errorf("%s entries must be 1..%d characters on one line without control or format characters", field, maxRunes)
	}
	return nil
}

// ParseJobPlan strictly decodes and validates a submitted plan. Errors are
// *Error CYD_BAD_REQUEST with details.reason (clipped, it can echo model
// input), so the agent can correct them.
func ParseJobPlan(data []byte) (JobPlan, error) {
	fail := func(reason string) (JobPlan, error) {
		reason = clipReason(reason)
		return JobPlan{}, NewError(CodeBadRequest, "the plan is invalid: "+reason, map[string]any{"reason": reason})
	}
	data = bytes.TrimSpace(data)
	if len(data) > maxPlanInputBytes {
		return fail("plan must be a JSON object of at most 32 KiB")
	}
	if len(data) > 0 && data[0] == '"' {
		var encoded string
		if err := json.Unmarshal(data, &encoded); err != nil {
			return fail("plan is not a JSON object")
		}
		data = bytes.TrimSpace([]byte(encoded))
	}
	if len(data) == 0 || data[0] != '{' || len(data) > maxPlanBytes {
		return fail("plan must be a JSON object of at most 32 KiB")
	}
	var plan JobPlan
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&plan); err != nil {
		return fail(err.Error())
	}
	if err := dec.Decode(new(json.RawMessage)); err != io.EOF {
		return fail("trailing data after the plan")
	}
	plan.Summary = strings.TrimSpace(plan.Summary)
	if plan.Summary == "" || utf8.RuneCountInString(plan.Summary) > 1000 ||
		strings.IndexFunc(plan.Summary, func(r rune) bool { return planRuneNotAllowed(r, true) }) >= 0 {
		return fail("summary must be 1..1000 characters without control or format characters (newlines are allowed)")
	}
	checks := []struct {
		field    string
		list     []string
		min, max int
		runes    int
	}{
		{"screens", plan.Screens, 1, MaxScreens, 120},
		{"widgets", plan.Widgets, 0, 64, 160},
		{"data_sources", plan.DataSources, 0, 16, 200},
	}
	for _, c := range checks {
		if len(c.list) < c.min || len(c.list) > c.max {
			return fail(fmt.Sprintf("%s must have %d..%d entries", c.field, c.min, c.max))
		}
		for _, v := range c.list {
			if err := planText(c.field, v, c.runes); err != nil {
				return fail(err.Error())
			}
		}
	}
	switch plan.WiFi {
	case "off", "optional", "required":
	default:
		return fail("wifi must be off, optional or required")
	}
	if plan.AuraGoLink && plan.WiFi == "off" {
		return fail("aurago_link needs wifi optional or required")
	}
	// Duplicates are refused, so a list longer than the catalog is invalid.
	if catalog := len(Libraries()); len(plan.Libraries) > catalog {
		return fail(fmt.Sprintf("libraries must have 0..%d entries", catalog))
	}
	for i, id := range plan.Libraries {
		if _, ok := libraryByID(id); !ok {
			return fail(fmt.Sprintf("unknown library %q", clipEcho(id)))
		}
		if slices.Contains(plan.Libraries[:i], id) {
			return fail(fmt.Sprintf("library %q is listed twice", id))
		}
	}
	plan.Screens, plan.Widgets = nonNil(plan.Screens), nonNil(plan.Widgets)
	plan.DataSources, plan.Libraries = nonNil(plan.DataSources), nonNil(plan.Libraries)
	return plan, nil
}

const jobColumns = `id,project_id,mode,status,phase,prompt,plan,repair_attempt,error_code,error_message,build_id,staging_dir,resume_of,created_at,started_at,finished_at`

func scanJob(row rowScanner) (Job, error) {
	var j Job
	var plan string
	if err := row.Scan(&j.ID, &j.ProjectID, &j.Mode, &j.Status, &j.Phase, &j.Prompt, &plan, &j.RepairAttempt, &j.ErrorCode,
		&j.ErrorMessage, &j.BuildID, &j.StagingDir, &j.ResumeOf, &j.CreatedAt, &j.StartedAt, &j.FinishedAt); err != nil {
		return Job{}, err
	}
	if plan != "" {
		var p JobPlan
		if err := json.Unmarshal([]byte(plan), &p); err != nil {
			// Written only by the service: a damaged row. Log the job id and the
			// error's type (a decoder message can quote content) and read no plan.
			slog.Warn("CYD Creator job record has an unreadable plan; reading it as null",
				"job_id", j.ID, "error_type", fmt.Sprintf("%T", err))
		} else {
			j.Plan = &p
		}
	}
	return j, nil
}

func planJSON(p *JobPlan) string {
	if p == nil {
		return ""
	}
	return mustJSON(p)
}

// loadJob returns the job or *Error CYD_JOB_NOT_FOUND.
func (s *Service) loadJob(ctx context.Context, id string) (Job, error) {
	j, err := scanJob(s.db.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM cc_jobs WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, NewError(CodeJobNotFound, "job not found", map[string]any{"job_id": id})
	}
	if err != nil {
		return Job{}, fmt.Errorf("read cyd creator job: %w", err)
	}
	return j, nil
}

// insertJob stores a new job row.
func (s *Service) insertJob(ctx context.Context, j Job) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO cc_jobs(id,project_id,mode,status,phase,prompt,plan,repair_attempt,error_code,
		error_message,build_id,staging_dir,resume_of,created_at,started_at,finished_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		j.ID, j.ProjectID, j.Mode, j.Status, j.Phase, j.Prompt, planJSON(j.Plan), j.RepairAttempt, j.ErrorCode, j.ErrorMessage,
		j.BuildID, j.StagingDir, j.ResumeOf, j.CreatedAt, j.StartedAt, j.FinishedAt)
	if err != nil {
		return fmt.Errorf("insert cyd creator job: %w", err)
	}
	return nil
}

// jobSaveRetryDelay is the pause before saveJob retries a failed UPDATE.
const jobSaveRetryDelay = 100 * time.Millisecond

// saveJob writes the mutable job fields and emits job_status. A failed UPDATE
// is retried once; job_status is emitted only for a stored state, so the UI
// never shows a state that cc_jobs (and so activeJob) does not have. The
// error is logged and returned (CYD_JOB_NOT_FOUND when the row is gone);
// callers that cannot act on it may ignore it.
func (s *Service) saveJob(j Job) error {
	var res sql.Result
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(jobSaveRetryDelay)
		}
		res, err = s.db.Exec(`UPDATE cc_jobs SET status=?, phase=?, prompt=?, plan=?, repair_attempt=?, error_code=?, error_message=?,
			build_id=?, started_at=?, finished_at=? WHERE id=?`, j.Status, j.Phase, j.Prompt, planJSON(j.Plan), j.RepairAttempt,
			j.ErrorCode, j.ErrorMessage, j.BuildID, j.StartedAt, j.FinishedAt, j.ID)
		if err == nil {
			break
		}
	}
	var n int64
	if err == nil {
		n, err = res.RowsAffected()
	}
	if err != nil {
		s.logger.Warn("CYD Creator job state could not be stored", "job_id", j.ID, "status", j.Status, "error", err)
		return fmt.Errorf("store cyd creator job state: %w", err)
	}
	if n == 0 {
		s.logger.Warn("CYD Creator job state was not stored: the job no longer exists", "job_id", j.ID, "status", j.Status)
		return NewError(CodeJobNotFound, "job not found", map[string]any{"job_id": j.ID})
	}
	s.Emit(j.ProjectID, EventJobStatus, j.ID, "", map[string]any{"job": j})
	return nil
}

// GetJob returns one job.
func (s *Service) GetJob(ctx context.Context, id string) (Job, error) {
	j, err := s.loadJob(ctx, id)
	return j, s.internalError("read job", err)
}

// ListJobs returns the newest 20 jobs of a project, newest first. Jobs created
// in the same millisecond are ordered by insertion (rowid), not by their
// random ids.
func (s *Service) ListJobs(ctx context.Context, projectID string) ([]Job, error) {
	if _, err := s.loadProjectRow(ctx, projectID); err != nil {
		return nil, s.internalError("list jobs", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+jobColumns+` FROM cc_jobs WHERE project_id=? ORDER BY created_at DESC, rowid DESC LIMIT ?`,
		projectID, maxListedJobs)
	if err != nil {
		return nil, s.internalError("list jobs", err)
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, s.internalError("list jobs", err)
		}
		out = append(out, j)
	}
	return out, s.internalError("list jobs", rows.Err())
}

// setProjectLock sets or clears cc_projects.locked_by_job under the project
// mutex (so no user write is half done) and emits lock_changed while still
// holding it (lock order project lock → eventsMu), so lock events of one
// project are stored in the order the lock changed. An unknown project is
// *Error CYD_NOT_FOUND and emits nothing.
func (s *Service) setProjectLock(ctx context.Context, projectID, jobID string) error {
	unlock := s.lockProject(projectID)
	defer unlock()
	res, err := s.db.ExecContext(ctx, `UPDATE cc_projects SET locked_by_job=? WHERE id=?`, jobID, projectID)
	if err != nil {
		return fmt.Errorf("set cyd creator project lock: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("set cyd creator project lock: %w", err)
	}
	if n == 0 {
		return NewError(CodeNotFound, "project not found", map[string]any{"project_id": projectID})
	}
	s.Emit(projectID, EventLockChanged, jobID, "", map[string]any{"locked_by_job": jobID})
	return nil
}

// JobStep kinds (overview SSE job_step).
const (
	StepLayout = "layout"
	StepFile   = "file"
	StepBuild  = "build"
	StepRepair = "repair"
	StepInfo   = "info"
)

var stepKinds = []string{StepLayout, StepFile, StepBuild, StepRepair, StepInfo}

// emitStep records a short English progress line. Only allowlisted kinds and
// server-written text reach the event; model output never does.
func (s *Service) emitStep(projectID, jobID, kind, text string) {
	if !slices.Contains(stepKinds, kind) {
		kind = StepInfo
	}
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) > 200 {
		text = string([]rune(text)[:199]) + "…"
	}
	s.Emit(projectID, EventJobStep, jobID, "", map[string]any{"job_id": jobID, "kind": kind, "text": text})
}

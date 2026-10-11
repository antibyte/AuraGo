package cydcreator

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
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

// planText checks one plan string entry.
func planText(field, value string, maxRunes int) error {
	if strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > maxRunes || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("%s entries must be 1..%d characters on one line", field, maxRunes)
	}
	return nil
}

// ParseJobPlan strictly decodes and validates a submitted plan. Errors are
// *Error CYD_BAD_REQUEST with details.reason, so the agent can correct them.
func ParseJobPlan(data []byte) (JobPlan, error) {
	fail := func(reason string) (JobPlan, error) {
		return JobPlan{}, NewError(CodeBadRequest, "the plan is invalid: "+reason, map[string]any{"reason": reason})
	}
	data = bytes.TrimSpace(data)
	if len(data) > 0 && data[0] == '"' {
		var encoded string
		if err := json.Unmarshal(data, &encoded); err != nil {
			return fail("plan is not a JSON object")
		}
		data = bytes.TrimSpace([]byte(encoded))
	}
	if len(data) == 0 || data[0] != '{' || len(data) > 32<<10 {
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
	if plan.Summary == "" || utf8.RuneCountInString(plan.Summary) > 1000 {
		return fail("summary must be 1..1000 characters")
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
	for _, id := range plan.Libraries {
		if _, ok := libraryByID(id); !ok {
			return fail(fmt.Sprintf("unknown library %q", id))
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
		if json.Unmarshal([]byte(plan), &p) == nil {
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

// saveJob writes the mutable job fields and emits job_status.
func (s *Service) saveJob(j Job) {
	if _, err := s.db.Exec(`UPDATE cc_jobs SET status=?, phase=?, prompt=?, plan=?, repair_attempt=?, error_code=?, error_message=?,
		build_id=?, started_at=?, finished_at=? WHERE id=?`, j.Status, j.Phase, j.Prompt, planJSON(j.Plan), j.RepairAttempt,
		j.ErrorCode, j.ErrorMessage, j.BuildID, j.StartedAt, j.FinishedAt, j.ID); err != nil {
		s.logger.Warn("CYD Creator job state could not be stored", "job_id", j.ID, "error", err)
	}
	s.Emit(j.ProjectID, EventJobStatus, j.ID, "", map[string]any{"job": j})
}

// GetJob returns one job.
func (s *Service) GetJob(ctx context.Context, id string) (Job, error) {
	j, err := s.loadJob(ctx, id)
	return j, s.internalError("read job", err)
}

// ListJobs returns the newest 20 jobs of a project, newest first.
func (s *Service) ListJobs(ctx context.Context, projectID string) ([]Job, error) {
	if _, err := s.loadProjectRow(ctx, projectID); err != nil {
		return nil, s.internalError("list jobs", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+jobColumns+` FROM cc_jobs WHERE project_id=? ORDER BY created_at DESC, id DESC LIMIT ?`,
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
// mutex (so no user write is half done) and emits lock_changed.
func (s *Service) setProjectLock(ctx context.Context, projectID, jobID string) error {
	unlock := s.lockProject(projectID)
	_, err := s.db.ExecContext(ctx, `UPDATE cc_projects SET locked_by_job=? WHERE id=?`, jobID, projectID)
	unlock()
	if err != nil {
		return fmt.Errorf("set cyd creator project lock: %w", err)
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

package cydcreator

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxToolDiagnostics = 50
	maxSummaryRunes    = 2000
)

// JobBuildResult is the cyd_creator_build answer.
type JobBuildResult struct {
	BuildID     string       `json:"build_id"`
	Status      string       `json:"status"`
	ErrorCode   string       `json:"error_code"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	// DiagnosticsTotal counts the findings before the 50-entry cap.
	DiagnosticsTotal int        `json:"diagnostics_total"`
	Usage            BuildUsage `json:"usage"`
	NextAction       string     `json:"next_action"`
}

// BuildForJob builds the working copy (an agent build in the shared queue)
// and waits for the result. Diagnostics in generated code are left out: they
// are generator bugs the agent must not work around.
func (s *Service) BuildForJob(ctx context.Context, jobID string) (JobBuildResult, error) {
	run, err := s.jobTool(jobID, ToolBuild)
	if err != nil {
		return JobBuildResult{}, err
	}
	b, err := s.jobBuild(ctx, run)
	if err != nil {
		return JobBuildResult{}, err
	}
	// Ordered before the cap, so the first error and the library notes stay.
	diags, keep := OrderAgentDiagnostics(agentDiagnostics(b.Diagnostics))
	total := len(diags)
	if len(diags) > max(maxToolDiagnostics, keep) {
		diags = diags[:max(maxToolDiagnostics, keep)]
	}
	out := JobBuildResult{BuildID: b.ID, Status: b.Status, ErrorCode: b.ErrorCode, Diagnostics: diags,
		DiagnosticsTotal: total, Usage: b.Usage}
	switch {
	case b.Status == BuildSucceeded:
		out.NextAction = "The build succeeded. Call cyd_creator_finish with a short summary for the user now."
	case repairableBuild(b, agentDiagnostics(b.Diagnostics)):
		out.NextAction = "Fix the listed errors in src/app, then build again."
	default:
		out.NextAction = "This failure is not caused by the app code; call cyd_creator_finish and describe what you completed."
	}
	return out, nil
}

// OrderAgentDiagnostics orders build findings for a bounded agent answer: the
// first error, the library notes, the other errors, warnings, other notes
// (each group in build order; the classes of capDiagnostics). keep counts the
// leading entries (first error and library notes) a shortened list must never
// drop. Ordering an ordered list changes nothing.
func OrderAgentDiagnostics(diags []Diagnostic) (ordered []Diagnostic, keep int) {
	first := firstError(diags)
	ordered = make([]Diagnostic, 0, len(diags))
	if first >= 0 {
		ordered = append(ordered, diags[first])
	}
	for class := 0; class < diagClasses; class++ {
		for i, d := range diags {
			if i != first && diagnosticClass(d) == class {
				ordered = append(ordered, d)
			}
		}
		if class == diagLibraryNote {
			keep = len(ordered)
		}
	}
	return ordered, keep
}

// SubmitPlan validates and accepts the planning result; the round ends after
// it (the runner's RunComplete hook reads PlanAccepted). The job_step line is
// server text: the plan's model-written summary reaches the chat only as the
// agent message of the waiting job, never a progress event.
func (s *Service) SubmitPlan(ctx context.Context, jobID string, data []byte) (JobPlan, error) {
	run, err := s.jobTool(jobID, ToolSubmitPlan)
	if err != nil {
		return JobPlan{}, err
	}
	plan, err := ParseJobPlan(data)
	if err != nil {
		return JobPlan{}, err
	}
	run.mu.Lock()
	run.plan, run.planAccepted = &plan, true
	run.mu.Unlock()
	s.emitStep(run.projectID, run.jobID, StepInfo, fmt.Sprintf("Plan submitted: %d screen(s), %d widget(s), %d data source(s)",
		len(plan.Screens), len(plan.Widgets), len(plan.DataSources)))
	return plan, nil
}

// FinishJob records the agent's summary for the chat and ends the coding or
// repair round (the runner's RunComplete hook reads JobFinished).
func (s *Service) FinishJob(ctx context.Context, jobID, summary string) error {
	run, err := s.jobTool(jobID, ToolFinish)
	if err != nil {
		return err
	}
	summary = strings.TrimSpace(summary)
	if summary == "" || utf8.RuneCountInString(summary) > maxSummaryRunes {
		return NewError(CodeBadRequest, "the summary must be 1..2000 characters", nil)
	}
	run.mu.Lock()
	run.summary, run.finished = summary, true
	run.mu.Unlock()
	return nil
}

package gamemaker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Capture data is transient and never included in events, revisions or exports.
type VisualCapture struct {
	Controlled bool   `json:"controlled"`
	Image      string `json:"image"`
	Scenario   string `json:"scenario"`
	At         string `json:"at"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	HUD        string `json:"hud,omitempty"`
}
type VisualFinding struct {
	Image       int     `json:"image"`
	Observation string  `json:"observation"`
	Region      string  `json:"region"`
	Severity    string  `json:"severity"`
	Confidence  float64 `json:"confidence"`
	Suggestion  string  `json:"suggestion"`
}
type VisualReview struct {
	Status   string          `json:"status"`
	Reason   string          `json:"reason,omitempty"`
	Provider string          `json:"provider,omitempty"`
	Model    string          `json:"model,omitempty"`
	BuildID  string          `json:"build_id,omitempty"`
	Findings []VisualFinding `json:"findings,omitempty"`
}

func (r VisualReview) RepairDiagnostics() []Diagnostic {
	var out []Diagnostic
	for _, f := range r.Findings {
		if f.Severity == "defect" && f.Confidence >= .85 {
			data, _ := json.Marshal(f)
			out = append(out, Diagnostic{Level: "visual", Message: string(data)})
		}
	}
	return out
}

func (s *Service) VisualBuildCurrent(jobID, buildID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activeJobID == jobID && s.previewCheck != nil && s.previewCheck.JobID == jobID && s.previewCheck.ID == buildID
}

func validCaptures(captures []VisualCapture) bool {
	if len(captures) > 2 {
		return false
	}
	for _, c := range captures {
		raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(c.Image, "data:image/png;base64,"))
		dimensions, err := png.DecodeConfig(bytes.NewReader(raw))
		if err != nil || dimensions.Width != c.Width || dimensions.Height != c.Height {
			return false
		}

		if len(c.Image) > 700000 || !validPreviewImage(c.Image) || len(c.Scenario) > 96 || len(c.At) > 40 || len(c.HUD) > 4000 || c.Width < 1 || c.Height < 1 || c.Width > 1280 || c.Height > 1280 {
			return false
		}
	}
	return true
}

// ReviewPreview reviews one current published preview without granting file tools.
func (s *Service) ReviewPreview(ctx context.Context, projectID string, report PreviewReport, providerID, model string) (VisualReview, error) {
	s.policyMu.RLock()
	policy := s.policy
	s.policyMu.RUnlock()
	if !policy.Enabled {
		return VisualReview{}, ErrDisabled
	}
	if policy.ReadOnly || !policy.AllowEdit {
		return VisualReview{}, ErrReadOnly
	}
	if !s.visualMu.TryLock() {
		return VisualReview{}, fmt.Errorf("visual review already running")
	}
	defer s.visualMu.Unlock()
	if len(report.Captures) == 0 || !validCaptures(report.Captures) {
		return VisualReview{}, fmt.Errorf("invalid preview captures")
	}
	current := func() bool {
		s.mu.RLock()
		grant, ok := s.tokens[report.Token]
		active := s.activeJobID
		s.mu.RUnlock()
		p, err := s.GetProject(ctx, projectID)
		return err == nil && ok && active == "" && grant.ProjectID == projectID && grant.JobID == "" && grant.Revision == p.CurrentRevision && time.Now().Before(grant.ExpiresAt)
	}
	if !current() {
		return VisualReview{}, ErrInvalidToken
	}
	p, err := s.GetProject(ctx, projectID)
	if err != nil {
		return VisualReview{}, err
	}
	if strings.TrimSpace(providerID) == "" {
		providerID = p.ProviderID
	}
	if strings.TrimSpace(model) == "" {
		model = p.Model
	}
	var plan *GamePlan
	// The same private plan used for the published revision; never export it.
	path, _, err := secureJoin(filepath.Join(s.opts.WorkspacePath, filepath.FromSlash(p.ProjectKey)), gamePlanPath, true)
	if err == nil {
		if b, e := os.ReadFile(path); e == nil && len(b) <= 32768 {
			_ = json.Unmarshal(b, &plan)
		}
	}
	result := BuildResult{Captures: report.Captures, Visual: VisualReview{BuildID: fmt.Sprintf("revision-%d", p.CurrentRevision)}}
	s.mu.RLock()
	runner := s.runner
	s.mu.RUnlock()
	if runner == nil {
		return VisualReview{}, fmt.Errorf("game runner unavailable")
	}
	err = runner.RunGameMakerJob(ctx, JobRun{Stage: "visual", Result: &result, Plan: plan, Project: p, Job: Job{ProviderID: strings.TrimSpace(providerID), Model: strings.TrimSpace(model)}, Captures: report.Captures})
	if err != nil {
		return VisualReview{}, err
	}
	if !current() {
		return VisualReview{}, ErrInvalidToken
	}
	return result.Visual, nil
}

func (s *Service) reviewAndRepairVisual(ctx context.Context, runner Runner, run JobRun, result BuildResult, scope string) BuildResult {
	for attempt := 0; attempt < 2; attempt++ {
		run.Stage = "visual"
		run.Result = &result
		run.Images = result.Images
		run.Captures = result.Captures
		if result.check != nil {
			result.Visual.BuildID = result.check.ID
		}
		if err := runner.RunGameMakerJob(ctx, run); err != nil {
			result.VisualStatus = "failed"
			result.Visual.Status = "failed"
			result.Visual.Reason = "analysis_failed"
			if ctx.Err() != nil {
				result.OK = false
			}
			return result
		}
		if ctx.Err() != nil {
			result.OK = false
			return result
		}
		if result.check != nil && !s.VisualBuildCurrent(run.Job.ID, result.check.ID) {
			return previewUnavailable("Preview build changed during visual analysis")
		}
		diagnostics := result.Visual.RepairDiagnostics()
		if attempt > 0 || len(diagnostics) == 0 {
			return result
		}
		// Preserve enough time to validate the repaired build under the existing job deadline.
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < 90*time.Second {
			return result
		}
		_ = s.EmitAgentEvent(ctx, run.Project.ID, run.Job.ID, "visual_progress", map[string]any{"status": "repairing"})
		run.Stage = "repair"
		run.Diagnostics = diagnostics
		if err := runner.RunGameMakerJob(ctx, run); err != nil {
			result.OK = false
			result.Diagnostics = append(result.Diagnostics, Diagnostic{Level: "error", Message: err.Error()})
			return result
		}
		result = s.ValidateJobScope(ctx, run.Job.ID, scope)
		if !result.OK {
			return result
		}
	}
	return result
}

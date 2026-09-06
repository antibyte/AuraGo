package gamemaker

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// previewCheck belongs to one build, never to a previous iframe or revision.
// All fields are protected by Service.mu.
type previewCheck struct {
	ID               string
	JobID            string
	ReadyAt          time.Time
	Diagnostics      []Diagnostic
	Scenarios        []GameScenario
	Observations     []GameObservation
	GameplayReceived bool
	BoundToken       string
	Images           []string
}

type PreviewReport struct {
	CanvasVisible bool              `json:"canvas_visible,omitempty"`
	Token         string            `json:"token"`
	Type          string            `json:"type"`
	Message       string            `json:"message,omitempty"`
	Observations  []GameObservation `json:"observations,omitempty"`
	Images        []string          `json:"images,omitempty"`
}

func boundedPreviewDiagnostics(input []Diagnostic) []Diagnostic {
	var out []Diagnostic
	for _, item := range input {
		message := strings.TrimSpace(item.Message)
		if message == "" {
			continue
		}
		runes := []rune(message)
		if len(runes) > 1000 {
			message = string(runes[:1000])
		}
		duplicate := false
		for _, previous := range out {
			duplicate = duplicate || previous.Message == message
		}
		if !duplicate {
			out = append(out, Diagnostic{Level: "runtime", Message: message})
		}
		if len(out) == 20 {
			break
		}
	}
	return out
}

// ReportPreview is called by the authenticated parent Studio, not the sandbox.
// Stale build reports are ignored; they must not validate or fail a newer build.
func (s *Service) ReportPreview(projectID string, report PreviewReport) error {
	s.policyMu.RLock()
	enabled := s.policy.Enabled
	s.policyMu.RUnlock()
	if !enabled {
		return ErrDisabled
	}
	switch report.Type {
	case "ready", "runtime_error", "resource_error", "diagnostic", "gameplay":
	default:
		return fmt.Errorf("unsupported preview report type")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	grant, ok := s.tokens[report.Token]
	if !ok || grant.ProjectID != projectID || time.Now().After(grant.ExpiresAt) {
		return ErrInvalidToken
	}
	check := s.previewCheck
	if check == nil || grant.ValidationID != check.ID || grant.JobID != check.JobID || s.activeJobID != check.JobID {
		return nil
	}
	if report.Type == "ready" {
		if !report.CanvasVisible {
			return nil
		}
		if check.ReadyAt.IsZero() {
			check.ReadyAt = time.Now()
			check.BoundToken = report.Token
		}
	} else if report.Type == "gameplay" {
		if report.Token != check.BoundToken || len(check.Scenarios) == 0 || check.GameplayReceived {
			return nil
		}
		if err := validateGameReport(report); err != nil {
			return err
		}
		check.Observations = report.Observations
		for _, image := range report.Images {
			if validPreviewImage(image) {
				check.Images = append(check.Images, image)
			}
		}
		check.GameplayReceived = true
	} else {
		message := strings.TrimSpace(report.Message)
		if message == "" {
			message = report.Type
		}
		check.Diagnostics = boundedPreviewDiagnostics(append(check.Diagnostics, Diagnostic{Message: message}))
	}
	return nil
}

// ValidateJob uses the existing sandboxed browser preview as a bounded startup
// smoke check. File writes still use BuildJob without waiting on a browser.
func (s *Service) ValidateJob(ctx context.Context, jobID string) BuildResult {
	return s.ValidateJobScope(ctx, jobID, "startup")
}

func (s *Service) waitForPreview(ctx context.Context, check *previewCheck, timeout time.Duration) BuildResult {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		s.mu.RLock()
		current := check != nil && s.previewCheck == check
		var diagnostics []Diagnostic
		var ready bool
		if current {
			diagnostics = append(diagnostics, check.Diagnostics...)
			// Canvas creation precedes scene setup and common one-second spawn
			// timers. Observe a short gameplay interval before accepting startup.
			ready = !check.ReadyAt.IsZero() && time.Since(check.ReadyAt) >= 3*time.Second
		}
		s.mu.RUnlock()
		if !current {
			return previewUnavailable("Preview build changed during validation; validate the current build again")
		}
		if len(diagnostics) > 0 {
			return BuildResult{RuntimeStatus: "failed", Diagnostics: diagnostics}
		}
		if err := ctx.Err(); err != nil {
			return previewUnavailable(err.Error())
		}
		if ready {
			return BuildResult{OK: true, RuntimeStatus: "passed"}
		}
		select {
		case <-ctx.Done():
			return previewUnavailable(ctx.Err().Error())
		case <-timer.C:
			return previewUnavailable("Browser startup could not be verified. Keep this project's Game Maker Studio preview open and retry; no revision was published")
		case <-tick.C:
		}
	}
}

func previewUnavailable(message string) BuildResult {
	return BuildResult{RuntimeStatus: "unavailable", Diagnostics: []Diagnostic{{Level: "error", Message: message}}}
}

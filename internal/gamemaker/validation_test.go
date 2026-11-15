package gamemaker

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestPreviewValidationRejectsStaleReportsAndReadyBeforeError(t *testing.T) {
	s := newTestService(t)
	check := &previewCheck{ID: "current", JobID: "job"}
	s.previewCheck = check
	s.activeJobID = "job"
	s.tokens["current-token"] = previewToken{ProjectID: "project", JobID: "job", ValidationID: "current", ExpiresAt: time.Now().Add(time.Minute)}
	s.tokens["old-token"] = previewToken{ProjectID: "project", JobID: "job", ValidationID: "old", ExpiresAt: time.Now().Add(time.Minute)}
	if err := s.ReportPreview("another-project", PreviewReport{Token: "current-token", Type: "ready"}); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("cross-project report: %v", err)
	}
	for _, kind := range []string{"ready", "runtime_error"} {
		if err := s.ReportPreview("project", PreviewReport{Token: "old-token", Type: kind, Message: "old error"}); err != nil {
			t.Fatal(err)
		}
	}
	if !check.ReadyAt.IsZero() || len(check.Diagnostics) != 0 {
		t.Fatal("stale iframe affected the current build")
	}
	_ = s.ReportPreview("project", PreviewReport{Token: "current-token", Type: "ready"})
	if check.ReadyAt.IsZero() {
		t.Fatal("HTML readiness required a canvas")
	}
	_ = s.ReportPreview("project", PreviewReport{Token: "current-token", Type: "ready", CanvasVisible: true})
	const message = "Uncaught TypeError: this.scale.setSize is not a function"
	for range 30 {
		_ = s.ReportPreview("project", PreviewReport{Token: "current-token", Type: "runtime_error", Message: message})
	}
	if got := check; len(got.Diagnostics) != 1 || got.Diagnostics[0].Message != message {
		t.Fatalf("ready suppressed runtime error or duplicates were retained: %+v", got)
	}
	if got := boundedPreviewDiagnostics([]Diagnostic{{Message: strings.Repeat("ö", 1001)}}); len([]rune(got[0].Message)) != 1000 {
		t.Fatal("diagnostics are not bounded as valid UTF-8")
	}
}

func TestPreviewBootObservesErrorsBeforeGameScripts(t *testing.T) {
	html := string(injectPreviewBoot([]byte(`<html><head><script src="game.js"></script></head><body></body></html>`)))
	if strings.Index(html, previewBootMarker) > strings.Index(html, `src="game.js"`) {
		t.Fatal("runtime listeners were injected after game code")
	}
	for _, marker := range []string{`addEventListener('error'`, `addEventListener('unhandledrejection'`, `'resource_error'`} {
		if !strings.Contains(html, marker) {
			t.Fatalf("missing browser diagnostic capture: %s", marker)
		}
	}
}

func TestPreviewPhysicsErrorsIncludeBoundedRepairGuidance(t *testing.T) {
	s := newTestService(t)
	s.previewCheck = &previewCheck{ID: "build", JobID: "job"}
	s.activeJobID = "job"
	s.tokens["token"] = previewToken{ProjectID: "project", JobID: "job", ValidationID: "build", ExpiresAt: time.Now().Add(time.Minute)}
	for _, method := range []string{"setVelocity", "setPosition", "setImmovable"} {
		message := "Uncaught TypeError: this.paddle.body." + method + " is not a function"
		for range 2 {
			if err := s.ReportPreview("project", PreviewReport{Token: "token", Type: "runtime_error", Message: message}); err != nil {
				t.Fatal(err)
			}
		}
	}
	result := s.previewCheck
	if len(result.Diagnostics) != 3 {
		t.Fatalf("physics errors were lost or duplicated: %+v", result)
	}
	for _, d := range result.Diagnostics {
		if !strings.Contains(d.Message, "fixed=false") || !strings.Contains(d.Message, "body.reset(x,y)") || strings.Count(d.Message, "Arcade Physics:") != 1 || len([]rune(d.Message)) > 1000 {
			t.Fatalf("missing or repeated physics repair guidance: %s", d.Message)
		}
	}

}

func TestPreviewGroupErrorGuidance(t *testing.T) {
	input := []Diagnostic{{Message: "Uncaught TypeError: this.blocks.removeAll is not a function"}}
	first := boundedPreviewDiagnostics(input)
	second := boundedPreviewDiagnostics(first)
	if len(second) != 1 || second[0] != first[0] || !strings.Contains(second[0].Message, "group.clear(true,true)") {
		t.Fatalf("missing or repeated group repair hint: %+v", second)
	}
}

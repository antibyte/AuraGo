package gamemaker

import (
	"context"
	"testing"
	"time"
)

func TestMissingPreviewHostOnlyMatchesAbsentHost(t *testing.T) {
	check := &previewCheck{ID: "validation", JobID: "job"}
	absent := previewUnavailable(previewHostAbsentMessage)
	absent.check = check
	if !missingPreviewHost(absent) {
		t.Fatal("the absent-preview timeout was not recognised")
	}
	if missingPreviewHost(previewUnavailable(previewHostAbsentMessage)) {
		t.Fatal("a result without a bound preview check must not wait")
	}
	stale := previewUnavailable("Preview build changed during validation; validate the current build again")
	stale.check = check
	if missingPreviewHost(stale) {
		t.Fatal("a changed build must fail immediately")
	}
	runtimeError := BuildResult{RuntimeStatus: "failed", Diagnostics: []Diagnostic{{Level: "error", Message: previewHostAbsentMessage}}}
	runtimeError.check = check
	if missingPreviewHost(runtimeError) {
		t.Fatal("a broken game must never enter the preview wait")
	}
}

func TestAwaitPreviewHostResumesWhenPreviewAppears(t *testing.T) {
	s := newTestService(t)
	check := &previewCheck{ID: "validation", JobID: "job"}
	s.previewCheck = check
	s.activeJobID = "job"
	s.tokens["token"] = previewToken{ProjectID: "project", JobID: "job", ValidationID: "validation", ExpiresAt: time.Now().Add(time.Minute)}
	job := Job{ID: "job", ProjectID: "project"}
	go func() {
		time.Sleep(120 * time.Millisecond)
		_ = s.ReportPreview("project", PreviewReport{Token: "token", Type: "ready", CanvasVisible: true})
	}()
	start := time.Now()
	if !s.awaitPreviewHost(context.Background(), job, check, 3*time.Second) {
		t.Fatal("a visible preview did not resume the parked validation")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("resumed too late: %s", elapsed)
	}
}

func TestAwaitPreviewHostTimesOutWithoutPreview(t *testing.T) {
	s := newTestService(t)
	check := &previewCheck{ID: "validation", JobID: "job"}
	s.previewCheck = check
	s.activeJobID = "job"
	job := Job{ID: "job", ProjectID: "project"}
	start := time.Now()
	if s.awaitPreviewHost(context.Background(), job, check, 150*time.Millisecond) {
		t.Fatal("a missing preview must not resume validation")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("wait exceeded its bound: %s", elapsed)
	}
}

func TestAwaitPreviewHostStopsForNewerBuild(t *testing.T) {
	s := newTestService(t)
	check := &previewCheck{ID: "validation", JobID: "job"}
	s.previewCheck = check
	s.activeJobID = "job"
	job := Job{ID: "job", ProjectID: "project"}
	go func() {
		time.Sleep(120 * time.Millisecond)
		s.mu.Lock()
		s.previewCheck = &previewCheck{ID: "replacement", JobID: "job"}
		s.mu.Unlock()
	}()
	if s.awaitPreviewHost(context.Background(), job, check, 3*time.Second) {
		t.Fatal("a stale validation must not resume after a newer build")
	}
}

func TestAwaitPreviewHostHonoursJobDeadline(t *testing.T) {
	s := newTestService(t)
	check := &previewCheck{ID: "validation", JobID: "job"}
	s.previewCheck = check
	s.activeJobID = "job"
	job := Job{ID: "job", ProjectID: "project"}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	if s.awaitPreviewHost(ctx, job, check, previewHostWait) {
		t.Fatal("a cancelled job must not resume validation")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("wait ignored the job deadline: %s", elapsed)
	}
	if ctx.Err() == nil {
		t.Fatal("expected the bounded context to expire")
	}
}

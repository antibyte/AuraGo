package optimizer

import (
	"aurago/internal/prompts"
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"
)

func TestExposureDeduplicatesRequestAndRetainsFailure(t *testing.T) {
	db, _ := openOptimizerTestDB(t)
	token := db.recordPromptGuideExposures([]prompts.ToolGuideExposure{{Manual: "docker", Version: "v1", SourceRevision: "source"}}, "request")["docker"]
	for _, success := range []bool{true, false, true} {
		if err := db.LogToolTrace("docker", success, 0, token, "failure", 1, "list"); err != nil {
			t.Fatal(err)
		}
	}
	var n, success int
	if err := db.db.QueryRow(`SELECT count(*),sum(success) FROM tool_traces WHERE exposure_id IS NOT NULL`).Scan(&n, &success); err != nil || n != 1 || success != 0 {
		t.Fatalf("inflated evidence: %d %d %v", n, success, err)
	}
	stats, err := db.GetDashboardStats()
	if err != nil || stats.TotalTraceEvents != 1 || stats.Exposures != 1 {
		t.Fatalf("invalid stats: %+v %v", stats, err)
	}
	if err := db.LogToolTrace("docker", true, 0, token, "", 1, "inspect"); err != nil {
		t.Fatal(err)
	}
	if err := db.db.QueryRow(`SELECT count(*),sum(success) FROM tool_traces WHERE exposure_id IS NOT NULL`).Scan(&n, &success); err != nil || n != 2 || success != 1 {
		t.Fatalf("different operations contaminated one another: %d %d %v", n, success, err)
	}
}

func TestGuideVariantIsStableAndRejectsStaleSource(t *testing.T) {
	db, _ := openOptimizerTestDB(t)
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(db.loadCanonicalManual("docker"))))
	_, err := db.db.Exec(`INSERT INTO prompt_overrides(tool_name,mutated_prompt,original_hash,active,shadow) VALUES('docker','Use list before inspect.',?,0,1)`, hash)
	if err != nil {
		t.Fatal(err)
	}
	first := selectToolGuideVariant("docker", "canonical body", "stable-run")
	for i := 0; i < 20; i++ {
		if got := selectToolGuideVariant("docker", "canonical body", "stable-run"); got != first {
			t.Fatal("assignment changed")
		}
	}
	if got := db.selectToolGuideVariant("docker", "canonical body", 0); got.Version == "v1" {
		t.Fatal("valid shadow not selected")
	}
	_, _ = db.db.Exec(`UPDATE prompt_overrides SET original_hash='outdated'`)
	if got := db.selectToolGuideVariant("docker", "canonical body", 0); got.Version != "v1" {
		t.Fatal("stale source selected")
	}
}

func TestOptimizerWaitsForComparableOperationEvidence(t *testing.T) {
	db, _ := openOptimizerTestDB(t)
	worker := NewOptimizerWorker(db, nil, nil, 0)
	for _, v := range []string{"v1", "candidate"} {
		for i := 0; i < 5; i++ {
			token := db.recordPromptGuideExposures([]prompts.ToolGuideExposure{{Manual: "docker", Version: v, SourceRevision: "same"}}, "r")["docker"]
			if err := db.LogToolTrace("docker", v != "v1", 0, token, "", 0, v); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, _, ok := worker.comparableRates(context.Background(), "docker", "candidate", "v1"); ok {
		t.Fatal("different operations compared")
	}
}

func TestManagedOptimizerToggleDoesNotOverlap(t *testing.T) {
	started, stopped := make(chan struct{}, 4), make(chan struct{}, 4)
	stop := startManagedOptimizer(context.Background(), false, func(ctx context.Context) { started <- struct{}{}; <-ctx.Done(); stopped <- struct{}{} })
	defer stop()
	wait := func(ch <-chan struct{}) {
		t.Helper()
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatal("lifecycle event missing")
		}
	}
	SetEnabled(true)
	wait(started)
	for i := 0; i < 20; i++ {
		SetEnabled(true)
	}
	select {
	case <-started:
		t.Fatal("duplicate worker")
	case <-time.After(20 * time.Millisecond):
	}
	SetEnabled(false)
	wait(stopped)
	SetEnabled(true)
	wait(started)
	SetEnabled(false)
	wait(stopped)
}

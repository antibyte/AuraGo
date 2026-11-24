package main

import (
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

// A config written before plan mode gains the video_studio.plan block from the template, and config.Load reads its
// defaults.
func TestRunAddsVideoStudioPlanDefaults(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(sourcePath, []byte("server:\n    port: 8088\n"+preProVideoStudioSection), 0o600); err != nil {
		t.Fatal(err)
	}
	run(sourcePath, filepath.Join("..", "..", "config_template.yaml"), sourcePath)
	merged := mergedFileMap(t, sourcePath)
	for key, want := range map[string]interface{}{"enabled": true, "agent_provider": "", "agent_model": "", "max_daily_jobs": 20, "job_timeout_seconds": 900} {
		if got := mergedValue(merged, "video_studio.plan."+key); got != want {
			t.Fatalf("video_studio.plan.%s = %#v, want %#v", key, got, want)
		}
	}
	cfg, err := config.Load(sourcePath)
	if err != nil {
		t.Fatalf("Load(merged): %v", err)
	}
	if want := (config.VideoStudioPlanConfig{Enabled: true, MaxDailyJobs: 20, JobTimeoutSeconds: 900}); cfg.VideoStudio.Plan != want {
		t.Fatalf("plan after the upgrade = %+v, want %+v", cfg.VideoStudio.Plan, want)
	}
}

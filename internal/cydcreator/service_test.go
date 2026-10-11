package cydcreator

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewServiceRequiresPaths(t *testing.T) {
	dir := t.TempDir()
	cases := []ServiceConfig{
		{DataDir: dir, DBPath: filepath.Join(dir, "x.db")},
		{WorkspaceDir: dir, DBPath: filepath.Join(dir, "x.db")},
		{WorkspaceDir: dir, DataDir: dir},
	}
	for i, cfg := range cases {
		if svc, err := NewService(cfg); err == nil {
			_ = svc.Close()
			t.Errorf("case %d: NewService accepted a missing path", i)
		}
	}
}

func TestNewServiceCreatesDirectories(t *testing.T) {
	env := newTestService(t)
	for _, d := range []string{filepath.Join(env.cfg.WorkspaceDir, "CYD"), filepath.Join(env.cfg.DataDir, "cyd_creator", "artifacts")} {
		if info, err := os.Stat(d); err != nil || !info.IsDir() {
			t.Errorf("directory %s missing: %v", d, err)
		}
	}
}

func TestServiceLimitsDefaultsAndLiveUpdate(t *testing.T) {
	env := newTestService(t)
	got := env.svc.Limits()
	if got != DefaultServiceLimits() {
		t.Fatalf("limits = %+v, want the defaults %+v", got, DefaultServiceLimits())
	}
	if got.BuildTimeout != 300*time.Second || got.ColdBuildTimeout != 600*time.Second || got.KeepBuildsPerProject != 5 ||
		got.MaxRepairAttempts != 3 || got.JobTimeout != 20*time.Minute || !got.OTA.Enabled || got.OTA.HeartbeatSeconds != 60 {
		t.Fatalf("defaults differ from spec §15: %+v", got)
	}
	env.svc.UpdateLimits(ServiceLimits{BuildTimeout: 90 * time.Second, ColdBuildTimeout: 30 * time.Second, KeepBuildsPerProject: 2})
	got = env.svc.Limits()
	if got.BuildTimeout != 90*time.Second || got.ColdBuildTimeout != 90*time.Second || got.KeepBuildsPerProject != 2 {
		t.Fatalf("updated limits = %+v (cold must be at least warm)", got)
	}
}

func TestDefaultServicePublication(t *testing.T) {
	env := newTestService(t)
	SetDefaultService(env.svc)
	t.Cleanup(func() { SetDefaultService(nil) })
	if DefaultService() != env.svc {
		t.Fatal("DefaultService did not return the published service")
	}
	SetDefaultService(nil)
	if DefaultService() != nil {
		t.Fatal("SetDefaultService(nil) did not withdraw the service")
	}
}

func TestValidOrigin(t *testing.T) {
	for origin, want := range map[string]bool{
		OriginUser: true, OriginRestore: true, OriginImport: true, JobOrigin("ccj_0001"): true,
		"": false, "job:": false, "job:../x": false, "admin": false,
	} {
		if got := validOrigin(origin); got != want {
			t.Errorf("validOrigin(%q) = %v, want %v", origin, got, want)
		}
	}
}

func TestNewServiceRecoversInterruptedBuilds(t *testing.T) {
	dir := t.TempDir()
	env := newTestService(t, testServiceOptions{dir: dir})
	now := "2026-10-10T08:00:00.000Z"
	if _, err := env.svc.db.Exec(`INSERT INTO cc_builds(id,project_id,origin,status,board,project_version,created_at)
		VALUES('ccb_crash','ccp_crash','user','running','esp32-2432s028r',1,?)`, now); err != nil {
		t.Fatal(err)
	}
	partial := filepath.Join(env.cfg.DataDir, "cyd_creator", "artifacts", "ccb_crash")
	staging := filepath.Join(env.cfg.DataDir, "cyd_creator", "artifacts", "ccb_other.tmp")
	for _, d := range []string{partial, staging} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	_ = env.svc.Close()

	restarted, err := NewService(env.cfg)
	if err != nil {
		t.Fatalf("NewService after crash: %v", err)
	}
	defer restarted.Close()
	b, err := restarted.loadBuild(context.Background(), "ccb_crash")
	if err != nil || b.Status != BuildInterrupted || b.ErrorCode != CodeBuildInterrupted || b.FinishedAt == "" {
		t.Fatalf("build after restart = %+v, %v", b, err)
	}
	for _, d := range []string{partial, staging} {
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			t.Errorf("partial artefacts %s were kept", d)
		}
	}
	events, err := restarted.EventsAfter(context.Background(), "ccp_crash", 0, 0)
	if err != nil || len(events) == 0 || events[len(events)-1].Type != EventBuildStatus || events[len(events)-1].BuildID != "ccb_crash" {
		t.Fatalf("events after restart = %+v, %v; want a build_status for ccb_crash", events, err)
	}
}

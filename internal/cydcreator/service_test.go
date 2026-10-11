package cydcreator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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

func TestCheckLock(t *testing.T) {
	cases := []struct {
		name     string
		lockedBy string
		origin   string
		code     string // "" = allowed
	}{
		{"invalid origin", "", "admin", CodeBadRequest},
		{"invalid job origin", "ccj_0001", "job:../x", CodeBadRequest},
		{"user while unlocked", "", OriginUser, ""},
		{"job while unlocked", "", JobOrigin("ccj_0001"), ""},
		{"own job", "ccj_0001", JobOrigin("ccj_0001"), ""},
		{"user while locked", "ccj_0001", OriginUser, CodeProjectLocked},
		{"restore while locked", "ccj_0001", OriginRestore, CodeProjectLocked},
		{"other job", "ccj_0001", JobOrigin("ccj_0002"), CodeProjectLocked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkLock(projectRow{ID: "ccp_1", LockedByJob: tc.lockedBy}, tc.origin)
			if tc.code == "" {
				if err != nil {
					t.Fatalf("checkLock = %v, want nil", err)
				}
				return
			}
			typed := wantCode(t, err, tc.code)
			if tc.code == CodeProjectLocked && typed.Details["job_id"] != tc.lockedBy {
				t.Fatalf("details = %v, want job_id %s", typed.Details, tc.lockedBy)
			}
		})
	}
}

func TestInternalError(t *testing.T) {
	var logs bytes.Buffer
	s := &Service{logger: slog.New(slog.NewTextHandler(&logs, nil))}
	typed := NewError(CodeNotFound, "project not found", nil)
	cases := []struct {
		name string
		err  error
		same bool // returned unchanged
	}{
		{"typed", typed, true},
		{"wrapped typed", fmt.Errorf("load: %w", typed), true},
		{"canceled", context.Canceled, true},
		{"wrapped deadline", fmt.Errorf("query: %w", context.DeadlineExceeded), true},
		{"cause", errors.New("disk I/O error at /secret/path"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			got := s.internalError("read project", tc.err)
			if tc.same {
				if got != tc.err || logs.Len() != 0 {
					t.Fatalf("internalError = %v (logged %q), want the error unchanged and nothing logged", got, logs.String())
				}
				return
			}
			internal := wantCode(t, got, CodeInternal)
			if internal.Message != "read project failed" || strings.Contains(internal.Error(), "secret") {
				t.Fatalf("internal error = %q, want a generic message", internal.Error())
			}
			if !strings.Contains(logs.String(), "disk I/O error at /secret/path") || !strings.Contains(logs.String(), "op=\"read project\"") {
				t.Fatalf("log = %q, want the op and the cause", logs.String())
			}
		})
	}
	if s.internalError("noop", nil) != nil {
		t.Fatal("internalError(nil) != nil")
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

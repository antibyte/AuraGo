package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/rustfs"
	"aurago/internal/selftest"
)

func TestRustFSConfigRejectsInvalidScopesBeforeRuntime(t *testing.T) {
	cfg := &config.Config{}
	cfg.RustFS.Permissions.AllowedLocations = []config.RustFSLocation{{Bucket: "aurago", Prefixes: []string{"private/*"}}}
	if _, err := (&Server{}).prepareRustFSConfigSave(context.Background(), cfg); err == nil {
		t.Fatal("wildcard scope accepted")
	}
	if !reservedVaultSecretKey("rustfs_state") || !isAdminProtectedPath("/api/rustfs/install") {
		t.Fatal("private administrative boundary missing")
	}
}

func TestRustFSConfigSaveCompletionFencesDurableAuthority(t *testing.T) {
	for _, phase := range []string{"aborted", "committed_before_reload", "incomplete_rollback"} {
		t.Run(phase, func(t *testing.T) {
			committed := phase == "committed_before_reload"
			cfg := &config.Config{}
			cfg.S3.Enabled = true
			cfg.S3.Provider = config.S3ProviderExternal
			cfg.S3.AccessKey = "test-s3-access"
			cfg.S3.SecretKey = "test-s3-secret"
			manager := rustfs.NewManager(context.Background(), cfg, nil, nil)
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if err := manager.Shutdown(ctx); err != nil {
					t.Error(err)
				}
			})
			s := &Server{Cfg: cfg, RustFS: manager}
			oldLease, err := manager.AcquireS3(context.Background(), cfg, true)
			if err != nil {
				t.Fatal(err)
			}
			defer oldLease.Release()
			next := cfg.Clone()
			next.S3.ReadOnly = true
			finish, err := s.prepareRustFSConfigSave(context.Background(), next)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := manager.AcquireS3(context.Background(), cfg, true); err == nil {
				t.Fatal("prepared change left old authority available")
			}
			var saveErr error
			if phase == "incomplete_rollback" {
				saveErr = &providerWriteError{code: "provider_rollback_failed", message: "fixture rollback failed"}
			}
			finish(committed, saveErr)
			finish(false, nil)
			if oldLease.Check() == nil {
				t.Fatal("completion revived an issued lease")
			}
			if phase == "incomplete_rollback" {
				for _, candidate := range []*config.Config{cfg, next} {
					if _, err := manager.AcquireS3(context.Background(), candidate, true); err == nil {
						t.Fatal("unknown rollback released prepared admission")
					}
				}
				return
			}
			effective := cfg
			if committed {
				if _, err := manager.AcquireS3(context.Background(), cfg, true); err == nil {
					t.Fatal("reload failure restored grants superseded by a durable save")
				}
				effective = next
			}
			lease, err := manager.AcquireS3(context.Background(), effective, true)
			if err != nil {
				t.Fatalf("completed save left external S3 blocked: %v", err)
			}
			lease.Release()
		})
	}
}

func TestManagedS3ProbeReportsMissingReadGrantWithoutExternalFallback(t *testing.T) {
	cfg := &config.Config{}
	cfg.S3.Enabled = true
	cfg.S3.Provider = config.S3ProviderRustFS
	cfg.RustFS.Enabled = true
	cfg.AcquireS3 = func(ctx context.Context, _ *config.Config, _ bool) (*config.RustFSS3Lease, error) {
		return &config.RustFSS3Lease{Context: ctx, Managed: true, Endpoint: "http://127.0.0.1:1", Region: "us-east-1", Bucket: "aurago", AccessKey: "probe-child", SecretKey: "probe-child-secret", Check: ctx.Err, Release: func() {}, Publish: func(fn func() error) error { return fn() }}, nil
	}
	s := &Server{Cfg: cfg}
	w := httptest.NewRecorder()
	handleS3Test(s)(w, httptest.NewRequest(http.MethodPost, "/api/s3/test", nil))
	if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), "read_verification_unavailable") {
		t.Fatalf("%d: %s", w.Code, w.Body.String())
	}
	result := s.executeSelfTestProbe(context.Background(), selftest.Case{Tool: "integration:s3"}, nil)
	if result.Verdict != "BLOCKED_ACCESS" || result.Code != "read_verification_unavailable" {
		t.Fatalf("unexpected self-test: %+v", result)
	}
}

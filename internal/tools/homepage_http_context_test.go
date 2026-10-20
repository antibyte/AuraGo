package tools

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHomepageBrowserChecksUseExplicitContext(t *testing.T) {
	original := homepageDockerExecFunc
	t.Cleanup(func() { homepageDockerExecFunc = original })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	homepageDockerExecFunc = func(cfg DockerConfig, _, _, _ string) string {
		calls++
		if cfg.Context != ctx || cfg.Context.Err() != context.Canceled {
			t.Fatal("Docker call lost the explicit cancelled context")
		}
		return `{"status":"cancelled"}`
	}
	HomepageScreenshot(ctx, HomepageConfig{}, "https://example.com", "", nil)
	HomepageCheckJS(ctx, HomepageConfig{}, "https://example.com", nil)
	if calls != 2 {
		t.Fatalf("Docker calls = %d, want 2", calls)
	}
}

type homepageObservationTransport func(*http.Request) (*http.Response, error)

func (fn homepageObservationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type homepageObservationBody struct {
	read func([]byte) (int, error)
}

func (b homepageObservationBody) Read(p []byte) (int, error) { return b.read(p) }
func (homepageObservationBody) Close() error                 { return nil }

func TestHomepageRemoteObservationDoesNotPersistCancellationOrPartialHash(t *testing.T) {
	originalConfiguredClient := homepageConfiguredProbeClient
	t.Cleanup(func() { homepageConfiguredProbeClient = originalConfiguredClient })
	homepageConfiguredProbeClient = func(string) (*http.Client, error) { return homepageVerifyHTTPClient, nil }

	original := homepageVerifyHTTPClient
	t.Cleanup(func() { homepageVerifyHTTPClient = original })
	for _, mode := range []string{"headers_cancelled", "body_cancelled", "body_error"} {
		t.Run(mode, func(t *testing.T) {
			db := newHomepageLedgerTestDB(t)
			project, err := EnsureHomepageProjectForDir(db, HomepageConfig{WorkspacePath: t.TempDir()}, "site", "Site", "html")
			if err != nil {
				t.Fatal(err)
			}
			if err := RecordHomepageDeployment(db, HomepageDeploymentRecord{ProjectID: project.ID, Provider: "local", URL: "https://example.com", BuildDir: ".", Status: "ok"}); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			homepageVerifyHTTPClient = &http.Client{Transport: homepageObservationTransport(func(req *http.Request) (*http.Response, error) {
				if req.Context() != ctx {
					t.Fatal("observation lost its owner context")
				}
				if mode == "headers_cancelled" {
					cancel()
					return nil, context.Canceled
				}
				body := homepageObservationBody{read: func(p []byte) (int, error) {
					n := copy(p, "partial document")
					if mode == "body_cancelled" {
						cancel()
						return n, context.Canceled
					}
					return n, io.ErrUnexpectedEOF
				}}
				return &http.Response{StatusCode: http.StatusOK, Body: body, Header: make(http.Header)}, nil
			})}
			status, _ := observeHomepageRemote(db, project.ID, ctx)
			observations, err := ListHomepageManagedRemoteObservations(db, project.ID, 10)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(mode, "cancelled") {
				if status != "" || len(observations) != 0 {
					t.Fatalf("cancelled observation persisted: %q %+v", status, observations)
				}
			} else if status != "remote_unknown" || len(observations) != 1 || observations[0].Status != "error" || observations[0].ContentHash != "" {
				t.Fatalf("partial body published as complete: %q %+v", status, observations)
			}
		})
	}
}

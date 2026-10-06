package invasion

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/dockerutil"
	"aurago/internal/testutil"
)

func TestEggIDPrefixKeepsUUIDPrefixAndRejectsUnsafeIDs(t *testing.T) {
	cases := []struct {
		id      string
		want    string
		wantErr bool
	}{
		{id: "12345678-abcd-ef12-3456-7890abcdef12", want: "12345678"},
		{id: "  7680f451-bad4-4908-92da-e286eb5f7c2a", want: "7680f451"},
		{id: "short", wantErr: true},
		{id: "", wantErr: true},
		{id: "abc;rm -rf /", wantErr: true},
		{id: "1234 678-abcd", wantErr: true},
	}
	for _, tc := range cases {
		got, err := eggIDPrefix(tc.id)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("eggIDPrefix(%q) = %q, want an error", tc.id, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("eggIDPrefix(%q) = %q, %v; want %q", tc.id, got, err, tc.want)
		}
	}
}

func TestDockerEggContainerNameKeepsHistoricUUIDNames(t *testing.T) {
	const nestID = "12345678-abcd-ef12-3456-7890abcdef12"
	name, err := dockerEggContainerName(nestID)
	if err != nil {
		t.Fatalf("dockerEggContainerName: %v", err)
	}
	if name != "aurago-egg-12345678" {
		t.Fatalf("container name = %q, want aurago-egg-12345678 so existing containers stay attached", name)
	}
	binds := dockerEggBinds(nestID)
	if len(binds) != 1 || binds[0] != "aurago-egg-12345678-log:/app/log" {
		t.Fatalf("binds = %#v, want the existing aurago-egg-12345678-log volume", binds)
	}
}

func TestDockerConnectorRejectsShortNestIDBeforeAnyEngineRequest(t *testing.T) {
	var requests atomic.Int64
	ts := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	nest := nestForMock(ts)
	nest.ID = "short"
	c := &DockerConnector{}
	ctx := context.Background()
	calls := map[string]func() error{
		"Deploy": func() error {
			return c.Deploy(ctx, nest, nil, EggDeployPayload{ConfigYAML: []byte("egg_mode: {}\n")})
		},
		"Stop": func() error { return c.Stop(ctx, nest, nil) },
		"Status": func() error {
			status, err := c.Status(ctx, nest, nil)
			if status != "unknown" {
				t.Errorf("Status = %q for an invalid nest ID, want unknown", status)
			}
			return err
		},
		"HealthCheck": func() error { return c.HealthCheck(ctx, nest, nil) },
		"Reconfigure": func() error { return c.Reconfigure(ctx, nest, nil, []byte("egg_mode: {}\n")) },
		"Rollback":    func() error { return c.Rollback(ctx, nest, nil) },
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			err := call() // a slice-bounds panic fails the test binary
			if err == nil || !strings.Contains(err.Error(), "invalid nest ID") {
				t.Fatalf("%s error = %v, want an invalid nest ID error", name, err)
			}
		})
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("engine received %d requests for an invalid nest ID, want 0", got)
	}
}

func TestDockerConnector_Deploy_PullStreamFailureLeavesRunningEggUntouched(t *testing.T) {
	cases := map[string]string{
		"error event":      "{\"status\":\"Pulling from antibyte/aurago\"}\n{\"errorDetail\":{\"message\":\"manifest unknown\"},\"error\":\"manifest unknown\"}\n",
		"truncated stream": "{\"status\":\"Pulling from antibyte/aurago\"}\n{\"status\":\"Downloading\",\"progressDetail\":{\"current\":1",
	}
	for name, stream := range cases {
		t.Run(name, func(t *testing.T) {
			var containerCalls atomic.Int64
			ts := mockDockerAPI(t, map[string]http.HandlerFunc{
				"/images/create": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = io.WriteString(w, stream)
				},
				"/containers/": func(w http.ResponseWriter, r *http.Request) {
					containerCalls.Add(1)
					w.WriteHeader(http.StatusNoContent)
				},
			})
			defer ts.Close()

			err := (&DockerConnector{}).Deploy(context.Background(), nestForMock(ts), nil, EggDeployPayload{ConfigYAML: []byte("egg_mode: {}\n")})
			if err == nil {
				t.Fatal("Deploy succeeded although the pull stream failed")
			}
			if name == "error event" && !strings.Contains(err.Error(), "manifest unknown") {
				t.Fatalf("error = %v, want the stream's manifest unknown message", err)
			}
			if got := containerCalls.Load(); got != 0 {
				t.Fatalf("Deploy sent %d container requests after a failed pull, want 0: the running egg must stay untouched", got)
			}
		})
	}
}

func TestDockerConnector_PullImageEscapesImageReference(t *testing.T) {
	var rawQuery, fromImage atomic.Value
	ts := mockDockerAPI(t, map[string]http.HandlerFunc{
		"/images/create": func(w http.ResponseWriter, r *http.Request) {
			rawQuery.Store(r.URL.RawQuery)
			fromImage.Store(r.URL.Query().Get("fromImage"))
			_, _ = io.WriteString(w, "{\"status\":\"Status: Image is up to date\"}\n")
		},
	})
	defer ts.Close()

	if err := (&DockerConnector{}).pullImage(context.Background(), nestForMock(ts), nil, "ghcr.io/antibyte/aurago:latest"); err != nil {
		t.Fatalf("pullImage: %v", err)
	}
	if got, _ := fromImage.Load().(string); got != "ghcr.io/antibyte/aurago:latest" {
		t.Fatalf("fromImage = %q, want ghcr.io/antibyte/aurago:latest", got)
	}
	if got, _ := rawQuery.Load().(string); got != "fromImage=ghcr.io%2Fantibyte%2Faurago%3Alatest" {
		t.Fatalf("raw query = %q, want the escaped image reference", got)
	}
}

func TestDockerConnector_ErrorBodiesAreBounded(t *testing.T) {
	huge := strings.Repeat("x", 1<<20)
	ts := mockDockerAPI(t, map[string]http.HandlerFunc{
		"/containers/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, huge)
		},
	})
	defer ts.Close()

	err := (&DockerConnector{}).Stop(context.Background(), nestForMock(ts), nil)
	if err == nil {
		t.Fatal("Stop succeeded against a failing engine")
	}
	if len(err.Error()) > dockerutil.MaxErrorBody+256 {
		t.Fatalf("error message is %d bytes, want at most MaxErrorBody plus context", len(err.Error()))
	}
}

func TestDockerConnector_StatusBoundsInspectBody(t *testing.T) {
	ts := mockDockerAPI(t, map[string]http.HandlerFunc{
		"/containers/": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, `{"State":{"Status":"running","Running":true},"Pad":"`)
			_, _ = io.WriteString(w, strings.Repeat("a", dockerInspectBodyLimit))
			_, _ = io.WriteString(w, `"}`)
		},
	})
	defer ts.Close()

	status, err := (&DockerConnector{}).Status(context.Background(), nestForMock(ts), nil)
	if err == nil || status != "unknown" {
		t.Fatalf("Status = %q, %v; want unknown with a decode error for an inspect body beyond %d bytes", status, err, dockerInspectBodyLimit)
	}
}

func TestDockerConnector_Deploy_PullStreamFailureFallsBackToCachedImage(t *testing.T) {
	var inspected, created atomic.Bool
	ts := mockDockerAPI(t, map[string]http.HandlerFunc{
		"/images/create": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "{\"status\":\"Pulling from antibyte/aurago\"}\n{\"errorDetail\":{\"message\":\"unexpected EOF\"},\"error\":\"unexpected EOF\"}\n")
		},
		"/images/ghcr.io/antibyte/aurago:latest/json": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("image inspect method = %s, want GET", r.Method)
			}
			inspected.Store(true)
			_, _ = io.WriteString(w, `{"Id":"sha256:cached"}`)
		},
		"/containers/create": func(w http.ResponseWriter, r *http.Request) {
			created.Store(true)
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"Id":"abc123"}`)
		},
		"/containers/": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPut {
				w.WriteHeader(http.StatusOK)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer ts.Close()

	err := (&DockerConnector{}).Deploy(context.Background(), nestForMock(ts), nil, EggDeployPayload{ConfigYAML: []byte("egg_mode: {}\n")})
	if err != nil {
		t.Fatalf("Deploy = %v; want the pre-K15 behaviour of deploying the image the Engine already holds", err)
	}
	if !inspected.Load() {
		t.Fatal("Deploy did not check whether the Engine already holds the image")
	}
	if !created.Load() {
		t.Fatal("Deploy did not create the egg container from the cached image")
	}
}

func TestDockerConnector_Deploy_PullStreamFailureWithFailingImageCheckLeavesRunningEggUntouched(t *testing.T) {
	var containerCalls atomic.Int64
	ts := mockDockerAPI(t, map[string]http.HandlerFunc{
		"/images/create": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "{\"errorDetail\":{\"message\":\"unexpected EOF\"},\"error\":\"unexpected EOF\"}\n")
		},
		"/images/ghcr.io/antibyte/aurago:latest/json": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, "engine busy")
		},
		"/containers/": func(w http.ResponseWriter, r *http.Request) {
			containerCalls.Add(1)
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer ts.Close()

	err := (&DockerConnector{}).Deploy(context.Background(), nestForMock(ts), nil, EggDeployPayload{ConfigYAML: []byte("egg_mode: {}\n")})
	if err == nil {
		t.Fatal("Deploy succeeded although the pull stream failed and the image check failed")
	}
	if !strings.Contains(err.Error(), "unexpected EOF") {
		t.Fatalf("error = %v, want the stream's pull error", err)
	}
	if got := containerCalls.Load(); got != 0 {
		t.Fatalf("Deploy sent %d container requests, want 0: the running egg must stay untouched", got)
	}
}

func TestDockerConnector_Deploy_HTTPPullFailureNeverFallsBackToCachedImage(t *testing.T) {
	var imageChecks, containerCalls atomic.Int64
	ts := mockDockerAPI(t, map[string]http.HandlerFunc{
		"/images/create": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"message":"manifest unknown"}`)
		},
		"/images/ghcr.io/antibyte/aurago:latest/json": func(w http.ResponseWriter, r *http.Request) {
			imageChecks.Add(1)
			_, _ = io.WriteString(w, `{"Id":"sha256:cached"}`)
		},
		"/containers/": func(w http.ResponseWriter, r *http.Request) {
			containerCalls.Add(1)
			w.WriteHeader(http.StatusNoContent)
		},
	})
	defer ts.Close()

	err := (&DockerConnector{}).Deploy(context.Background(), nestForMock(ts), nil, EggDeployPayload{ConfigYAML: []byte("egg_mode: {}\n")})
	if err == nil {
		t.Fatal("Deploy succeeded although the pull failed with an HTTP error status")
	}
	if got := imageChecks.Load(); got != 0 {
		t.Fatalf("Deploy checked for a cached image %d times after an HTTP pull failure, want 0: only in-stream failures fall back", got)
	}
	if got := containerCalls.Load(); got != 0 {
		t.Fatalf("Deploy sent %d container requests after an HTTP pull failure, want 0: the running egg must stay untouched", got)
	}
}

func TestDockerConnector_UnbuildableRequestsReturnErrorsInsteadOfPanicking(t *testing.T) {
	nest := NestRecord{ID: "12345678-abcd-ef12-3456-7890abcdef12", Host: "bad host", DeployMethod: "docker_remote"}
	c := &DockerConnector{}
	if err := c.Rollback(context.Background(), nest, nil); err == nil || !strings.Contains(err.Error(), "failed to create backup check request") {
		t.Fatalf("Rollback error = %v, want a backup check request error", err)
	}
	if err := c.renameContainer(context.Background(), nest, nil, "aurago-egg-12345678", "aurago-egg-12345678-prev"); err == nil {
		t.Fatal("renameContainer succeeded with an unbuildable request URL")
	}
}

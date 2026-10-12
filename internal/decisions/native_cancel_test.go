package decisions

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const nativeDiscoveryHelperEnv = "AURAGO_DECISIONS_DISCOVERY_TEST_HELPER"
const nativeDiscoveryMarkerEnv = "AURAGO_DECISIONS_DISCOVERY_TEST_MARKER"

func TestMain(m *testing.M) {
	if os.Getenv(nativeDiscoveryHelperEnv) == "1" {
		if err := os.WriteFile(os.Getenv(nativeDiscoveryMarkerEnv), []byte("started"), 0o600); err != nil {
			os.Exit(2)
		}
		for {
			time.Sleep(time.Hour)
		}
	}
	os.Exit(m.Run())
}

func TestNativeDiscoveryPreservesPreexistingCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := discoverNativeDevices(ctx, t.TempDir(), t.TempDir(), "model.gguf")
	if ErrorCode(err) != CodeCancelled || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled native discovery error = %v (cause %v), want canceled", err, errors.Unwrap(err))
	}
}

func TestNativeDiscoveryPreservesCancellationWhileProbeRuns(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "probe-started")
	t.Setenv(nativeDiscoveryHelperEnv, "1")
	t.Setenv(nativeDiscoveryMarkerEnv, marker)
	managedTarget := filepath.Join(t.TempDir(), runtimeTarget(runtime.GOOS, runtime.GOARCH))
	backendDir := filepath.Join(managedTarget, runtimeBackends(runtime.GOOS)[0])
	if err := os.MkdirAll(backendDir, 0o700); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	writeTestRuntimeBundle(t, backendDir, runtimeBackends(runtime.GOOS)[0], binary)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sourceRoot := t.TempDir()
	result := make(chan error, 1)
	go func() {
		_, err := discoverNativeDevices(ctx, filepath.Dir(managedTarget), sourceRoot, "model.gguf")
		result <- err
	}()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("native discovery helper did not enter the running probe")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-result:
		if ErrorCode(err) != CodeCancelled || !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled running discovery error = %v (cause %v), want cancelled", err, errors.Unwrap(err))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("native discovery did not return after cancellation")
	}
}

func TestNativeCancelUsesAuthenticatedRequestIDEndpoint(t *testing.T) {
	token := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(token, []byte("secret-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	called := false
	session := &nativeRuntimeSession{
		baseURL: "http://127.0.0.1:18765", token: token,
		client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			called = true
			if request.Method != http.MethodPost || request.URL.Path != "/cancel" {
				t.Errorf("cancel request = %s %s", request.Method, request.URL.Path)
			}
			if request.Header.Get("Authorization") != "Bearer secret-token" {
				t.Errorf("Authorization header = %q", request.Header.Get("Authorization"))
			}
			var body struct {
				RequestID string `json:"request_id"`
			}
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Errorf("decode cancel request: %v", err)
			} else if body.RequestID != "0123456789abcdef0123456789abcdef" {
				t.Errorf("request_id = %q", body.RequestID)
			}
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"cancelled":true}`)), Request: request}, nil
		})},
	}
	session.cancelRequest("0123456789abcdef0123456789abcdef")
	if !called {
		t.Fatal("native cancel endpoint was not called")
	}
}

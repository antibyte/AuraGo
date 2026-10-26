package decisions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"aurago/internal/config"
)

func TestDockerCloseConfirmsRemovalAndRetainsFailedSession(t *testing.T) {
	for _, failRemove := range []bool{false, true} {
		docker := newDecisionsDocker("")
		removes, cleanups := 0, 0
		docker.client.HTTPClient().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
			status := http.StatusInternalServerError
			switch {
			case strings.HasSuffix(r.URL.Path, "/stop"): // A successful forced removal supersedes this failure.
			case r.Method == http.MethodDelete:
				removes++
				if !failRemove || removes > 1 {
					status = http.StatusNoContent
				}
			case strings.HasSuffix(r.URL.Path, "/json"):
				status = http.StatusNotFound
			default:
				t.Fatalf("unexpected request: %s", r.URL)
			}
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
		})
		session := &dockerRuntimeSession{nativeRuntimeSession: &nativeRuntimeSession{}, docker: docker, containerID: "owned", cleanup: func(_ context.Context) error { cleanups++; return nil }}
		err := session.Close(t.Context())
		if failRemove {
			if err == nil || session.containerID == "" || cleanups != 0 {
				t.Fatal("failed removal lost ownership")
			}
			if err := session.Close(t.Context()); err != nil {
				t.Fatal(err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
		if session.containerID != "" || cleanups != 1 {
			t.Fatal("confirmed removal did not settle cleanup exactly once")
		}
	}
}

func TestDockerCloseRejectsUnconfirmedRemoval(t *testing.T) {
	docker := newDecisionsDocker("")
	docker.client.HTTPClient().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		status := http.StatusOK
		if r.Method == http.MethodDelete {
			status = http.StatusNoContent
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"State":{"Running":true}}`)), Header: make(http.Header)}, nil
	})
	session := &dockerRuntimeSession{nativeRuntimeSession: &nativeRuntimeSession{}, docker: docker, containerID: "still-running"}
	if err := session.Close(t.Context()); err == nil || session.containerID == "" {
		t.Fatal("removal response was trusted without confirming container absence")
	}
}

func TestDockerCloseCompletesAndRetriesNetworkCleanup(t *testing.T) {
	for _, mode := range []string{"slow", "failure", "already gone"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				docker := newDecisionsDocker("")
				attempts, containers, removed := 0, 0, false
				docker.client.HTTPClient().Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
					status, body := http.StatusNoContent, "{}"
					switch {
					case strings.HasSuffix(request.URL.Path, "/networks/create"):
						status, body = http.StatusCreated, `{"Id":"owned-network"}`
					case strings.HasSuffix(request.URL.Path, "/stop"):
						time.Sleep(2800 * time.Millisecond)
					case request.Method == http.MethodDelete && strings.Contains(request.URL.Path, "/containers/"):
						containers++
						time.Sleep(1200 * time.Millisecond)
					case strings.HasSuffix(request.URL.Path, "/json"):
						time.Sleep(800 * time.Millisecond)
						status = http.StatusNotFound
					case request.Method == http.MethodDelete && strings.Contains(request.URL.Path, "/networks/"):
						attempts++
						if mode == "failure" && attempts == 1 {
							status = http.StatusInternalServerError
						} else if mode == "already gone" {
							status, removed = http.StatusNotFound, true
						} else {
							select {
							case <-time.After(time.Second):
								removed = true
							case <-request.Context().Done():
								return nil, request.Context().Err()
							}
						}
					default:
						t.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
						return nil, fmt.Errorf("unexpected Docker request")
					}
					return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
				})
				root := t.TempDir()
				token, err := writeRuntimeToken(root)
				if err != nil {
					t.Fatal(err)
				}
				_, _, _, _, _, cleanup, err := prepareDockerRuntime(t.Context(), docker, RuntimeAccess{}, modelPaths{Model: filepath.Join(root, "models", "model.gguf")}, token, decisionsDockerPort, "cpu", nil)
				if err != nil {
					t.Fatal(err)
				}
				session := &dockerRuntimeSession{nativeRuntimeSession: &nativeRuntimeSession{token: token}, docker: docker, containerID: "owned", cleanup: cleanup}
				manager := readyManagerForTest(t, validTestSession())
				manager.session = session
				started := time.Now()
				err = manager.Stop(t.Context())
				if mode == "failure" {
					if ErrorCode(err) != CodeRuntimeUnavailable || !manager.cleanupIsPending() || manager.Status().State != "stopped_with_error" {
						t.Fatalf("failed network cleanup lost ownership: %v, status=%+v", err, manager.Status())
					}
					if _, err := os.Stat(token); err != nil || session.containerID != "" {
						t.Fatalf("container/token cleanup state: %v", err)
					}
					if err := manager.Start(t.Context()); ErrorCode(err) != CodeLifecycleBusy {
						t.Fatalf("start bypassed pending network cleanup: %v", err)
					}
					if err := manager.Stop(t.Context()); err != nil {
						t.Fatal(err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				if !removed || manager.cleanupIsPending() || manager.Status().State != "stopped" || containers != 1 {
					t.Fatalf("incomplete teardown: removed=%v containers=%d attempts=%d", removed, containers, attempts)
				}
				if mode == "slow" && time.Since(started) != 5800*time.Millisecond {
					t.Fatalf("network cleanup lost the remaining manager budget: %v", time.Since(started))
				}
				if _, err := os.Stat(token); !os.IsNotExist(err) {
					t.Fatalf("token retained after completed cleanup: %v", err)
				}
				before := attempts
				if err := session.Close(t.Context()); err != nil || attempts != before {
					t.Fatalf("completed cleanup repeated: %v", err)
				}
			})
		})
	}
}

func TestDockerFailedStartRetainsNetworkCleanup(t *testing.T) {
	for _, stage := range []string{"connect", "create", "start"} {
		t.Run(stage, func(t *testing.T) {
			data := t.TempDir()
			imageID := "sha256:" + strings.Repeat("a", 64)
			var networkRemoves atomic.Int32
			engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/version":
					json.NewEncoder(w).Encode(map[string]string{"ApiVersion": "1.45", "MinAPIVersion": "1.25"})
				case strings.Contains(r.URL.Path, "/images/"):
					json.NewEncoder(w).Encode(map[string]any{"Id": imageID, "Os": "linux", "Architecture": runtime.GOARCH, "Config": map[string]any{"Labels": map[string]string{
						"org.aurago.decisions.protocol": "v1", "org.aurago.decisions.backend": "vulkan", "org.aurago.decisions.llama_cpp_patch_sha256": llamaCPPPatchSHA256, "org.opencontainers.image.source": "https://github.com/antibyte/AuraGo", "org.opencontainers.image.revision": llamaCPPCommit,
					}}})
				case strings.HasSuffix(r.URL.Path, "/containers/self/json"):
					json.NewEncoder(w).Encode(map[string]any{"Id": "self", "State": map[string]bool{"Running": true}, "Mounts": []map[string]string{{"Type": "bind", "Source": filepath.ToSlash(data), "Destination": filepath.ToSlash(data)}}})
				case strings.HasSuffix(r.URL.Path, "/networks/create"):
					json.NewEncoder(w).Encode(map[string]string{"Id": "owned-network"})
				case strings.HasSuffix(r.URL.Path, "/connect") && stage == "connect", strings.HasSuffix(r.URL.Path, "/containers/create") && stage == "create", strings.HasSuffix(r.URL.Path, "/start"), strings.HasSuffix(r.URL.Path, "/disconnect"):
					w.WriteHeader(http.StatusInternalServerError)
				case strings.HasSuffix(r.URL.Path, "/connect"), strings.HasSuffix(r.URL.Path, "/stop"):
					w.WriteHeader(http.StatusNoContent)
				case strings.HasSuffix(r.URL.Path, "/containers/create"):
					json.NewEncoder(w).Encode(map[string]string{"Id": "owned"})
				case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/containers/owned"):
					w.WriteHeader(http.StatusNoContent)
				case strings.HasSuffix(r.URL.Path, "/containers/owned/json"):
					w.WriteHeader(http.StatusNotFound)
				case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/networks/owned-network"):
					if networkRemoves.Add(1) == 1 {
						w.WriteHeader(http.StatusInternalServerError)
					} else {
						w.WriteHeader(http.StatusNoContent)
					}
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer engine.Close()
			access := RuntimeAccess{IsDocker: true, DockerEnabled: true, DockerHost: strings.Replace(engine.URL, "http://", "tcp://", 1)}
			device := runtimeDevice{ID: "gpu", Backend: "vulkan", Type: "GPU", DockerImageID: imageID}
			session, err := startDockerRuntime(t.Context(), device, modelPaths{Model: filepath.Join(data, "decisions", "models", "model.gguf")}, config.DecisionsConfig{}, access, data, func(context.Context, string) (string, error) { return "self", nil }, nil)
			if ErrorCode(err) != CodeRuntimeStartFailed || session == nil || networkRemoves.Load() != 1 {
				t.Fatalf("failed start lost network ownership: %v, session=%v, attempts=%d", err, session != nil, networkRemoves.Load())
			}
			if err := session.Close(t.Context()); err != nil || networkRemoves.Load() != 2 {
				t.Fatalf("network cleanup retry: %v, attempts=%d", err, networkRemoves.Load())
			}
		})
	}
}

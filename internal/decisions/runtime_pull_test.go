package decisions

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRuntimePullUsesPinnedDigestAndReportsStreamFailure(t *testing.T) {
	previous := releaseRuntimeCatalog
	defer func() { releaseRuntimeCatalog = previous }()
	reference := "ghcr.io/antibyte/aurago-decisions-runtime@sha256:" + strings.Repeat("a", 64)
	releaseRuntimeCatalog = runtimeCatalog{Images: []runtimeImage{{Backend: "vulkan", Reference: reference}}}
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "stream_error"}[fail], func(t *testing.T) {
			pulled, pulls, progress := false, 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/version":
					w.Write([]byte(`{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`))
				case strings.HasSuffix(r.URL.Path, "/info"):
					w.Write([]byte(`{"Runtimes":{}}`))
				case strings.HasSuffix(r.URL.Path, "/images/create"):
					pulls++
					if r.Method != "POST" || r.URL.Query().Get("fromImage") != reference {
						t.Errorf("unpinned pull: %s", r.URL)
					}
					w.Write([]byte("{\"id\":\"layer\",\"status\":\"Downloading\",\"progressDetail\":{\"current\":5,\"total\":10}}\n"))
					if fail {
						w.Write([]byte("{\"errorDetail\":{\"message\":\"denied\"}}\n"))
					} else {
						pulled = true
					}
				case strings.Contains(r.URL.Path, "/images/") && strings.HasSuffix(r.URL.Path, "/json"):
					if !pulled {
						http.NotFound(w, r)
						return
					}
					var metadata dockerImageMetadata
					metadata.ID = "sha256:" + strings.Repeat("b", 64)
					metadata.OS = "linux"
					metadata.Architecture = runtime.GOARCH
					metadata.Config.Labels = map[string]string{"org.aurago.decisions.protocol": "v1", "org.aurago.decisions.backend": "vulkan", "org.aurago.decisions.llama_cpp_patch_sha256": llamaCPPPatchSHA256, "org.opencontainers.image.source": "https://github.com/antibyte/AuraGo", "org.opencontainers.image.revision": llamaCPPCommit}
					json.NewEncoder(w).Encode(metadata)
				default:
					t.Errorf("unexpected Docker operation: %s %s", r.Method, r.URL)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			err := ensureDockerRuntimeImages(t.Context(), RuntimeAccess{DockerEnabled: true, DockerHost: strings.Replace(server.URL, "http://", "tcp://", 1)}, false, func(phase string, current, total int64) {
				if current == 5 && total == 10 {
					progress++
				}
			})
			if fail && ErrorCode(err) != "runtime_download_failed" || !fail && err != nil {
				t.Fatalf("pull error: %v", err)
			}
			if pulls != 1 || progress != 1 {
				t.Fatalf("pulls=%d progress=%d", pulls, progress)
			}
		})
	}
}

func TestPublishedCatalogPreservesVerifiedLocalDockerImages(t *testing.T) {
	previous := releaseRuntimeCatalog
	defer func() { releaseRuntimeCatalog = previous }()
	reference := "ghcr.io/antibyte/aurago-decisions-runtime@sha256:" + strings.Repeat("a", 64)
	releaseRuntimeCatalog = runtimeCatalog{Images: []runtimeImage{{Backend: "vulkan", Reference: reference}}}
	for _, test := range []struct {
		name          string
		pinnedStatus  int
		invalidPinned bool
		invalidLocal  bool
		wantLegacy    bool
		wantError     string
	}{
		{name: "published_preferred", pinnedStatus: http.StatusOK},
		{name: "local_reused", pinnedStatus: http.StatusNotFound, wantLegacy: true},
		{name: "invalid_local_rejected", pinnedStatus: http.StatusNotFound, invalidLocal: true, wantLegacy: true, wantError: CodeRuntimeAttestationFailed},
		{name: "invalid_published_rejected", pinnedStatus: http.StatusOK, invalidPinned: true, wantError: CodeRuntimeAttestationFailed},
		{name: "inspection_denied", pinnedStatus: http.StatusForbidden, wantError: CodeDockerUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			var usedLegacy atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/version":
					w.Write([]byte(`{"ApiVersion":"1.45","MinAPIVersion":"1.25"}`))
				case strings.HasSuffix(r.URL.Path, "/info"):
					w.Write([]byte(`{"Runtimes":{}}`))
				case strings.HasSuffix(r.URL.Path, "/images/"+reference+"/json"), strings.HasSuffix(r.URL.Path, "/images/aurago-decisions-runtime:vulkan/json"):
					legacy := strings.HasSuffix(r.URL.Path, "/images/aurago-decisions-runtime:vulkan/json")
					if legacy {
						usedLegacy.Store(true)
					}
					if !legacy && test.pinnedStatus != http.StatusOK {
						http.Error(w, "unavailable", test.pinnedStatus)
						return
					}
					var metadata dockerImageMetadata
					metadata.ID = "sha256:" + strings.Repeat("b", 64)
					metadata.OS, metadata.Architecture = "linux", runtime.GOARCH
					metadata.Config.Labels = map[string]string{"org.aurago.decisions.protocol": "v1", "org.aurago.decisions.backend": "vulkan", "org.aurago.decisions.llama_cpp_patch_sha256": llamaCPPPatchSHA256, "org.opencontainers.image.source": "https://github.com/antibyte/AuraGo", "org.opencontainers.image.revision": llamaCPPCommit}
					if legacy && test.invalidLocal || !legacy && test.invalidPinned {
						metadata.Config.Labels["org.opencontainers.image.revision"] = "unverified"
					}
					json.NewEncoder(w).Encode(metadata)
				default:
					t.Errorf("unexpected Docker operation: %s %s", r.Method, r.URL)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			host := strings.Replace(server.URL, "http://", "tcp://", 1)
			err := ensureDockerRuntimeImages(t.Context(), RuntimeAccess{DockerEnabled: true, DockerHost: host}, false, nil)
			if ErrorCode(err) != test.wantError {
				t.Fatalf("setup = %v; want %q", err, test.wantError)
			}
			if err == nil {
				docker := newDecisionsDocker(host)
				defer docker.client.CloseIdleConnections()
				if _, err := docker.image(t.Context(), "vulkan"); err != nil {
					t.Fatalf("runtime image lookup after setup: %v", err)
				}
			}
			if usedLegacy.Load() != test.wantLegacy {
				t.Fatalf("legacy image inspected = %v; want %v", usedLegacy.Load(), test.wantLegacy)
			}
		})
	}
}

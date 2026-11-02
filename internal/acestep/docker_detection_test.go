package acestep

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/dockerutil"
)

func TestDetectPullsOnlySelectedBackends(t *testing.T) {
	vulkan := Device{ID: "vulkan:0", Backend: "vulkan", FreeGB: 12, Verified: true}
	cuda := Device{ID: "cuda:0", Backend: "cuda", FreeGB: 16, Verified: true}
	rocm := Device{ID: "rocm:0", Backend: "rocm", FreeGB: 12, Verified: true}
	xpu := Device{ID: "xpu:0", Backend: "xpu", FreeGB: 12, Verified: true}
	unverified := vulkan
	unverified.Verified = false
	lowMemory := vulkan
	lowMemory.FreeGB = 4
	for _, tc := range []struct {
		name, backend, selected, kernel string
		nvidia                          bool
		devices                         []Device
		wantBackends                    []string
		wantDevice                      string
	}{
		{name: "amd_auto", devices: []Device{vulkan}, wantBackends: []string{"vulkan"}, wantDevice: "vulkan:0"},
		{name: "nvidia_auto", nvidia: true, devices: []Device{cuda, vulkan}, wantBackends: []string{"cuda", "vulkan"}, wantDevice: "cuda:0"},
		{name: "largest_free_memory", nvidia: true, devices: []Device{{ID: "cuda:0", Backend: "cuda", FreeGB: 8, Verified: true}, vulkan}, wantBackends: []string{"cuda", "vulkan"}, wantDevice: "vulkan:0"},
		{name: "wsl_cuda", kernel: "microsoft-standard-WSL2", devices: []Device{cuda}, wantBackends: []string{"cuda"}, wantDevice: "cuda:0"},
		{name: "no_gpu_no_cpu_fallback", wantBackends: []string{"vulkan"}},
		{name: "unverified_gpu", devices: []Device{unverified}, wantBackends: []string{"vulkan"}},
		{name: "insufficient_free_memory", devices: []Device{lowMemory}, wantBackends: []string{"vulkan"}},
		{name: "explicit_rocm", backend: "rocm", devices: []Device{rocm}, wantBackends: []string{"rocm"}, wantDevice: "rocm:0"},
		{name: "explicit_xpu", backend: "xpu", devices: []Device{xpu}, wantBackends: []string{"xpu"}, wantDevice: "xpu:0"},
		{name: "selected_rocm_device", selected: "rocm:0", devices: []Device{rocm}, wantBackends: []string{"rocm"}, wantDevice: "rocm:0"},
		{name: "selected_xpu_device", selected: "xpu:0", devices: []Device{xpu}, wantBackends: []string{"xpu"}, wantDevice: "xpu:0"},
		{name: "selected_vulkan_device", nvidia: true, selected: "vulkan:0", devices: []Device{cuda, vulkan}, wantBackends: []string{"vulkan"}, wantDevice: "vulkan:0"},
		{name: "explicit_cpu", backend: "cpu", devices: []Device{{ID: "cpu", Backend: "cpu", Verified: true}}, wantBackends: []string{"cpu"}, wantDevice: "cpu"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var pulled []string
			var probeBackend string
			created := false
			downloaded := map[string]bool{}
			images := manifest().Images
			backendFor := func(reference string) string {
				for backend, image := range images {
					if image == reference {
						return backend
					}
				}
				t.Errorf("unexpected image %q", reference)
				return ""
			}
			docker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				path := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/v1.45"), "/")
				switch {
				case path == "version":
					io.WriteString(w, `{"ApiVersion":"1.45"}`)
				case path == "info":
					runtimes := map[string]any{}
					if tc.nvidia {
						runtimes["nvidia"] = map[string]any{}
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"OSType": "linux", "Architecture": "amd64", "KernelVersion": tc.kernel, "Runtimes": runtimes})
				case path == "images/create" && r.Method == http.MethodPost:
					reference := r.URL.Query().Get("fromImage")
					pulled = append(pulled, backendFor(reference))
					downloaded[reference] = true
					io.WriteString(w, "{}\n")
				case strings.HasPrefix(path, "images/") && r.Method == http.MethodGet:
					reference := strings.TrimSuffix(strings.TrimPrefix(path, "images/"), "/json")
					if !downloaded[reference] {
						w.WriteHeader(http.StatusNotFound)
					} else {
						io.WriteString(w, "{}")
					}
				case path == "containers/create" && r.Method == http.MethodPost:
					var spec struct{ Image string }
					if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
						t.Error(err)
					}
					probeBackend, created = backendFor(spec.Image), true
					w.WriteHeader(http.StatusCreated)
					io.WriteString(w, `{"Id":"probe-id"}`)
				case strings.HasPrefix(path, "containers/") && strings.HasSuffix(path, "/json"):
					if !created {
						w.WriteHeader(http.StatusNotFound)
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"Id": "probe-id", "Config": map[string]any{"Labels": dockerutil.ManagedLabels(Owner, "music", "probe", "")}, "State": map[string]any{"Running": false, "ExitCode": 0}})
				case strings.HasSuffix(path, "/start") && r.Method == http.MethodPost:
					w.WriteHeader(http.StatusNoContent)
				case strings.HasSuffix(path, "/logs"):
					var devices []Device
					for _, device := range tc.devices {
						if device.Backend == probeBackend {
							devices = append(devices, device)
						}
					}
					payload, _ := json.Marshal(map[string]any{"devices": devices})
					header := make([]byte, 8)
					header[0] = 1
					binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
					_, _ = w.Write(append(header, payload...))
				case path == "containers/probe-id" && r.Method == http.MethodDelete:
					created = false
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected Docker request %s %s", r.Method, path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer docker.Close()
			m := testManager(t)
			m.docker = dockerutil.NewClient(strings.Replace(docker.URL, "http://", "tcp://", 1), time.Second)
			defer m.docker.CloseIdleConnections()
			want := m.want
			if tc.backend != "" {
				want.Local.Backend = tc.backend
			}
			if tc.selected != "" {
				want.Local.Device = tc.selected
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			device, err := m.detect(ctx, want)
			if tc.wantDevice == "" {
				if err == nil || err.Error() != "acestep_no_compatible_gpu" {
					t.Fatalf("wanted no compatible GPU, got %+v, %v", device, err)
				}
			} else if err != nil || device.ID != tc.wantDevice {
				t.Fatalf("device = %+v, %v; want %s", device, err, tc.wantDevice)
			}
			mu.Lock()
			actual := slices.Clone(pulled)
			mu.Unlock()
			if !slices.Equal(actual, tc.wantBackends) {
				t.Fatalf("downloaded backends %v; want %v", actual, tc.wantBackends)
			}
		})
	}
}

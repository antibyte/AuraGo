package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
	"aurago/internal/dockerutil"
	"aurago/internal/tools"
)

func TestZiroExportMountsOnlyThePrivateJobAndCleanupBypassesRevokedGrants(t *testing.T) {
	for _, mode := range []string{"native", "bind", "volume"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/version") {
					w.Write([]byte(`{"ApiVersion":"1.45","MinAPIVersion":"1.24"}`))
					return
				}
				calls++
				if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/containers/create") {
					var payload struct {
						Image      string
						HostConfig struct {
							NetworkMode    string
							ReadonlyRootfs bool
							Mounts         []struct {
								Type, Source, Target string
								VolumeOptions        struct{ Subpath string }
							}
						}
					}
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Fatal(err)
					}
					host := payload.HostConfig
					if payload.Image != desktop.ZiroExportImage || host.NetworkMode != "none" || !host.ReadonlyRootfs || len(host.Mounts) != 1 || host.Mounts[0].Target != "/job" {
						t.Fatal("worker isolation missing")
					}
					mount := host.Mounts[0]
					if mode == "bind" && mount.Source != "/host/data/ziro/jobs/job-1" {
						t.Fatal("bind mapping", mount)
					}
					if mode == "volume" && (mount.Source != "aurago-data" || mount.VolumeOptions.Subpath != "ziro/jobs/job-1") {
						t.Fatal("volume mapping", mount)
					}
					w.WriteHeader(201)
					w.Write([]byte(`{"Id":"test-container"}`))
					return
				}
				if r.Method != "DELETE" || r.URL.Query().Get("force") != "true" {
					t.Fatal("unexpected lifecycle request", r.URL)
				}
				w.WriteHeader(204)
			}))
			defer engine.Close()
			jobs := "/app/data/ziro/jobs"
			d := &ziroDocker{client: dockerutil.NewClient("tcp://"+strings.TrimPrefix(engine.URL, "http://"), 0), jobs: jobs}
			defer d.client.CloseIdleConnections()
			if mode != "native" {
				d.mountType, d.mountTarget, d.mountSource = mode, "/app/data", "/host/data"
			}
			if mode == "volume" {
				d.mountSource = "aurago-data"
			}
			name := "aurago-ziro-export-test"
			if _, err := d.CreateContainer(t.Context(), desktop.CodeDockerCreateRequest{Name: name, Image: desktop.ZiroExportImage, Volumes: []string{filepath.Join(jobs, "job-1") + ":/job"}}); err != nil {
				t.Fatal(err)
			}
			if err := d.ContainerAction(tools.WithRuntimePermissions(context.Background(), tools.RuntimePermissions{}), name, "remove"); err != nil {
				t.Fatal(err)
			}
			if _, err := d.CreateContainer(t.Context(), desktop.CodeDockerCreateRequest{Name: name, Image: desktop.ZiroExportImage, Volumes: []string{filepath.Join(jobs, "..", "other") + ":/job"}}); err == nil {
				t.Fatal("outside job accepted")
			}
			if calls != 2 {
				t.Fatal("unexpected Docker access", calls)
			}
		})
	}
}

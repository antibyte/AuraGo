package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestGo2RTCDockerAccessRequirements(t *testing.T) {
	previous, configured := currentRuntimePermissions()
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})
	t.Cleanup(func() {
		if configured {
			ConfigureRuntimePermissions(previous)
		} else {
			ClearRuntimePermissionsForTest()
		}
	})

	tests := []struct {
		name       string
		inDocker   bool
		denyPath   string
		denyMethod string
		wantCode   string
	}{
		{name: "all capabilities available"},
		{name: "containers denied", denyPath: "/containers/json", denyMethod: http.MethodGet, wantCode: "docker_containers_denied"},
		{name: "images denied", denyPath: "/images/json", denyMethod: http.MethodGet, wantCode: "docker_images_denied"},
		{name: "networks denied in Docker", inDocker: true, denyPath: "/networks", denyMethod: http.MethodGet, wantCode: "docker_networks_denied"},
		{name: "POST denied", denyPath: "/containers/aurago_go2rtc_capability_", denyMethod: http.MethodPost, wantCode: "docker_post_denied"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			postRequests := 0
			created := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := strings.TrimPrefix(r.URL.Path, "/"+dockerAPIVersion)
				if r.Method == http.MethodPost {
					postRequests++
					if strings.Contains(path, "/create") {
						created = true
					}
				}
				if r.Method == test.denyMethod && strings.HasPrefix(path, test.denyPath) {
					http.Error(w, "denied", http.StatusForbidden)
					return
				}
				if r.Method == http.MethodPost && strings.HasPrefix(path, "/containers/aurago_go2rtc_capability_") && strings.HasSuffix(path, "/start") {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`[]`))
			}))
			defer server.Close()

			cfg := &config.Config{}
			cfg.Docker.Host = "tcp://" + strings.TrimPrefix(server.URL, "http://")
			cfg.Runtime.IsDocker = test.inDocker
			manager := NewGo2RTCManager(cfg, nil, nil, nil)
			requirements := manager.DockerAccessRequirements(context.Background())
			if test.wantCode == "" {
				if len(requirements) != 0 {
					t.Fatalf("requirements = %#v, want none", requirements)
				}
			} else if !hasGo2RTCDockerRequirement(requirements, test.wantCode) {
				t.Fatalf("requirements = %#v, want code %q", requirements, test.wantCode)
			}
			if postRequests != 1 {
				t.Fatalf("POST requests = %d, want 1 sentinel start probe", postRequests)
			}
			if created {
				t.Fatal("capability probe attempted to create a Docker resource")
			}
		})
	}
}

func TestGo2RTCDockerAccessRequirementsReportsUnreachableEndpoint(t *testing.T) {
	previous, configured := currentRuntimePermissions()
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})
	t.Cleanup(func() {
		if configured {
			ConfigureRuntimePermissions(previous)
		} else {
			ClearRuntimePermissionsForTest()
		}
	})
	cfg := &config.Config{}
	cfg.Docker.Host = "tcp://127.0.0.1:1"
	manager := NewGo2RTCManager(cfg, nil, nil, nil)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	requirements := manager.DockerAccessRequirements(ctx)
	if !hasGo2RTCDockerRequirement(requirements, "docker_unreachable") {
		t.Fatalf("requirements = %#v, want docker_unreachable", requirements)
	}
}

func hasGo2RTCDockerRequirement(requirements []Go2RTCDockerRequirement, code string) bool {
	for _, requirement := range requirements {
		if requirement.Code == code {
			return true
		}
	}
	return false
}

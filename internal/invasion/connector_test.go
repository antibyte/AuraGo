package invasion

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetConnector_Docker(t *testing.T) {
	for _, method := range []string{"docker_remote", "docker_local"} {
		n := NestRecord{DeployMethod: method}
		c := GetConnector(n)
		if _, ok := c.(*DockerConnector); !ok {
			t.Errorf("GetConnector(%q) = %T, want *DockerConnector", method, c)
		}
	}
}

func TestGetConnector_SSH(t *testing.T) {
	for _, method := range []string{"ssh", "", "unknown"} {
		n := NestRecord{DeployMethod: method}
		c := GetConnector(n)
		if _, ok := c.(*SSHConnector); !ok {
			t.Errorf("GetConnector(%q) = %T, want *SSHConnector", method, c)
		}
	}
}

func TestDockerConnector_apiURL_Remote(t *testing.T) {
	c := &DockerConnector{}
	n := NestRecord{Host: "10.0.0.5", Port: 2376, DeployMethod: "docker_remote"}
	got := c.apiURL(n, "/version")
	want := fmt.Sprintf("http://10.0.0.5:2376/%s/version", dockerAPIVersion)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDockerConnector_apiURL_Remote_DefaultPort(t *testing.T) {
	c := &DockerConnector{}
	n := NestRecord{Host: "10.0.0.5", Port: 0, DeployMethod: "docker_remote"}
	got := c.apiURL(n, "/version")
	want := fmt.Sprintf("http://10.0.0.5:2375/%s/version", dockerAPIVersion)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDockerConnector_apiURL_Local(t *testing.T) {
	c := &DockerConnector{}
	n := NestRecord{DeployMethod: "docker_local"}
	got := c.apiURL(n, "/containers/json")
	want := fmt.Sprintf("http://localhost/%s/containers/json", dockerAPIVersion)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExtractYAMLField_TopLevel(t *testing.T) {
	data := []byte("name: test\nversion: 1.0\n")
	if v := extractYAMLField(data, "name"); v != "test" {
		t.Errorf("got %q", v)
	}
}

func TestExtractYAMLField_Nested(t *testing.T) {
	data := []byte("egg_mode:\n  master_url: ws://localhost:8080\n  egg_id: e1\n")
	if v := extractYAMLField(data, "master_url"); v != "ws://localhost:8080" {
		t.Errorf("got %q", v)
	}
	if v := extractYAMLField(data, "egg_id"); v != "e1" {
		t.Errorf("got %q", v)
	}
}

func TestExtractYAMLField_Missing(t *testing.T) {
	data := []byte("foo: bar\n")
	if v := extractYAMLField(data, "nonexistent"); v != "" {
		t.Errorf("expected empty, got %q", v)
	}
}

func TestExtractYAMLField_InvalidYAML(t *testing.T) {
	data := []byte(":::invalid:::")
	if v := extractYAMLField(data, "foo"); v != "" {
		t.Errorf("expected empty for invalid YAML, got %q", v)
	}
}

func TestExtractMasterURL(t *testing.T) {
	data := []byte("egg_mode:\n  master_url: wss://example.com/api/invasion/ws\n")
	got := extractMasterURL(data)
	if got != "wss://example.com/api/invasion/ws" {
		t.Errorf("got %q", got)
	}
}

func TestDockerLocalHostHonorsDockerHostEnv(t *testing.T) {
	t.Setenv("DOCKER_HOST", "npipe:////./pipe/docker_engine")

	got := dockerLocalHost()
	if got != "npipe:////./pipe/docker_engine" {
		t.Fatalf("dockerLocalHost() = %q, want npipe host from DOCKER_HOST", got)
	}
}

func TestDockerConnector_ConfigArchivePathMatchesEntrypoint(t *testing.T) {
	if dockerEggConfigArchivePath != "/app/data" {
		t.Fatalf("dockerEggConfigArchivePath = %q, want /app/data", dockerEggConfigArchivePath)
	}
	if dockerEggConfigFileName != "config.yaml" {
		t.Fatalf("dockerEggConfigFileName = %q, want config.yaml", dockerEggConfigFileName)
	}
}

func TestDockerConnector_ConfigArchiveIsReadableByContainerUser(t *testing.T) {
	cfg := []byte("egg_mode:\n  enabled: true\n")
	archive, err := buildDockerEggConfigArchive(cfg)
	if err != nil {
		t.Fatalf("buildDockerEggConfigArchive: %v", err)
	}

	tr := tar.NewReader(bytes.NewReader(archive))
	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("reading config tar header: %v", err)
	}
	if hdr.Name != dockerEggConfigFileName {
		t.Fatalf("tar entry name = %q, want %q", hdr.Name, dockerEggConfigFileName)
	}
	if hdr.Mode != 0600 {
		t.Fatalf("tar entry mode = %#o, want 0600", hdr.Mode)
	}
	if hdr.Uid != dockerEggConfigUID || hdr.Gid != dockerEggConfigGID {
		t.Fatalf("tar entry uid/gid = %d/%d, want %d/%d", hdr.Uid, hdr.Gid, dockerEggConfigUID, dockerEggConfigGID)
	}
	if hdr.Uname != dockerEggConfigUser || hdr.Gname != dockerEggConfigGroup {
		t.Fatalf("tar entry user/group = %q/%q, want %q/%q", hdr.Uname, hdr.Gname, dockerEggConfigUser, dockerEggConfigGroup)
	}
}

func TestDockerConnector_EggBindsDoNotReuseDataVolume(t *testing.T) {
	binds := dockerEggBinds("7680f451-bad4-4908-92da-e286eb5f7c2a")
	for _, bind := range binds {
		if strings.Contains(bind, ":/app/data") {
			t.Fatalf("docker egg binds must not reuse /app/data across hatches; got %q", bind)
		}
	}
}

func TestDockerConnector_RemoveContainerPathRemovesAnonymousVolumes(t *testing.T) {
	got := dockerRemoveContainerPath("aurago-egg-test")
	if !strings.Contains(got, "v=true") {
		t.Fatalf("remove path = %q, want v=true to remove anonymous /app/data volumes", got)
	}
}

func TestDockerEggCreateBodyUsesEggPortHealthcheck(t *testing.T) {
	body := dockerEggCreateBody("aurago:test", "12345678-abcd-ef12-3456-7890abcdef12", EggDeployPayload{
		ConfigYAML: []byte("server:\n  port: 8099\n"),
		EggPort:    8099,
	})
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal create body: %v", err)
	}

	var got struct {
		ExposedPorts map[string]struct{} `json:"ExposedPorts"`
		Healthcheck  struct {
			Test []string `json:"Test"`
		} `json:"Healthcheck"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode create body: %v", err)
	}
	if _, ok := got.ExposedPorts["8099/tcp"]; !ok {
		t.Fatalf("docker egg create body should expose 8099/tcp, got %#v", got.ExposedPorts)
	}
	if _, ok := got.ExposedPorts["8088/tcp"]; ok {
		t.Fatalf("docker egg create body should not expose the master's default HTTP port, got %#v", got.ExposedPorts)
	}
	if len(got.Healthcheck.Test) == 0 {
		t.Fatal("docker egg create body should override the image healthcheck")
	}
	healthcheck := strings.Join(got.Healthcheck.Test, " ")
	if !strings.Contains(healthcheck, "/app/data/config.yaml") || !strings.Contains(healthcheck, "/api/ready") {
		t.Fatalf("healthcheck should read the egg config and probe readiness, got %q", healthcheck)
	}
}

// ── Docker API mock tests ───────────────────────────────────────────────────

// mockDockerAPI creates an httptest.Server that emulates key Docker Engine endpoints.
func mockDockerAPI(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip API version prefix
		path := r.URL.Path
		parts := strings.SplitN(path, "/", 3)
		if len(parts) >= 3 {
			path = "/" + parts[2]
		}

		// Match handler
		for pattern, handler := range handlers {
			if strings.HasPrefix(path, pattern) {
				handler(w, r)
				return
			}
		}
		t.Logf("unhandled Docker API request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
}

// nestForMock creates a NestRecord pointing at the mock server.
func nestForMock(ts *httptest.Server) NestRecord {
	// Parse host:port from httptest URL
	addr := strings.TrimPrefix(ts.URL, "http://")
	host := addr
	port := 0
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		host = addr[:idx]
		fmt.Sscanf(addr[idx+1:], "%d", &port)
	}
	return NestRecord{
		ID:           "12345678-abcd-ef12-3456-7890abcdef12",
		Host:         host,
		Port:         port,
		DeployMethod: "docker_remote",
	}
}

func TestDockerConnector_Validate_OK(t *testing.T) {
	ts := mockDockerAPI(t, map[string]http.HandlerFunc{
		"/version": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]string{"Version": "24.0.0"})
		},
	})
	defer ts.Close()

	c := &DockerConnector{}
	err := c.Validate(context.Background(), nestForMock(ts), nil)
	if err != nil {
		t.Errorf("Validate should succeed: %v", err)
	}
}

func TestDockerConnector_Validate_Unreachable(t *testing.T) {
	c := &DockerConnector{}
	nest := NestRecord{
		ID:           "12345678-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
		Host:         "192.0.2.1", // TEST-NET, unreachable
		Port:         1,
		DeployMethod: "docker_remote",
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*1000*1000) // 50ms
	defer cancel()
	err := c.Validate(ctx, nest, nil)
	if err == nil {
		t.Error("Validate should fail for unreachable host")
	}
}

func TestDockerConnector_Status_Running(t *testing.T) {
	ts := mockDockerAPI(t, map[string]http.HandlerFunc{
		"/containers/": func(w http.ResponseWriter, r *http.Request) {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"State": map[string]interface{}{
					"Status":  "running",
					"Running": true,
				},
			})
		},
	})
	defer ts.Close()

	c := &DockerConnector{}
	status, err := c.Status(context.Background(), nestForMock(ts), nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != "running" {
		t.Errorf("status = %q, want running", status)
	}
}

func TestDockerConnector_Status_Stopped(t *testing.T) {
	ts := mockDockerAPI(t, map[string]http.HandlerFunc{
		"/containers/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
	})
	defer ts.Close()

	c := &DockerConnector{}
	status, err := c.Status(context.Background(), nestForMock(ts), nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != "stopped" {
		t.Errorf("status = %q, want stopped", status)
	}
}

func TestDockerConnector_Stop_OK(t *testing.T) {
	ts := mockDockerAPI(t, map[string]http.HandlerFunc{
		"/containers/": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "POST" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		},
	})
	defer ts.Close()

	c := &DockerConnector{}
	err := c.Stop(context.Background(), nestForMock(ts), nil)
	if err != nil {
		t.Errorf("Stop should succeed: %v", err)
	}
}

func TestDockerConnector_Stop_AlreadyStopped(t *testing.T) {
	ts := mockDockerAPI(t, map[string]http.HandlerFunc{
		"/containers/": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotModified) // 304 = already stopped
		},
	})
	defer ts.Close()

	c := &DockerConnector{}
	err := c.Stop(context.Background(), nestForMock(ts), nil)
	if err != nil {
		t.Errorf("Stop on already-stopped container should succeed: %v", err)
	}
}

func TestDockerConnector_Deploy_OK(t *testing.T) {
	configYAML := []byte("egg_mode:\n  master_url: ws://localhost:8080/api/invasion/ws\n  egg_id: e1\n  nest_id: n1\n")
	var archivePath string
	var createBody struct {
		ExposedPorts map[string]struct{} `json:"ExposedPorts"`
		Healthcheck  struct {
			Test []string `json:"Test"`
		} `json:"Healthcheck"`
		HostConfig struct {
			Binds []string `json:"Binds"`
		} `json:"HostConfig"`
	}

	ts := mockDockerAPI(t, map[string]http.HandlerFunc{
		"/images/create": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		},
		"/containers/create": func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{"Id": "abc123"})
		},
		"/containers/": func(w http.ResponseWriter, r *http.Request) {
			if r.Method == "DELETE" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			if r.Method == "PUT" { // archive upload (config.yaml copy)
				archivePath = r.URL.Query().Get("path")
				w.WriteHeader(http.StatusOK)
				return
			}
			if r.Method == "POST" { // start / rename
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		},
	})
	defer ts.Close()

	c := &DockerConnector{}
	err := c.Deploy(context.Background(), nestForMock(ts), nil, EggDeployPayload{
		ConfigYAML: configYAML,
		EggPort:    8099,
		SharedKey:  "aabb",
	})
	if err != nil {
		t.Errorf("Deploy should succeed: %v", err)
	}
	if archivePath != "/app/data" {
		t.Fatalf("config archive path = %q, want /app/data so the entrypoint reads the generated egg config", archivePath)
	}
	for _, bind := range createBody.HostConfig.Binds {
		if strings.Contains(bind, ":/app/data") {
			t.Fatalf("docker deploy must not reuse a named /app/data volume across hatches; got bind %q", bind)
		}
	}
	if _, ok := createBody.ExposedPorts["8099/tcp"]; !ok {
		t.Fatalf("docker deploy should expose the configured egg HTTP port 8099/tcp, got %#v", createBody.ExposedPorts)
	}
	if _, ok := createBody.ExposedPorts["8088/tcp"]; ok {
		t.Fatalf("docker deploy should not expose the master's default HTTP port for an egg, got %#v", createBody.ExposedPorts)
	}
	if len(createBody.Healthcheck.Test) == 0 {
		t.Fatal("docker deploy should override the image healthcheck for egg containers")
	}
	healthcheck := strings.Join(createBody.Healthcheck.Test, " ")
	if !strings.Contains(healthcheck, "/app/data/config.yaml") || !strings.Contains(healthcheck, "/api/ready") {
		t.Fatalf("healthcheck should read the egg config and probe readiness, got %q", healthcheck)
	}
}

func TestDockerConnector_Deploy_PullFails(t *testing.T) {
	ts := mockDockerAPI(t, map[string]http.HandlerFunc{
		"/images/create": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("registry unreachable"))
		},
	})
	defer ts.Close()

	c := &DockerConnector{}
	err := c.Deploy(context.Background(), nestForMock(ts), nil, EggDeployPayload{
		ConfigYAML: []byte("egg_mode:\n  master_url: ws://localhost\n"),
	})
	if err == nil {
		t.Error("Deploy should fail when pull fails")
	}
	if !strings.Contains(err.Error(), "pull") {
		t.Errorf("error should mention pull: %v", err)
	}
}

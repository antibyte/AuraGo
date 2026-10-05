package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/acestep"
	"aurago/internal/dockerutil"
)

func TestDockerComposeResolvedConfigReturnsStdoutOnly(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	workspace := t.TempDir()
	composeFile := filepath.Join(workspace, "compose.yml")
	if err := os.WriteFile(composeFile, []byte("version: \"3.8\"\nservices:\n  web:\n    image: alpine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	original := runDockerComposeConfig
	t.Cleanup(func() { runDockerComposeConfig = original })
	var gotArgs []string
	runDockerComposeConfig = func(_ context.Context, args []string) ([]byte, []byte, error) {
		gotArgs = append([]string(nil), args...)
		return []byte(`{"name":"ws","services":{"web":{"image":"alpine"}}}`),
			[]byte("time=\"2026-10-05T19:28:33+02:00\" level=warning msg=\"compose.yml: the attribute `version` is obsolete\"\n"), nil
	}

	resolved, err := DockerComposeResolvedConfig(DockerConfig{WorkspaceDir: workspace}, "compose.yml")
	if err != nil {
		t.Fatalf("DockerComposeResolvedConfig() error = %v", err)
	}
	if strings.Contains(resolved, "obsolete") {
		t.Fatalf("stderr warning leaked into the model: %s", resolved)
	}
	model, err := ParseDockerComposeModel(resolved)
	if err != nil || model.Services["web"].Image != "alpine" {
		t.Fatalf("ParseDockerComposeModel() = %+v, %v", model, err)
	}
	want := []string{"compose", "-f", composeFile, "config", "--format", "json"}
	if strings.Join(gotArgs, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("args = %q, want %q", gotArgs, want)
	}

	runDockerComposeConfig = func(context.Context, []string) ([]byte, []byte, error) {
		return nil, []byte("env file " + filepath.Join(workspace, "missing.env") + " not found"), errors.New("exit status 1")
	}
	if _, err := DockerComposeResolvedConfig(DockerConfig{WorkspaceDir: workspace}, "compose.yml"); err == nil || !strings.Contains(err.Error(), "missing.env not found") {
		t.Fatalf("error = %v, want the stderr detail", err)
	}
}

func TestParseDockerComposeModelReadsResolvedShapes(t *testing.T) {
	resolved := `{
  "name": "k4",
  "services": {
    "app": {
      "image": "alpine",
      "container_name": "myapp",
      "labels": {"a": "b"},
      "environment": {"FOO": "bar", "UNSET": null},
      "build": {"context": "/ws/ctx", "dockerfile": "Dockerfile", "additional_contexts": {"extra": "/srv/other"}, "args": {"V": "1"}},
      "volumes_from": ["third"],
      "cap_add": ["NET_ADMIN"],
      "security_opt": ["apparmor:unconfined"],
      "ipc": "host",
      "userns_mode": "host",
      "devices": [{"source": "/dev/dri", "target": "/dev/dri", "permissions": "rwm"}, "/dev/ttyUSB0:/dev/ttyUSB0"],
      "volumes": [
        {"type": "volume", "source": "hostvol", "target": "/data", "volume": {}},
        {"type": "npipe", "source": "\\\\.\\pipe\\docker_engine", "target": "\\\\.\\pipe\\docker_engine"},
        {"type": "bind", "source": "/etc/localtime", "target": "/etc/localtime", "read_only": true, "bind": {}}
      ]
    },
    "other": {"image": "busybox", "network_mode": "service:app", "pid": "container:abc", "privileged": true, "uts": "host", "cgroup": "host"}
  },
  "secrets": {"s1": {"name": "k4_s1", "file": "/ws/secret.txt"}},
  "configs": {"c1": {"name": "k4_c1", "file": "/etc/hostname"}},
  "volumes": {"hostvol": {"name": "k4_hostvol", "driver": "local", "driver_opts": {"device": "/", "o": "bind", "type": "none"}}}
}`
	model, err := ParseDockerComposeModel(resolved)
	if err != nil {
		t.Fatalf("ParseDockerComposeModel() error = %v", err)
	}
	app := model.Services["app"]
	if model.Name != "k4" || app.ContainerName != "myapp" || app.Labels["a"] != "b" {
		t.Fatalf("name/container/labels not decoded: %+v", model)
	}
	if app.Environment["UNSET"] != nil || app.Environment["FOO"] == nil || *app.Environment["FOO"] != "bar" {
		t.Fatalf("environment = %+v", app.Environment)
	}
	if app.Build == nil || app.Build.Context != "/ws/ctx" || app.Build.AdditionalContexts["extra"] != "/srv/other" || app.Build.Args["V"] == nil {
		t.Fatalf("build = %+v", app.Build)
	}
	if len(app.Devices) != 2 || app.Devices[0].Source != "/dev/dri" || app.Devices[1].Source != "/dev/ttyUSB0" || app.Devices[1].Target != "/dev/ttyUSB0" {
		t.Fatalf("devices = %+v", app.Devices)
	}
	if len(app.Volumes) != 3 || app.Volumes[1].Type != "npipe" || !app.Volumes[2].ReadOnly {
		t.Fatalf("volumes = %+v", app.Volumes)
	}
	if other := model.Services["other"]; !other.Privileged || other.NetworkMode != "service:app" || other.Uts != "host" || other.Cgroup != "host" {
		t.Fatalf("other = %+v", model.Services["other"])
	}
	if model.Volumes["hostvol"].DriverOpts["o"] != "bind" || model.Secrets["s1"].File != "/ws/secret.txt" || model.Configs["c1"].File != "/etc/hostname" {
		t.Fatalf("top-level resources = %+v %+v %+v", model.Volumes, model.Secrets, model.Configs)
	}
}

func TestParseDockerComposeModelRejectsEmptyAndNonJSON(t *testing.T) {
	for _, resolved := range []string{"", "   ", "services:\n  web:\n    image: alpine\n"} {
		if _, err := ParseDockerComposeModel(resolved); err == nil {
			t.Fatalf("ParseDockerComposeModel(%q) accepted a non-model", resolved)
		}
	}
}

func TestDockerComposeModelOwnerFindsStructuredReferences(t *testing.T) {
	cases := map[string]struct {
		resolved string
		want     string
	}{
		"garage container":           {`{"services":{"w":{"container_name":"aurago-boring-garage"}}}`, dockerutil.BoringGarageOwner},
		"homepage label":             {`{"services":{"w":{"labels":{"aurago.managed":"homepage"}}}}`, dockerutil.HomepageOwner},
		"legacy local llm label":     {`{"services":{"w":{"labels":{"com.aurago.managed":"true","com.aurago.owner":"local-llm"}}}}`, dockerutil.LocalLLMOwner},
		"homepage image digest":      {`{"services":{"w":{"image":"ghcr.io/x/aurago-homepage@sha256:abc"}}}`, dockerutil.HomepageOwner},
		"local llm named volume":     {`{"services":{"w":{"volumes":[{"type":"volume","source":"models","target":"/m"}]}},"volumes":{"models":{"name":"aurago_models"}}}`, dockerutil.LocalLLMOwner},
		"acestep container":          {fmt.Sprintf(`{"services":{"w":{"container_name":%q}}}`, acestep.ContainerName), acestep.Owner},
		"garage data bind":           {`{"services":{"w":{"volumes":[{"type":"bind","source":"/opt/aurago/data/sidecars/garage","target":"/d"}]}}}`, dockerutil.BoringGarageOwner},
		"garage local volume device": {`{"services":{},"volumes":{"g":{"name":"x_g","driver":"local","driver_opts":{"type":"none","o":"bind","device":"/opt/aurago/data/sidecars/garage"}}}}`, dockerutil.BoringGarageOwner},
		"volumes_from container":     {`{"services":{"w":{"volumes_from":["container:aurago-homepage:ro"]}}}`, dockerutil.HomepageOwner},
		"network of llm container":   {`{"services":{"w":{"network_mode":"container:aurago-local-llm"}}}`, dockerutil.LocalLLMOwner},
		"app container name":         {`{"services":{"w":{"container_name":"aurago"}}}`, dockerutil.AppOwner},
		"app owner label":            {`{"services":{"w":{"labels":{"aurago.managed":"aurago-app"}}}}`, dockerutil.AppOwner},
		"volumes_from app container": {`{"services":{"w":{"volumes_from":["container:aurago"]}}}`, dockerutil.AppOwner},
		"user name ending in aurago": {`{"services":{"w":{"container_name":"dev-aurago"}}}`, ""},
		"user name with aurago":      {`{"services":{"w":{"container_name":"my-aurago-1"}}}`, ""},
		"same-project volumes_from":  {`{"services":{"aurago":{"image":"x"},"w":{"volumes_from":["aurago"]}}}`, ""},
		"plain stack":                {`{"services":{"w":{"image":"nginx:1.27","container_name":"web","volumes":[{"type":"bind","source":"/srv/www","target":"/usr/share/nginx/html"}]}}}`, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			model, err := ParseDockerComposeModel(tc.resolved)
			if err != nil {
				t.Fatalf("ParseDockerComposeModel() error = %v", err)
			}
			if got := DockerComposeModelOwner(model); got != tc.want {
				t.Fatalf("DockerComposeModelOwner() = %q, want %q", got, tc.want)
			}
		})
	}
}

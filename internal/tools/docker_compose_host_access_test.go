package tools

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

func composeBind(source, target string, readOnly bool) DockerComposeMount {
	return DockerComposeMount{Type: "bind", Source: source, Target: target, ReadOnly: readOnly}
}

func composeServices(services map[string]DockerComposeService) DockerComposeModel {
	return DockerComposeModel{Services: services}
}

func composeViolationFields(violations []DockerComposeViolation) map[string]bool {
	fields := make(map[string]bool, len(violations))
	for _, violation := range violations {
		fields[violation.Field] = true
	}
	return fields
}

type homeLabComposeFixture struct {
	model  DockerComposeModel
	fields []string
}

// homeLabComposeFixtures are stacks users deploy through the agent today. With
// docker.allow_host_access they must keep working unchanged.
func homeLabComposeFixtures() map[string]homeLabComposeFixture {
	return map[string]homeLabComposeFixture{
		"traefik": {composeServices(map[string]DockerComposeService{"traefik": {Image: "traefik:v3.1",
			Volumes: []DockerComposeMount{composeBind("/var/run/docker.sock", "/var/run/docker.sock", true)}}}), []string{"volumes"}},
		"portainer on windows": {composeServices(map[string]DockerComposeService{"portainer": {Image: "portainer/portainer-ce:2.21.0",
			Volumes: []DockerComposeMount{{Type: "npipe", Source: `\\.\pipe\docker_engine`, Target: `\\.\pipe\docker_engine`}, {Type: "volume", Source: "portainer_data", Target: "/data"}}}}), []string{"volumes"}},
		"home assistant": {composeServices(map[string]DockerComposeService{"homeassistant": {Image: "ghcr.io/home-assistant/home-assistant:stable", Privileged: true, NetworkMode: "host",
			Volumes: []DockerComposeMount{composeBind("/srv/homeassistant/config", "/config", false), composeBind("/etc/localtime", "/etc/localtime", true), composeBind("/run/dbus", "/run/dbus", true)}}}), []string{"privileged", "network_mode", "volumes"}},
		"frigate": {composeServices(map[string]DockerComposeService{"frigate": {Image: "ghcr.io/blakeblackshear/frigate:stable",
			Devices: []DockerComposeDevice{{Source: "/dev/bus/usb", Target: "/dev/bus/usb"}, {Source: "/dev/dri/renderD128", Target: "/dev/dri/renderD128"}},
			Volumes: []DockerComposeMount{composeBind("/etc/localtime", "/etc/localtime", true), composeBind("/srv/frigate/media", "/media/frigate", false)}}}), []string{"devices", "volumes"}},
		"node-exporter": {composeServices(map[string]DockerComposeService{"node-exporter": {Image: "quay.io/prometheus/node-exporter:v1.8.2", Pid: "host", NetworkMode: "host",
			Volumes: []DockerComposeMount{composeBind("/", "/host", true)}}}), []string{"pid", "network_mode", "volumes"}},
		"media server": {composeServices(map[string]DockerComposeService{"jellyfin": {Image: "lscr.io/linuxserver/jellyfin:latest",
			Volumes: []DockerComposeMount{composeBind("/srv/media", "/data/media", true)}}}), []string{"volumes"}},
		"vpn capabilities": {composeServices(map[string]DockerComposeService{"wireguard": {Image: "lscr.io/linuxserver/wireguard:latest",
			CapAdd: []string{"NET_ADMIN", "SYS_MODULE"}, SecurityOpt: []string{"apparmor:unconfined"}}}), []string{"cap_add", "security_opt"}},
		"systemd container": {composeServices(map[string]DockerComposeService{"systemd": {Image: "jrei/systemd-ubuntu:24.04",
			Privileged: true, Uts: "host", Cgroup: "host", Ipc: "host", UsernsMode: "host"}}), []string{"privileged", "uts", "cgroup", "ipc", "userns_mode"}},
		"local bind volume": {DockerComposeModel{
			Services: map[string]DockerComposeService{"backup": {Image: "restic/restic:0.17.1", Volumes: []DockerComposeMount{{Type: "volume", Source: "hostdata", Target: "/source"}}}},
			Volumes:  map[string]DockerComposeNamedVolume{"hostdata": {Name: "backup_hostdata", Driver: "local", DriverOpts: map[string]string{"type": "none", "o": "bind", "device": "/srv/data"}}},
		}, []string{"volumes.hostdata.driver_opts.device"}},
		"cloud provider service": {composeServices(map[string]DockerComposeService{"database": {Provider: &DockerComposeProvider{Type: "awesomecloud"}}}), []string{"provider"}},
		"watch outside the workspace": {composeServices(map[string]DockerComposeService{"web": {Image: "node:22",
			Develop: &DockerComposeDevelop{Watch: []DockerComposeWatch{{Path: "/srv/src/web", Action: "sync"}}}}}), []string{"develop.watch"}},
		"privileged build with ssh": {DockerComposeModel{
			Services: map[string]DockerComposeService{"app": {Image: "example.invalid/app", Build: &DockerComposeBuild{
				Context: "/srv/src/app", Dockerfile: "Dockerfile", SSH: DockerComposeList{"default"},
				Secrets: DockerComposeRefs{"npmrc"}, Privileged: true, Entitlements: DockerComposeList{"network.host"},
			}}},
			Secrets: map[string]DockerComposeFileResource{"npmrc": {File: "/srv/secrets/npmrc"}},
		}, []string{"build.context", "build.ssh", "build.privileged", "build.entitlements", "secrets.npmrc.file"}},
	}
}

func composeTestPolicy(t *testing.T, allow bool) DockerComposeHostPolicy {
	t.Helper()
	root := t.TempDir()
	return DockerComposeHostPolicy{
		WorkspaceDir:    filepath.Join(root, "agent_workspace", "workdir"),
		AllowHostAccess: allow,
		ProtectedRoots:  []string{filepath.Join(root, "data"), "/etc/aurago"},
		ProtectedFiles:  []string{filepath.Join(root, "config.yaml"), filepath.Join(root, ".env")},
		MasterKey:       strings.Repeat("ab", 32),
	}
}

func TestEvaluateDockerComposeHostAccessAllowsHomeLabStacksWithHostAccess(t *testing.T) {
	policy := composeTestPolicy(t, true)
	for name, fixture := range homeLabComposeFixtures() {
		for _, scope := range []DockerComposeHostAccessScope{DockerComposeScopeRun, DockerComposeScopeBuild, DockerComposeScopeRender} {
			if violations := EvaluateDockerComposeHostAccess(fixture.model, nil, scope, policy); len(violations) != 0 {
				t.Fatalf("%s (scope %d): violations = %+v, want none with docker.allow_host_access", name, scope, violations)
			}
		}
	}
}

func TestEvaluateDockerComposeHostAccessRejectsHostAttributesWithoutHostAccess(t *testing.T) {
	policy := composeTestPolicy(t, false)
	for name, fixture := range homeLabComposeFixtures() {
		violations := EvaluateDockerComposeHostAccess(fixture.model, nil, DockerComposeScopeRun, policy)
		fields := composeViolationFields(violations)
		for _, field := range fixture.fields {
			if !fields[field] {
				t.Fatalf("%s: missing %s violation in %+v", name, field, violations)
			}
		}
		for _, violation := range violations {
			if violation.Always {
				t.Fatalf("%s: host-access violation marked always: %+v", name, violation)
			}
		}
		// config/convert render the model; the host-access tier never applies.
		if violations := EvaluateDockerComposeHostAccess(fixture.model, nil, DockerComposeScopeRender, policy); len(violations) != 0 {
			t.Fatalf("%s: config/convert rejected host attributes: %+v", name, violations)
		}
	}
	outside := DockerComposeModel{
		Services: map[string]DockerComposeService{"app": {Image: "example.invalid/app", Build: &DockerComposeBuild{Context: "/srv/src/app", Dockerfile: "Dockerfile"}}},
		Secrets:  map[string]DockerComposeFileResource{"token": {File: "/srv/secrets/token"}},
		Configs:  map[string]DockerComposeFileResource{"nginx": {File: "/etc/nginx/nginx.conf"}},
	}
	envFiles := []DockerComposeEnvFile{{Service: "app", Path: "/srv/env/app.env"}}
	fields := composeViolationFields(EvaluateDockerComposeHostAccess(outside, envFiles, DockerComposeScopeRun, policy))
	for _, field := range []string{"build.context", "secrets.token.file", "configs.nginx.file", "env_file"} {
		if !fields[field] {
			t.Fatalf("missing %s violation, got %v", field, fields)
		}
	}
}

func TestEvaluateDockerComposeHostAccessAllowsWorkspaceStacksWithoutHostAccess(t *testing.T) {
	policy := composeTestPolicy(t, false)
	workspace := policy.WorkspaceDir
	model := DockerComposeModel{
		Services: map[string]DockerComposeService{"app": {
			Image:       "example.invalid/app",
			NetworkMode: "service:db",
			Volumes: []DockerComposeMount{
				composeBind(filepath.Join(workspace, "stack", "data"), "/data", false),
				{Type: "volume", Source: "appdata", Target: "/var/lib/app"},
				{Type: "tmpfs", Target: "/tmp"},
			},
			Build: &DockerComposeBuild{Context: filepath.Join(workspace, "stack", "app"), Dockerfile: "Dockerfile",
				AdditionalContexts: map[string]string{"base": "docker-image://alpine:3.20", "deps": "service:db"},
				Secrets:            DockerComposeRefs{"token"}},
			Develop: &DockerComposeDevelop{Watch: []DockerComposeWatch{{Path: filepath.Join(workspace, "stack", "app", "src"), Action: "sync"}}},
		}},
		Secrets: map[string]DockerComposeFileResource{"token": {File: filepath.Join(workspace, "stack", "token.txt")}},
		Volumes: map[string]DockerComposeNamedVolume{"appdata": {Name: "stack_appdata", Driver: "local"}},
	}
	envFiles := []DockerComposeEnvFile{{Service: "app", Path: filepath.Join(workspace, "stack", "app.env")}}
	for _, scope := range []DockerComposeHostAccessScope{DockerComposeScopeRun, DockerComposeScopeBuild, DockerComposeScopeRender} {
		if violations := EvaluateDockerComposeHostAccess(model, envFiles, scope, policy); len(violations) != 0 {
			t.Fatalf("workspace stack rejected (scope %d): %+v", scope, violations)
		}
	}
}

func TestEvaluateDockerComposeHostAccessAlwaysRejectsAuraGoState(t *testing.T) {
	policy := composeTestPolicy(t, true)
	root := filepath.Dir(policy.ProtectedFiles[0])
	dataDir := policy.ProtectedRoots[0]
	masterKey := policy.MasterKey
	single := func(service DockerComposeService) DockerComposeModel {
		return composeServices(map[string]DockerComposeService{"app": service})
	}
	cases := map[string]struct {
		model    DockerComposeModel
		envFiles []DockerComposeEnvFile
		field    string
	}{
		"vault bind":           {model: single(DockerComposeService{Image: "x", Volumes: []DockerComposeMount{composeBind(filepath.Join(dataDir, "vault.bin"), "/v", true)}}), field: "volumes"},
		"data dir bind":        {model: single(DockerComposeService{Image: "x", Volumes: []DockerComposeMount{composeBind(dataDir, "/state", true)}}), field: "volumes"},
		"config bind":          {model: single(DockerComposeService{Image: "x", Volumes: []DockerComposeMount{composeBind(filepath.Join(root, "config.yaml"), "/c", true)}}), field: "volumes"},
		"systemd master key":   {model: single(DockerComposeService{Image: "x", Volumes: []DockerComposeMount{composeBind("/etc/aurago/master.key", "/k", true)}}), field: "volumes"},
		"master key file name": {model: single(DockerComposeService{Image: "x", Volumes: []DockerComposeMount{composeBind("/opt/keys/aurago_master.key", "/k", true)}}), field: "volumes"},
		"aurago env file":      {model: single(DockerComposeService{Image: "x"}), envFiles: []DockerComposeEnvFile{{Service: "app", Path: filepath.Join(root, ".env")}}, field: "env_file"},
		"secret from data dir": {model: DockerComposeModel{Services: map[string]DockerComposeService{"app": {Image: "x"}}, Secrets: map[string]DockerComposeFileResource{"vault": {File: filepath.Join(dataDir, "vault.bin")}}}, field: "secrets.vault.file"},
		"config from data dir": {model: DockerComposeModel{Services: map[string]DockerComposeService{"app": {Image: "x"}}, Configs: map[string]DockerComposeFileResource{"cfg": {File: filepath.Join(root, "config.yaml")}}}, field: "configs.cfg.file"},
		"build context data":   {model: single(DockerComposeService{Image: "x", Build: &DockerComposeBuild{Context: dataDir, Dockerfile: "Dockerfile"}}), field: "build.context"},
		"master key in env":    {model: single(DockerComposeService{Image: "x", Environment: map[string]*string{"KEY": &masterKey}}), field: "environment.KEY"},
		"master key in label":  {model: single(DockerComposeService{Image: "x", Labels: map[string]string{"k": "prefix-" + masterKey}}), field: "labels.k"},
		"master key build arg": {model: single(DockerComposeService{Image: "x", Build: &DockerComposeBuild{Context: "/srv/app", Args: map[string]*string{"K": &masterKey}}}), field: "build.args.K"},
		"local bind of data":   {model: DockerComposeModel{Services: map[string]DockerComposeService{"app": {Image: "x"}}, Volumes: map[string]DockerComposeNamedVolume{"state": {Name: "s", Driver: "local", DriverOpts: map[string]string{"type": "none", "o": "bind", "device": dataDir}}}}, field: "volumes.state.driver_opts.device"},
		"watch of data dir":    {model: single(DockerComposeService{Image: "x", Develop: &DockerComposeDevelop{Watch: []DockerComposeWatch{{Path: dataDir, Action: "sync"}}}}), field: "develop.watch"},
		"secret from env key":  {model: DockerComposeModel{Services: map[string]DockerComposeService{"app": {Image: "x"}}, Secrets: map[string]DockerComposeFileResource{"mk": {Environment: "AURAGO_MASTER_KEY"}}}, field: "secrets.mk.environment"},
		"config content key":   {model: DockerComposeModel{Services: map[string]DockerComposeService{"app": {Image: "x"}}, Configs: map[string]DockerComposeFileResource{"c": {Content: "key=" + masterKey}}}, field: "configs.c.content"},
	}
	for name, tc := range cases {
		violations := EvaluateDockerComposeHostAccess(tc.model, tc.envFiles, DockerComposeScopeRun, policy)
		if len(violations) == 0 || !violations[0].Always || violations[0].Field != tc.field {
			t.Fatalf("%s: violations = %+v, want an always-tier %s violation", name, violations, tc.field)
		}
	}
	// Parents of protected paths are not blocked at this tier: node-exporter
	// style binds keep working with host access (and do expose the children).
	for _, parent := range []string{root, "/"} {
		model := single(DockerComposeService{Image: "x", Volumes: []DockerComposeMount{composeBind(parent, "/host", true)}})
		if violations := EvaluateDockerComposeHostAccess(model, nil, DockerComposeScopeRun, policy); len(violations) != 0 {
			t.Fatalf("parent bind %s rejected with host access: %+v", parent, violations)
		}
	}
}

func TestEvaluateDockerComposeHostAccessRenderChecksOnlyAuraGoStateFiles(t *testing.T) {
	// `docker compose config` prints env_file content inlined into environment,
	// so AuraGo state files are rejected for config/convert too, while binds,
	// build contexts and other host attributes are not checked there.
	for _, allow := range []bool{false, true} {
		policy := composeTestPolicy(t, allow)
		root := filepath.Dir(policy.ProtectedFiles[0])
		dataDir := policy.ProtectedRoots[0]
		masterKey := policy.MasterKey
		cases := map[string]struct {
			model    DockerComposeModel
			envFiles []DockerComposeEnvFile
			field    string
		}{
			"aurago env file":    {model: composeServices(map[string]DockerComposeService{"app": {Image: "x"}}), envFiles: []DockerComposeEnvFile{{Service: "app", Path: filepath.Join(root, ".env")}}, field: "env_file"},
			"secret in data dir": {model: DockerComposeModel{Services: map[string]DockerComposeService{"app": {Image: "x"}}, Secrets: map[string]DockerComposeFileResource{"v": {File: filepath.Join(dataDir, "vault.bin")}}}, field: "secrets.v.file"},
			"config file":        {model: DockerComposeModel{Services: map[string]DockerComposeService{"app": {Image: "x"}}, Configs: map[string]DockerComposeFileResource{"c": {File: filepath.Join(root, "config.yaml")}}}, field: "configs.c.file"},
			"master key in env":  {model: composeServices(map[string]DockerComposeService{"app": {Image: "x", Environment: map[string]*string{"KEY": &masterKey}}}), field: "environment.KEY"},
		}
		for name, tc := range cases {
			violations := EvaluateDockerComposeHostAccess(tc.model, tc.envFiles, DockerComposeScopeRender, policy)
			if len(violations) != 1 || !violations[0].Always || violations[0].Field != tc.field {
				t.Fatalf("allow=%v %s: violations = %+v, want one always-tier %s violation", allow, name, violations, tc.field)
			}
		}
		binds := composeServices(map[string]DockerComposeService{"app": {Image: "x", Privileged: true,
			Volumes: []DockerComposeMount{composeBind(dataDir, "/state", true), composeBind("/srv/media", "/media", true)},
			Build:   &DockerComposeBuild{Context: dataDir}}})
		envFiles := []DockerComposeEnvFile{{Service: "app", Path: "/srv/env/app.env"}}
		if violations := EvaluateDockerComposeHostAccess(binds, envFiles, DockerComposeScopeRender, policy); len(violations) != 0 {
			t.Fatalf("allow=%v: config/convert checked binds, build contexts or outside env files: %+v", allow, violations)
		}
	}
}

func TestEvaluateDockerComposeHostAccessBuildChecksBuildSectionsOnly(t *testing.T) {
	policy := composeTestPolicy(t, false)
	model := composeServices(map[string]DockerComposeService{"app": {Image: "x", Privileged: true,
		Volumes: []DockerComposeMount{composeBind("/etc", "/host-etc", true)},
		Develop: &DockerComposeDevelop{Watch: []DockerComposeWatch{{Path: "/srv/src", Action: "sync"}}},
		Build:   &DockerComposeBuild{Context: filepath.Join(policy.WorkspaceDir, "app"), Dockerfile: "Dockerfile"}}})
	model.Secrets = map[string]DockerComposeFileResource{"unused": {File: "/srv/secrets/unused"}}
	envFiles := []DockerComposeEnvFile{{Service: "app", Path: "/srv/env/app.env"}}
	if violations := EvaluateDockerComposeHostAccess(model, envFiles, DockerComposeScopeBuild, policy); len(violations) != 0 {
		t.Fatalf("build checked non-build attributes: %+v", violations)
	}
	model.Services["app"] = DockerComposeService{Image: "x", Build: &DockerComposeBuild{Context: "/srv/other/app", Dockerfile: "Dockerfile"}}
	fields := composeViolationFields(EvaluateDockerComposeHostAccess(model, nil, DockerComposeScopeBuild, policy))
	if !fields["build.context"] || len(fields) != 1 {
		t.Fatalf("build context outside workspace: fields = %v, want only build.context", fields)
	}
	model.Services["app"] = DockerComposeService{Image: "x", Build: &DockerComposeBuild{Context: filepath.Join(policy.WorkspaceDir, "app"),
		SSH: DockerComposeList{"default"}, Secrets: DockerComposeRefs{"unused"}, Privileged: true, Entitlements: DockerComposeList{"security.insecure"}}}
	fields = composeViolationFields(EvaluateDockerComposeHostAccess(model, nil, DockerComposeScopeBuild, policy))
	for _, field := range []string{"build.ssh", "build.secrets.unused", "build.privileged", "build.entitlements"} {
		if !fields[field] {
			t.Fatalf("build: missing %s violation, got %v", field, fields)
		}
	}
	policy.AllowHostAccess = true
	if violations := EvaluateDockerComposeHostAccess(model, nil, DockerComposeScopeBuild, policy); len(violations) != 0 {
		t.Fatalf("build with host access: %+v", violations)
	}
}

func TestParseDockerComposeModelReadsHostAccessFields(t *testing.T) {
	resolved := `{
  "services": {
    "app": {
      "image": "alpine",
      "env_file": ["rel.env", {"path": "/ws/abs.env", "required": false}],
      "provider": {"type": "awesomecloud", "options": {"type": ["mysql"]}},
      "develop": {"watch": [{"path": "/ws/src", "action": "sync", "target": "/app", "exec": {"command": null}}]},
      "build": {"context": "/ws", "ssh": ["default"], "secrets": [{"source": "tok"}, "plain"], "privileged": true, "entitlements": ["network.host"]}
    },
    "odd": {"image": "alpine", "build": {"context": "/ws", "ssh": {"default": ""}, "entitlements": "security.insecure", "secrets": {"tok": null}}}
  },
  "secrets": {"tok": {"name": "x_tok", "file": "/ws/tok"}, "env": {"name": "x_env", "environment": "AURAGO_MASTER_KEY"}},
  "configs": {"c": {"name": "x_c", "content": "inline"}}
}`
	model, err := ParseDockerComposeModel(resolved)
	if err != nil {
		t.Fatalf("ParseDockerComposeModel() error = %v", err)
	}
	app := model.Services["app"]
	if len(app.EnvFile) != 2 || app.EnvFile[0].Path != "rel.env" || app.EnvFile[1].Path != "/ws/abs.env" {
		t.Fatalf("env_file = %+v", app.EnvFile)
	}
	if app.Provider == nil || app.Provider.Type != "awesomecloud" {
		t.Fatalf("provider = %+v", app.Provider)
	}
	if app.Develop == nil || len(app.Develop.Watch) != 1 || app.Develop.Watch[0].Path != "/ws/src" {
		t.Fatalf("develop = %+v", app.Develop)
	}
	build := app.Build
	if build == nil || len(build.SSH) != 1 || strings.Join(build.Secrets, ",") != "tok,plain" || !build.Privileged || len(build.Entitlements) != 1 {
		t.Fatalf("build = %+v", build)
	}
	odd := model.Services["odd"].Build
	if odd == nil || len(odd.SSH) != 1 || len(odd.Entitlements) != 1 || strings.Join(odd.Secrets, ",") != "tok" {
		t.Fatalf("other build shapes = %+v", odd)
	}
	if model.Secrets["env"].Environment != "AURAGO_MASTER_KEY" || model.Configs["c"].Content != "inline" {
		t.Fatalf("secret/config sources = %+v %+v", model.Secrets, model.Configs)
	}
	// Shapes no Compose release prints must not break the preflight.
	for _, build := range []string{`{"ssh": 42}`, `{"entitlements": [1, {"a": true}]}`, `{"secrets": [7]}`} {
		if _, err := ParseDockerComposeModel(`{"services":{"w":{"build":` + build + `}}}`); err != nil {
			t.Fatalf("build %s broke the parse: %v", build, err)
		}
	}
}

func TestDockerComposeServiceEnvFilesListsBothShapes(t *testing.T) {
	workspace := t.TempDir()
	appEnv, _ := json.Marshal(filepath.Join(workspace, "app.env"))
	model, err := ParseDockerComposeModel(`{"services":{"b":{"env_file":["rel.env"]},"a":{"env_file":[{"path":` + string(appEnv) + `,"required":true}]},"c":{"image":"alpine"}}}`)
	if err != nil {
		t.Fatalf("ParseDockerComposeModel() error = %v", err)
	}
	files := DockerComposeServiceEnvFiles(model, workspace)
	want := []DockerComposeEnvFile{{Service: "a", Path: filepath.Join(workspace, "app.env")}, {Service: "b", Path: filepath.Join(workspace, "rel.env")}}
	if len(files) != len(want) || files[0] != want[0] || files[1] != want[1] {
		t.Fatalf("env files = %+v, want %+v", files, want)
	}
}

func TestDockerComposeProtectedPathsCoverAuraGoState(t *testing.T) {
	root := t.TempDir()
	cfg := &config.Config{}
	cfg.Directories.DataDir = filepath.Join(root, "data")
	cfg.ConfigPath = filepath.Join(root, "config.yaml")
	roots, files := DockerComposeProtectedPaths(cfg)
	all := strings.Join(append(append([]string(nil), roots...), files...), "\n")
	for _, want := range []string{cfg.Directories.DataDir, "/etc/aurago", cfg.ConfigPath, filepath.Join(root, ".env"), "/run/secrets/aurago_master_key", filepath.Join(cfg.Directories.DataDir, "vault.bin")} {
		if !strings.Contains(all, want) {
			t.Fatalf("protected paths %q miss %q", all, want)
		}
	}
}

func TestDockerComposeSubcommand(t *testing.T) {
	for command, want := range map[string]string{"up -d": "up", "  build --pull app": "build", "config": "config", "exec web sh": "", "": "", "up --volume x": ""} {
		if got := DockerComposeSubcommand(command); got != want {
			t.Fatalf("DockerComposeSubcommand(%q) = %q, want %q", command, got, want)
		}
	}
}

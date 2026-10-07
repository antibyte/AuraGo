package tools

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"aurago/internal/dockerutil"
)

func TestDockerMutationsDenyWhenRuntimeReadOnly(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true, DockerReadOnly: true})
	t.Cleanup(func() {
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})

	result := DockerCreateContainer(DockerConfig{}, "test", "alpine:latest", nil, nil, nil, nil, "", nil)
	if !strings.Contains(result, "docker mutation is disabled") {
		t.Fatalf("DockerCreateContainer = %s, want docker readonly denial", result)
	}

	if _, _, err := DockerRequest(DockerConfig{}, "POST", "/containers/test/start", ""); err == nil || !strings.Contains(err.Error(), "docker mutation is disabled") {
		t.Fatalf("DockerRequest mutation error = %v, want docker readonly denial", err)
	}

	copyResult := DockerCopy(DockerConfig{WorkspaceDir: t.TempDir()}, "container-1", "src.txt", "dest.txt", "to_container")
	if !strings.Contains(copyResult, "docker mutation is disabled") {
		t.Fatalf("DockerCopy = %s, want docker readonly denial", copyResult)
	}

	composeResult := DockerCompose(DockerConfig{}, "docker-compose.yml", "up -d")
	if !strings.Contains(composeResult, "docker mutation is disabled") {
		t.Fatalf("DockerCompose = %s, want docker readonly denial", composeResult)
	}
}

func TestBuildDockerCreateContainerPayloadAppliesNetworkModeOption(t *testing.T) {
	payload := buildDockerCreateContainerPayloadWithOptions(
		"alpine:latest",
		nil,
		map[string]string{},
		nil,
		nil,
		"no",
		nil,
		ContainerCreateOptions{NetworkMode: "none"},
	)
	hostConfig, ok := payload["HostConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("HostConfig = %#v", payload["HostConfig"])
	}
	if got := hostConfig["NetworkMode"]; got != "none" {
		t.Fatalf("NetworkMode = %#v, want none", got)
	}
}

func TestBuildDockerCreateContainerPayloadControlsAutoRemove(t *testing.T) {
	for _, tt := range []struct {
		name    string
		options ContainerCreateOptions
		want    bool
	}{
		{name: "internal default remains disabled", options: ContainerCreateOptions{}, want: false},
		{name: "agent option is forwarded", options: ContainerCreateOptions{AutoRemove: true}, want: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			payload := buildDockerCreateContainerPayloadWithOptions(
				"alpine:latest", nil, map[string]string{}, nil, nil, "no", nil, tt.options,
			)
			hostConfig, ok := payload["HostConfig"].(map[string]interface{})
			if !ok {
				t.Fatalf("HostConfig = %#v", payload["HostConfig"])
			}
			got, present := hostConfig["AutoRemove"]
			if tt.want {
				if !present || got != true {
					t.Fatalf("AutoRemove = %#v (present %v), want true", got, present)
				}
			} else if present {
				t.Fatalf("AutoRemove = %#v, want omitted to preserve internal payloads", got)
			}
		})
	}
}

func TestBuildDockerCreateContainerPayloadAppliesCapAddOption(t *testing.T) {
	payload := buildDockerCreateContainerPayloadWithOptions(
		"alpine:latest",
		nil,
		map[string]string{},
		nil,
		nil,
		"no",
		nil,
		ContainerCreateOptions{CapAdd: []string{"CHOWN", "FOWNER"}},
	)
	hostConfig, ok := payload["HostConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("HostConfig = %#v", payload["HostConfig"])
	}
	got, ok := hostConfig["CapAdd"].([]string)
	if !ok {
		t.Fatalf("CapAdd = %#v, want []string", hostConfig["CapAdd"])
	}
	if strings.Join(got, ",") != "CHOWN,FOWNER" {
		t.Fatalf("CapAdd = %#v, want CHOWN,FOWNER", got)
	}
}

func TestValidateDockerCopyContainerPathRejectsTraversal(t *testing.T) {
	_, err := validateDockerCopyContainerPath("../../etc/shadow")
	if err == nil {
		t.Fatal("expected traversal error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "path traversal") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateDockerCopyContainerPathNormalizesSafePath(t *testing.T) {
	got, err := validateDockerCopyContainerPath("/var/lib//app/data.txt")
	if err != nil {
		t.Fatalf("validateDockerCopyContainerPath returned error: %v", err)
	}
	if got != "/var/lib/app/data.txt" {
		t.Fatalf("normalized path = %q, want %q", got, "/var/lib/app/data.txt")
	}
}

func TestResolveDockerCopyHostPathRejectsEscape(t *testing.T) {
	root := t.TempDir()
	workspaceDir := filepath.Join(root, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	for _, path := range []string{filepath.Join("..", "..", "..", "outside.txt"), filepath.Join("..", "..", "data", "artifact.txt")} {
		if _, err := resolveDockerCopyHostPath(DockerConfig{WorkspaceDir: workspaceDir}, path); err == nil {
			t.Fatalf("expected path escape error for %q", path)
		}
	}
}

func TestResolveDockerCopyHostPathAllowsProjectPath(t *testing.T) {
	root := t.TempDir()
	workspaceDir := filepath.Join(root, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	got, err := resolveDockerCopyHostPath(DockerConfig{WorkspaceDir: workspaceDir}, filepath.Join("..", "data", "artifact.txt"))
	if err != nil {
		t.Fatalf("resolveDockerCopyHostPath returned error: %v", err)
	}
	want := filepath.Join(root, "agent_workspace", "data", "artifact.txt")
	if got != want {
		t.Fatalf("resolved path = %q, want %q", got, want)
	}
}

func TestDockerCopyRejectsInvalidPathsBeforeCLI(t *testing.T) {
	root := t.TempDir()
	workspaceDir := filepath.Join(root, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	result := DockerCopy(DockerConfig{WorkspaceDir: workspaceDir}, "container-1", filepath.Join("..", "..", "..", "outside.txt"), "dest.txt", "to_container")
	if !strings.Contains(result, `"status":"error"`) {
		t.Fatalf("expected error result, got %s", result)
	}
	if !strings.Contains(strings.ToLower(result), "invalid host path") {
		t.Fatalf("expected host path validation error, got %s", result)
	}

	result = DockerCopy(DockerConfig{WorkspaceDir: workspaceDir}, "container-1", "../../etc/shadow", "dest.txt", "from_container")
	if !strings.Contains(result, `"status":"error"`) {
		t.Fatalf("expected error result, got %s", result)
	}
	if !strings.Contains(strings.ToLower(result), "invalid container path") {
		t.Fatalf("expected container path validation error, got %s", result)
	}
}

func TestExtractDockerPortsRejectsUnexpectedPortsType(t *testing.T) {
	_, err := extractDockerPorts(map[string]interface{}{
		"NetworkSettings": map[string]interface{}{
			"Ports": []interface{}{"80/tcp"},
		},
	})
	if err == nil {
		t.Fatal("expected error for unexpected Ports type")
	}
	if !strings.Contains(err.Error(), "unexpected Ports type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractDockerPortsReturnsPortsMap(t *testing.T) {
	ports, err := extractDockerPorts(map[string]interface{}{
		"NetworkSettings": map[string]interface{}{
			"Ports": map[string]interface{}{
				"80/tcp": []interface{}{map[string]interface{}{"HostPort": "8080"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("extractDockerPorts returned error: %v", err)
	}
	encoded, err := json.Marshal(ports)
	if err != nil {
		t.Fatalf("marshal ports: %v", err)
	}
	if !strings.Contains(string(encoded), "8080") {
		t.Fatalf("expected marshaled ports to contain mapped port, got %s", encoded)
	}
}

func TestValidateDockerBindMountRejectsSensitiveHostPaths(t *testing.T) {
	for _, bind := range []string{
		"/var/run/docker.sock:/sock",
		"/etc:/host/etc:ro",
		"/root/.ssh:/ssh",
		"/proc:/host/proc",
		"/sys:/host/sys",
	} {
		if err := validateDockerBindMount(DockerConfig{}, bind); err == nil {
			t.Fatalf("expected bind mount %q to be rejected", bind)
		}
	}
}

func TestValidateDockerBindMountRejectsSensitiveWindowsHostPaths(t *testing.T) {
	for _, bind := range []string{
		`C:\Windows\System32:/host/system32`,
		`C:\Temp\..\Windows\System32:/host/system32`,
		`C:/ProgramData/Docker:/host/docker:ro`,
		`D:\Windows:/host/windows`,
	} {
		if err := validateDockerBindMount(DockerConfig{}, bind); err == nil {
			t.Fatalf("expected Windows bind mount %q to be rejected", bind)
		}
	}
}

func TestValidateDockerBindMountRejectsWindowsWorkspaceEscape(t *testing.T) {
	cfg := DockerConfig{WorkspaceDir: `C:\Users\andi\workspace`}
	if err := validateDockerBindMount(cfg, `C:\Users\andi\other:/data`); err == nil {
		t.Fatal("expected Windows bind mount outside workspace to be rejected")
	}
	if err := validateDockerBindMount(cfg, `C:\Users\andi\workspace\project:/data:ro`); err != nil {
		t.Fatalf("expected Windows bind mount inside workspace to be allowed: %v", err)
	}
}

func TestValidateDockerBindMountRejectsSymlinkOutsideWorkspace(t *testing.T) {
	base := t.TempDir()
	workspace := filepath.Join(base, "workspace")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(workspace, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := validateDockerBindMount(DockerConfig{WorkspaceDir: workspace}, link+":/data"); err == nil {
		t.Fatal("symlink escaped configured workspace")
	}
}

func TestDockerCLIArgsIncludeConfiguredSocketHosts(t *testing.T) {
	tests := []struct {
		name string
		host string
		want []string
	}{
		{
			name: "unix socket",
			host: "unix:///custom/docker.sock",
			want: []string{"-H", "unix:///custom/docker.sock", "ps"},
		},
		{
			name: "windows named pipe",
			host: "npipe:////./pipe/docker_engine",
			want: []string{"-H", "npipe:////./pipe/docker_engine", "ps"},
		},
		{
			name: "tcp host",
			host: "tcp://docker-proxy:2375",
			want: []string{"-H", "tcp://docker-proxy:2375", "ps"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dockerCLIArgs(DockerConfig{Host: tt.host}, "ps")
			if strings.Join(got, "\x00") != strings.Join(tt.want, "\x00") {
				t.Fatalf("dockerCLIArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDockerCreateStatusAcceptsAnySuccessCode(t *testing.T) {
	for _, code := range []int{200, 201, 202, 204} {
		if !dockerCreateSucceeded(code) {
			t.Fatalf("expected create status %d to be accepted", code)
		}
	}
	for _, code := range []int{199, 300, 400, 500} {
		if dockerCreateSucceeded(code) {
			t.Fatalf("expected create status %d to be rejected", code)
		}
	}
}

func TestBuildDockerCreateContainerPayloadIncludesResourceLimits(t *testing.T) {
	payload := buildDockerCreateContainerPayload(
		"aurago/code-studio:latest",
		[]string{"HOME=/home/developer"},
		nil,
		[]string{"/tmp/workspace:/workspace"},
		[]string{"sleep", "infinity"},
		"unless-stopped",
		&ContainerResources{MemoryMB: 4096, CPUCores: 2, PidsLimit: 1024},
	)

	hostConfig, ok := payload["HostConfig"].(map[string]interface{})
	if !ok {
		t.Fatalf("HostConfig type = %T, want map[string]interface{}", payload["HostConfig"])
	}
	if got := hostConfig["Memory"]; got != int64(4096)*1024*1024 {
		t.Fatalf("Memory = %v, want 4GiB bytes", got)
	}
	if got := hostConfig["MemorySwap"]; got != int64(4096)*1024*1024 {
		t.Fatalf("MemorySwap = %v, want 4GiB bytes", got)
	}
	if got := hostConfig["CpuQuota"]; got != int64(200000) {
		t.Fatalf("CpuQuota = %v, want 200000", got)
	}
	if got := hostConfig["CpuPeriod"]; got != int64(100000) {
		t.Fatalf("CpuPeriod = %v, want 100000", got)
	}
	if got := hostConfig["PidsLimit"]; got != int64(1024) {
		t.Fatalf("PidsLimit = %v, want 1024", got)
	}
	if ports, ok := payload["ExposedPorts"].(map[string]interface{}); !ok || len(ports) != 0 {
		t.Fatalf("ExposedPorts = %#v, want empty map when no ports are configured", payload["ExposedPorts"])
	}
}

func TestValidateDockerComposeArgsRejectsHighRiskSubcommands(t *testing.T) {
	for _, command := range []string{
		"run --rm -v /:/host alpine sh",
		"exec app sh -c whoami",
		"cp app:/etc/passwd ./passwd",
		"push app",
		"config --environment",
		"config --environment=true",
		"up -d --env-file /etc/aurago/master.key",
		"config --env-file=/home/aurago/aurago/.env",
	} {
		if err := validateDockerComposeArgs(command); err == nil {
			t.Fatalf("expected compose command %q to be rejected", command)
		}
	}
}

// planComposeOutput parses command like DockerCompose and plans its -o target.
func planComposeOutput(t *testing.T, cfg DockerConfig, command string) (dockerComposeOutputPlan, error) {
	t.Helper()
	parts, err := dockerComposeParts(command)
	if err != nil {
		t.Fatalf("%s: dockerComposeParts() error = %v", command, err)
	}
	return planDockerComposeOutput(cfg, parts)
}

func TestDockerComposeOutputArgsStayInWorkspace(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(filepath.Join(workspace, "rendered"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := DockerConfig{WorkspaceDir: workspace}
	resolvedWorkspace, err := secureResolveFinalPath(workspace)
	if err != nil {
		t.Fatal(err)
	}
	stack := filepath.Join(resolvedWorkspace, "rendered", "stack.yml")
	const staged = "STAGED"
	// Compose never gets the validated path: the output flag is replaced by one
	// pointing at the staging file, at the place of the last output flag.
	for command, wantArgs := range map[string]string{
		"config -o rendered/stack.yml":                                                 "config --output=STAGED",
		"config --output rendered/stack.yml":                                           "config --output=STAGED",
		"config --output=rendered/stack.yml":                                           "config --output=STAGED",
		"config -o=rendered/stack.yml":                                                 "config --output=STAGED",
		"config -orendered/stack.yml":                                                  "config --output=STAGED",
		"config -qo rendered/stack.yml":                                                "config -q --output=STAGED",
		"config -qo=rendered/stack.yml":                                                "config -q --output=STAGED",
		"config --format json -o " + filepath.Join(workspace, "rendered", "stack.yml"): "config --format json --output=STAGED",
		"convert -o rendered/stack.yml":                                                "convert --output=STAGED",
		"config -o rendered/stack.yml -- web":                                          "config --output=STAGED -- web",
		"config -o rendered/stack.yml --services":                                      "config --output=STAGED --services",
		"config -o rendered/other.yml -o rendered/stack.yml":                           "config --output=STAGED",
	} {
		plan, err := planComposeOutput(t, cfg, command)
		if err != nil {
			t.Fatalf("%s: plan error = %v", command, err)
		}
		if !plan.writesFile() || plan.target != stack || plan.rel != filepath.Join("rendered", "stack.yml") || plan.root != resolvedWorkspace {
			t.Fatalf("%s: plan = %+v, want target %s", command, plan, stack)
		}
		if got := strings.Join(plan.argsWithOutput(staged), " "); got != wantArgs {
			t.Fatalf("%s: arguments for Compose = %q, want %q", command, got, wantArgs)
		}
		if strings.Contains(strings.Join(plan.args, " "), "output") || strings.Contains(strings.Join(plan.argsWithOutput(staged), " "), workspace) {
			t.Fatalf("%s: the workspace path must not reach Compose: %q", command, plan.argsWithOutput(staged))
		}
	}
	// `-o=` names a file called "=" (pflag keeps the "=" when nothing follows it),
	// so the argument after it stays a service name.
	plan, err := planComposeOutput(t, cfg, "config -o= web")
	if err != nil || plan.target != filepath.Join(resolvedWorkspace, "=") || strings.Join(plan.argsWithOutput(staged), " ") != "config --output=STAGED web" {
		t.Fatalf("-o= plan = %+v, %v", plan, err)
	}
	// An "=" after other letters of the value belongs to the value: -ostack=1.yml
	// names the file "stack=1.yml".
	plan, err = planComposeOutput(t, cfg, "config -ostack=1.yml")
	if err != nil || plan.target != filepath.Join(resolvedWorkspace, "stack=1.yml") {
		t.Fatalf("-ostack=1.yml plan = %+v, %v", plan, err)
	}
	// An empty --output= writes to stdout: the flag is dropped, nothing to confine.
	plan, err = planComposeOutput(t, cfg, "config --output= --services")
	if err != nil || plan.writesFile() || strings.Join(plan.args, " ") != "config --services" {
		t.Fatalf("empty --output= plan = %+v, %v", plan, err)
	}
	// Compose keeps the last output flag; an empty one last means stdout, but
	// every flag is validated.
	plan, err = planComposeOutput(t, cfg, "config -o rendered/stack.yml --output=")
	if err != nil || plan.writesFile() || strings.Join(plan.args, " ") != "config" {
		t.Fatalf("last empty --output= plan = %+v, %v", plan, err)
	}
	for _, command := range []string{
		"config -o ../outside.yml",
		"config --output=" + filepath.Join(root, "outside.yml"),
		"config -qo ../../etc/cron.d/aurago",
		"config -o",
		"config -qo",
		"config --output",
		"config -o rendered",
		"config -o .env",
		"config --output rendered/vault.bin",
		"config -o ../outside.yml -o rendered/stack.yml",
		"config -o rendered/stack.yml -o ../outside.yml",
		"config -o rendered/stack.yml --output=../outside.yml",
	} {
		_, err := planComposeOutput(t, cfg, command)
		if err == nil {
			t.Fatalf("%s: accepted", command)
		}
		var denied *dockerComposeDeniedError
		if !errors.As(err, &denied) || denied.code != dockerComposeOutputDeniedCode {
			t.Fatalf("%s: error %v is not a %s denial", command, err, dockerComposeOutputDeniedCode)
		}
	}
	// Other subcommands, config without -o and everything after `--` reach
	// Compose unchanged.
	for _, command := range []string{
		"logs -f --tail 20",
		"config --format json --services",
		"config -- -o ../outside.yml",
		"up -d -o x",
		"config -q=foo",
	} {
		plan, err := planComposeOutput(t, cfg, command)
		if err != nil || plan.writesFile() || strings.Join(plan.args, " ") != command {
			t.Fatalf("%s: plan = %+v, %v; want it unchanged", command, plan, err)
		}
	}

	result := DockerCompose(cfg, "compose.yml", "config -o ../outside.yml")
	if !strings.Contains(result, "must stay within the configured workspace") || !strings.Contains(result, `"code":"docker_compose_output_denied"`) {
		t.Fatalf("DockerCompose() = %s, want the coded workspace denial before the CLI runs", result)
	}
	for _, command := range []string{"config --environment", "config --environment=true", "up -d --env-file /etc/aurago/master.key"} {
		result := DockerCompose(cfg, "compose.yml", command)
		if !strings.Contains(result, "is not allowed") || !strings.Contains(result, `"code":"docker_compose_argument_denied"`) {
			t.Fatalf("DockerCompose(%s) = %s, want the coded argument denial before the CLI runs", command, result)
		}
	}
	if result := DockerCompose(cfg, "compose.yml", "exec app sh"); strings.Contains(result, `"code"`) {
		t.Fatalf("other denials keep their envelope: %s", result)
	}
	// Writing a rendered file stays a read-only Compose command, so it keeps
	// working in Docker read-only mode.
	if DockerComposeCommandMutates("config -o rendered/stack.yml") {
		t.Fatal("config -o must stay a read-only Compose command")
	}
}

func TestDockerComposeArgumentsDenial(t *testing.T) {
	for _, command := range []string{"up -d", "config -o rendered/stack.yml", "ps"} {
		if denial := DockerComposeArgumentsDenial(command); denial != "" {
			t.Fatalf("%s: unexpected denial %s", command, denial)
		}
	}
	if denial := DockerComposeArgumentsDenial("config --env-file=/x"); !strings.Contains(denial, `"code":"docker_compose_argument_denied"`) {
		t.Fatalf("--env-file denial = %s", denial)
	}
	denial := DockerComposeArgumentsDenial("exec app sh")
	if !strings.Contains(denial, "not allowed by the safe compose policy") || strings.Contains(denial, `"code"`) {
		t.Fatalf("exec denial = %s, want the plain envelope", denial)
	}
}

func TestDockerComposeOutputArgsRejectAuraGoState(t *testing.T) {
	// Inside the workspace is not enough: AuraGo's own data directory, config,
	// vault and the protected Desktop Notes stay untouchable, like for the file
	// tools, while any other file of the workspace is a valid target.
	workspace := t.TempDir()
	dataDir := filepath.Join(workspace, "data")
	notes := filepath.Join(workspace, "Documents", "Notes")
	for _, dir := range []string{dataDir, notes, filepath.Join(workspace, "rendered")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(workspace, "config.yaml")
	if err := os.WriteFile(configPath, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	protectAuraGoStateForTest(t, dataDir, configPath, notes)
	cfg := DockerConfig{WorkspaceDir: workspace}
	for _, command := range []string{
		"config -o data/short_term.db",
		"config --output=data/new/stack.yml",
		"config -o config.yaml",
		"config -qo CONFIG.YAML",
		"config -o Documents/Notes/rendered.md",
		"config -o rendered/prod.env",
	} {
		if plan, err := planComposeOutput(t, cfg, command); err == nil {
			t.Fatalf("%s: accepted as %+v", command, plan)
		}
	}
	// An existing rendered file may be overwritten.
	if err := os.WriteFile(filepath.Join(workspace, "rendered", "stack.yml"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if plan, err := planComposeOutput(t, cfg, "config -o rendered/stack.yml"); err != nil || !plan.writesFile() {
		t.Fatalf("existing workspace file rejected: %+v, %v", plan, err)
	}
}

// protectAuraGoStateForTest configures the runtime snapshot's protected paths.
func protectAuraGoStateForTest(t *testing.T, dataDir, configPath, notes string) {
	t.Helper()
	previous, configured := currentRuntimePermissions()
	perms := RuntimePermissions{ProtectedDataDir: dataDir, ProtectedSystemFiles: []string{configPath}}
	if notes != "" {
		perms.ProtectedNotesRoots = []string{notes}
	}
	ConfigureRuntimePermissions(perms)
	t.Cleanup(func() {
		if configured {
			ConfigureRuntimePermissions(previous)
		} else {
			ClearRuntimePermissionsForTest()
		}
	})
}

func TestDockerComposeOutputArgsWithoutWorkspaceStayInWorkingDirectory(t *testing.T) {
	// Without a configured workspace the agent Compose preflight confines the
	// compose file to the process working directory; -o uses the same root, and
	// the denial says so.
	workdir := t.TempDir()
	t.Chdir(workdir)
	resolvedWorkdir, err := secureResolveFinalPath(workdir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := DockerConfig{}
	plan, err := planComposeOutput(t, cfg, "config -o rendered/stack.yml")
	if err != nil {
		t.Fatalf("plan error = %v", err)
	}
	if want := filepath.Join(resolvedWorkdir, "rendered", "stack.yml"); plan.target != want || plan.root != resolvedWorkdir {
		t.Fatalf("plan = %+v, want target %s", plan, want)
	}
	for _, command := range []string{
		"config -o ../outside.yml",
		"config --output=" + filepath.Join(filepath.Dir(resolvedWorkdir), "outside.yml"),
	} {
		_, err := planComposeOutput(t, cfg, command)
		if err == nil {
			t.Fatalf("%s: accepted", command)
		}
		if !strings.Contains(err.Error(), "AuraGo's working directory") || strings.Contains(err.Error(), "configured workspace") {
			t.Fatalf("%s: denial %q must name the working directory", command, err)
		}
	}
}

func TestDockerComposeOutputArgsRejectSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root, filepath.Join(workspace, "link")); err != nil {
		t.Skipf("symlinks cannot be created here (%v); Go does not resolve Windows junctions, as for every other jail check", err)
	}
	cfg := DockerConfig{WorkspaceDir: workspace}
	for _, command := range []string{"config -o link/escape.yml", "config --output=link", "config -o link/workspace/../x.yml"} {
		if plan, err := planComposeOutput(t, cfg, command); err == nil {
			t.Fatalf("%s: symlink to the workspace parent escaped as %+v", command, plan)
		}
	}
}

// stubComposeRunner replaces the Compose CLI. run sees the Compose arguments
// and the path of the --output file (empty when there is none).
func stubComposeRunner(t *testing.T, run func(args []string, output string) string) {
	t.Helper()
	previous := dockerComposeCLIRunner
	dockerComposeCLIRunner = func(cfg DockerConfig, args ...string) string {
		output := ""
		for _, arg := range args {
			if value, ok := strings.CutPrefix(arg, "--output="); ok {
				output = value
			}
		}
		return run(args, output)
	}
	t.Cleanup(func() { dockerComposeCLIRunner = previous })
}

const composeOKResult = `{"output":"","status":"ok"}`

func TestDockerComposeConfigOutputIsStagedAndPublished(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true, DockerReadOnly: true})
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	workspace := t.TempDir()
	resolvedWorkspace, err := secureResolveFinalPath(workspace)
	if err != nil {
		t.Fatal(err)
	}
	cfg := DockerConfig{WorkspaceDir: workspace}
	rendered := filepath.Join(resolvedWorkspace, "rendered", "stack.yml")
	if err := os.MkdirAll(filepath.Dir(rendered), 0o755); err != nil {
		t.Fatal(err)
	}

	var stagedPath string
	writeStaged := func(args []string, output string) string {
		if output == "" {
			return composeOKResult
		}
		stagedPath = output
		if strings.HasPrefix(output, resolvedWorkspace) || strings.HasPrefix(output, workspace) {
			t.Errorf("Compose was handed a path inside the workspace: %s", output)
		}
		if err := os.WriteFile(output, []byte("services: {}\n"), 0o600); err != nil {
			t.Errorf("Compose cannot write the staging file: %v", err)
		}
		return composeOKResult
	}

	t.Run("published into the workspace, also read-only, staging removed", func(t *testing.T) {
		stubComposeRunner(t, writeStaged)
		result := DockerCompose(cfg, "compose.yml", "config -o rendered/stack.yml")
		var payload map[string]any
		if err := json.Unmarshal([]byte(result), &payload); err != nil || payload["status"] != "ok" || payload["output_file"] != rendered {
			t.Fatalf("result = %s (%v), want ok with output_file %s", result, err, rendered)
		}
		if data, err := os.ReadFile(rendered); err != nil || string(data) != "services: {}\n" {
			t.Fatalf("published file = %q, %v", data, err)
		}
		if _, err := os.Stat(filepath.Dir(stagedPath)); !os.IsNotExist(err) {
			t.Fatalf("staging directory %s was not removed (%v)", filepath.Dir(stagedPath), err)
		}
	})

	t.Run("an existing target is replaced, never written through a hardlink", func(t *testing.T) {
		alias := filepath.Join(resolvedWorkspace, "alias.txt")
		if err := os.Remove(alias); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := os.WriteFile(rendered, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(rendered, alias); err != nil {
			t.Skipf("hardlinks cannot be created here: %v", err)
		}
		stubComposeRunner(t, writeStaged)
		if result := DockerCompose(cfg, "compose.yml", "config -o rendered/stack.yml"); !strings.Contains(result, `"status":"ok"`) {
			t.Fatalf("result = %s", result)
		}
		if data, _ := os.ReadFile(alias); string(data) != "old" {
			t.Fatalf("hardlinked alias was written through: %q", data)
		}
		if data, _ := os.ReadFile(rendered); string(data) != "services: {}\n" {
			t.Fatalf("target = %q, want the rendered file", data)
		}
	})

	t.Run("a missing parent directory below the workspace is created", func(t *testing.T) {
		stubComposeRunner(t, writeStaged)
		result := DockerCompose(cfg, "compose.yml", "config --output=new/dir/stack.yml")
		want := filepath.Join(resolvedWorkspace, "new", "dir", "stack.yml")
		if !strings.Contains(result, `"status":"ok"`) || !strings.Contains(result, "output_file") {
			t.Fatalf("result = %s", result)
		}
		if data, err := os.ReadFile(want); err != nil || string(data) != "services: {}\n" {
			t.Fatalf("published file = %q, %v", data, err)
		}
	})

	t.Run("the last output flag is the one written", func(t *testing.T) {
		stubComposeRunner(t, writeStaged)
		if result := DockerCompose(cfg, "compose.yml", "config -o first.yml -o rendered/last.yml"); !strings.Contains(result, `"status":"ok"`) {
			t.Fatalf("result = %s", result)
		}
		if _, err := os.Stat(filepath.Join(resolvedWorkspace, "first.yml")); !os.IsNotExist(err) {
			t.Fatalf("first.yml must not be written (%v)", err)
		}
		if _, err := os.Stat(filepath.Join(resolvedWorkspace, "rendered", "last.yml")); err != nil {
			t.Fatalf("last.yml missing: %v", err)
		}
	})

	t.Run("-q and list flags write nothing and publish nothing", func(t *testing.T) {
		stubComposeRunner(t, func(args []string, output string) string { return composeOKResult })
		result := DockerCompose(cfg, "compose.yml", "config -qo rendered/quiet.yml")
		if result != composeOKResult {
			t.Fatalf("result = %s, want the plain Compose result", result)
		}
		if _, err := os.Stat(filepath.Join(resolvedWorkspace, "rendered", "quiet.yml")); !os.IsNotExist(err) {
			t.Fatalf("quiet.yml must not exist (%v)", err)
		}
	})

	t.Run("an empty --output= reaches Compose without an output flag", func(t *testing.T) {
		var seen []string
		stubComposeRunner(t, func(args []string, output string) string {
			seen = args
			return composeOKResult
		})
		if result := DockerCompose(cfg, "compose.yml", "config --output= --services"); result != composeOKResult {
			t.Fatalf("result = %s", result)
		}
		if strings.Contains(strings.Join(seen, " "), "output") || seen[len(seen)-1] != "--services" {
			t.Fatalf("Compose arguments = %q", seen)
		}
	})

	t.Run("a failed Compose call publishes nothing", func(t *testing.T) {
		failed := filepath.Join(resolvedWorkspace, "rendered", "failed.yml")
		stubComposeRunner(t, func(args []string, output string) string {
			_ = os.WriteFile(output, []byte("partial"), 0o600)
			return errJSON("Command failed: exit status 1")
		})
		result := DockerCompose(cfg, "compose.yml", "config -o rendered/failed.yml")
		if !strings.Contains(result, "Command failed") {
			t.Fatalf("result = %s", result)
		}
		if _, err := os.Stat(failed); !os.IsNotExist(err) {
			t.Fatalf("failed.yml must not exist (%v)", err)
		}
	})

	t.Run("a directory swapped in after validation is refused", func(t *testing.T) {
		target := filepath.Join(resolvedWorkspace, "rendered", "swapped.yml")
		stubComposeRunner(t, func(args []string, output string) string {
			if err := os.WriteFile(output, []byte("services: {}\n"), 0o600); err != nil {
				t.Error(err)
			}
			if err := os.Mkdir(target, 0o755); err != nil {
				t.Error(err)
			}
			return composeOKResult
		})
		result := DockerCompose(cfg, "compose.yml", "config -o rendered/swapped.yml")
		if !strings.Contains(result, `"status":"error"`) || !strings.Contains(result, "not a regular file") {
			t.Fatalf("result = %s, want a refusal", result)
		}
	})

	t.Run("a symlink swapped in after validation is replaced or refused, never followed", func(t *testing.T) {
		outside := filepath.Join(t.TempDir(), "outside.txt")
		if err := os.WriteFile(outside, []byte("outside"), 0o644); err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(resolvedWorkspace, "rendered", "linked.yml")
		stubComposeRunner(t, func(args []string, output string) string {
			if err := os.WriteFile(output, []byte("services: {}\n"), 0o600); err != nil {
				t.Error(err)
			}
			if err := os.Symlink(outside, target); err != nil {
				t.Skipf("symlinks cannot be created here: %v", err)
			}
			return composeOKResult
		})
		result := DockerCompose(cfg, "compose.yml", "config -o rendered/linked.yml")
		if !strings.Contains(result, `"status":"error"`) {
			t.Fatalf("result = %s, want a refusal", result)
		}
		if data, _ := os.ReadFile(outside); string(data) != "outside" {
			t.Fatalf("the symlink target outside the workspace was written: %q", data)
		}
	})
}

func TestDockerComposeConvertIsReadOnlyLikeConfig(t *testing.T) {
	for _, command := range []string{"convert", "convert --format json", "convert -o rendered/stack.yml"} {
		if DockerComposeCommandMutates(command) {
			t.Fatalf("%s must be a read-only Compose command, like config", command)
		}
	}
	for _, command := range []string{"up -d", "down", "build", "pull", "create"} {
		if !DockerComposeCommandMutates(command) {
			t.Fatalf("%s must stay mutating", command)
		}
	}
}

func TestValidateDockerBindMountAcceptsBindsInsideAWorkspaceUnderASensitiveLocation(t *testing.T) {
	workspaces := []string{"/mnt/user/appdata/aurago/workdir", "/root/aurago/agent_workspace/workdir"}
	if runtime.GOOS == "windows" {
		workspaces = []string{`C:\ProgramData\AuraGo\agent_workspace\workdir`}
	}
	for _, workspace := range workspaces {
		if _, err := secureResolveFinalPath(filepath.Clean(workspace)); err != nil {
			t.Logf("%s: skipped, the test user cannot resolve it: %v", workspace, err)
			continue
		}
		cfg := DockerConfig{WorkspaceDir: workspace}
		inside := strings.TrimRight(dockerutil.NormalizeHostPathForBind(workspace), "/") + "/stack/data:/data"
		if err := validateDockerBindMount(cfg, inside); err != nil {
			t.Fatalf("%s: bind inside the workspace rejected: %v", workspace, err)
		}
	}
	// Outside the workspace, a sensitive location below it, and a workspace that
	// is itself a sensitive location keep today's rejection.
	for _, tc := range []struct{ workspace, bind string }{
		{"/mnt/user/appdata/aurago/workdir", "/mnt/user/other:/o"},
		{"/var/lib", "/var/lib/docker/volumes:/v"},
		{"/var/run", "/var/run/docker.sock:/s"},
		{"/", "/:/host"},
		{"/etc", "/etc/shadow:/s"},
	} {
		if err := validateDockerBindMount(DockerConfig{WorkspaceDir: tc.workspace}, tc.bind); err == nil {
			t.Fatalf("workspace %s accepted %s", tc.workspace, tc.bind)
		}
	}
}

func TestDockerComposeOutputPublishRefusalsAreCoded(t *testing.T) {
	ConfigureRuntimePermissions(RuntimePermissions{DockerEnabled: true})
	t.Cleanup(func() { ConfigureRuntimePermissions(defaultRuntimePermissionsForTests()) })
	workspace := t.TempDir()
	resolved, err := secureResolveFinalPath(workspace)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(resolved, "rendered", "stack.yml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	stubComposeRunner(t, func(args []string, output string) string {
		if err := os.Mkdir(target, 0o755); err != nil { // a directory appears while Compose renders
			t.Error(err)
		}
		if output != "" {
			_ = os.WriteFile(output, []byte("services: {}\n"), 0o600)
		}
		return composeOKResult
	})
	result := DockerCompose(DockerConfig{WorkspaceDir: workspace}, "compose.yml", "config -o rendered/stack.yml")
	if !strings.Contains(result, `"code":"docker_compose_output_denied"`) || !strings.Contains(result, "not a regular file") {
		t.Fatalf("result = %s, want the coded publish refusal", result)
	}
}

// Publishing creates missing folders one by one and confirms each, on every
// platform.
func TestDockerComposeOutputPublishCreatesMissingFolders(t *testing.T) {
	workspace := t.TempDir()
	resolved, err := secureResolveFinalPath(workspace)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := planComposeOutput(t, DockerConfig{WorkspaceDir: workspace}, "config -o a/b/c/stack.yml")
	if err != nil {
		t.Fatal(err)
	}
	staged := filepath.Join(t.TempDir(), "out")
	if err := os.WriteFile(staged, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if ok, err := plan.publish(staged); !ok || err != nil {
		t.Fatalf("publish into new folders: %v, %v", ok, err)
	}
	data, err := os.ReadFile(filepath.Join(resolved, "a", "b", "c", "stack.yml"))
	if err != nil || string(data) != "services: {}\n" {
		t.Fatalf("published file = %q, %v", data, err)
	}
	// A second publish finds the folders and replaces the file.
	if ok, err := plan.publish(staged); !ok || err != nil {
		t.Fatalf("publish into existing folders: %v, %v", ok, err)
	}
}

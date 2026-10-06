package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	want := "--output=" + filepath.Join(resolvedWorkspace, "rendered", "stack.yml")
	for _, command := range []string{
		"config -o rendered/stack.yml",
		"config --output rendered/stack.yml",
		"config --output=rendered/stack.yml",
		"config -o=rendered/stack.yml",
		"config -orendered/stack.yml",
		"config -qo rendered/stack.yml",
		"config --format json -o " + filepath.Join(workspace, "rendered", "stack.yml"),
		"convert -o rendered/stack.yml",
	} {
		parts, err := dockerComposeParts(command)
		if err != nil {
			t.Fatalf("%s: dockerComposeParts() error = %v", command, err)
		}
		got, err := dockerComposeRewriteOutputArgs(cfg, parts)
		if err != nil {
			t.Fatalf("%s: rewrite error = %v", command, err)
		}
		if strings.Join(got, " ") == strings.Join(parts, " ") || !strings.Contains(strings.Join(got, "\x00"), want) {
			t.Fatalf("%s: rewritten = %q, want %s", command, got, want)
		}
		if strings.HasPrefix(command, "config -qo") && got[1] != "-q" {
			t.Fatalf("%s: quiet flag lost: %q", command, got)
		}
	}
	for _, command := range []string{
		"config -o ../outside.yml",
		"config --output=" + filepath.Join(root, "outside.yml"),
		"config -qo ../../etc/cron.d/aurago",
		"config -o",
		"config -o rendered",
		"config -o .env",
		"config --output rendered/vault.bin",
	} {
		parts, err := dockerComposeParts(command)
		if err != nil {
			t.Fatalf("%s: dockerComposeParts() error = %v", command, err)
		}
		if got, err := dockerComposeRewriteOutputArgs(cfg, parts); err == nil {
			t.Fatalf("%s: accepted as %q", command, got)
		}
	}
	link := filepath.Join(workspace, "link")
	if err := os.Symlink(root, link); err == nil {
		// Skipped where symlinks cannot be created (Windows without the privilege;
		// Go does not resolve Windows junctions, as for every other jail check).
		for _, command := range []string{"config -o link/escape.yml", "config --output=link"} {
			parts, _ := dockerComposeParts(command)
			if got, err := dockerComposeRewriteOutputArgs(cfg, parts); err == nil {
				t.Fatalf("%s: symlink to the workspace parent escaped as %q", command, got)
			}
		}
	}
	parts, _ := dockerComposeParts("logs -f --tail 20")
	if got, err := dockerComposeRewriteOutputArgs(cfg, parts); err != nil || strings.Join(got, " ") != "logs -f --tail 20" {
		t.Fatalf("other subcommands must stay unchanged: %q, %v", got, err)
	}
	parts, _ = dockerComposeParts("config --format json --services")
	if got, err := dockerComposeRewriteOutputArgs(cfg, parts); err != nil || strings.Join(got, " ") != "config --format json --services" {
		t.Fatalf("config without -o must stay unchanged: %q, %v", got, err)
	}
	// After `--` Compose reads service names, so nothing there is an output flag.
	parts, _ = dockerComposeParts("config -- -o ../outside.yml")
	if got, err := dockerComposeRewriteOutputArgs(cfg, parts); err != nil || strings.Join(got, " ") != "config -- -o ../outside.yml" {
		t.Fatalf("arguments after -- must stay unchanged: %q, %v", got, err)
	}
	parts, _ = dockerComposeParts("config -qo rendered/stack.yml -- web")
	if got, err := dockerComposeRewriteOutputArgs(cfg, parts); err != nil ||
		strings.Join(got, "\x00") != "config\x00-q\x00"+want+"\x00--\x00web" {
		t.Fatalf("output before -- must be rewritten and the service kept: %q, %v", got, err)
	}
	if result := DockerCompose(cfg, "compose.yml", "config -o ../outside.yml"); !strings.Contains(result, "must stay within the configured workspace") {
		t.Fatalf("DockerCompose() = %s, want the workspace denial before the CLI runs", result)
	}
	if result := DockerCompose(cfg, "compose.yml", "config --environment"); !strings.Contains(result, "is not allowed") {
		t.Fatalf("DockerCompose() = %s, want --environment rejected before the CLI runs", result)
	}
	// Writing a rendered file stays a read-only Compose command, so it keeps
	// working in Docker read-only mode.
	if DockerComposeCommandMutates("config -o rendered/stack.yml") {
		t.Fatal("config -o must stay a read-only Compose command")
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
	previous, configured := currentRuntimePermissions()
	ConfigureRuntimePermissions(RuntimePermissions{
		ProtectedDataDir:     dataDir,
		ProtectedSystemFiles: []string{configPath},
		ProtectedNotesRoots:  []string{notes},
	})
	t.Cleanup(func() {
		if configured {
			ConfigureRuntimePermissions(previous)
		} else {
			ClearRuntimePermissionsForTest()
		}
	})
	cfg := DockerConfig{WorkspaceDir: workspace}
	for _, command := range []string{
		"config -o data/short_term.db",
		"config --output=data/new/stack.yml",
		"config -o config.yaml",
		"config -qo CONFIG.YAML",
		"config -o Documents/Notes/rendered.md",
		"config -o rendered/prod.env",
	} {
		parts, err := dockerComposeParts(command)
		if err != nil {
			t.Fatalf("%s: dockerComposeParts() error = %v", command, err)
		}
		if got, err := dockerComposeRewriteOutputArgs(cfg, parts); err == nil {
			t.Fatalf("%s: accepted as %q", command, got)
		}
	}
	// An existing rendered file may be overwritten.
	if err := os.WriteFile(filepath.Join(workspace, "rendered", "stack.yml"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	parts, _ := dockerComposeParts("config -o rendered/stack.yml")
	if got, err := dockerComposeRewriteOutputArgs(cfg, parts); err != nil || len(got) != 2 || !strings.HasPrefix(got[1], "--output=") {
		t.Fatalf("existing workspace file rejected: %q, %v", got, err)
	}
}

func TestDockerComposeOutputArgsWithoutWorkspaceStayInWorkingDirectory(t *testing.T) {
	// Without a configured workspace the agent Compose preflight confines the
	// compose file to the process working directory; -o uses the same root.
	workdir := t.TempDir()
	t.Chdir(workdir)
	resolvedWorkdir, err := secureResolveFinalPath(workdir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := DockerConfig{}
	parts, _ := dockerComposeParts("config -o rendered/stack.yml")
	got, err := dockerComposeRewriteOutputArgs(cfg, parts)
	if err != nil {
		t.Fatalf("rewrite error = %v", err)
	}
	want := "--output=" + filepath.Join(resolvedWorkdir, "rendered", "stack.yml")
	if strings.Join(got, "\x00") != "config\x00"+want {
		t.Fatalf("rewritten = %q, want config %s", got, want)
	}
	for _, command := range []string{
		"config -o ../outside.yml",
		"config --output=" + filepath.Join(filepath.Dir(resolvedWorkdir), "outside.yml"),
	} {
		parts, _ := dockerComposeParts(command)
		if got, err := dockerComposeRewriteOutputArgs(cfg, parts); err == nil {
			t.Fatalf("%s: accepted as %q", command, got)
		}
	}
}

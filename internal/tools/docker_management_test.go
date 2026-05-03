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

	_, err := resolveDockerCopyHostPath(DockerConfig{WorkspaceDir: workspaceDir}, filepath.Join("..", "..", "..", "outside.txt"))
	if err == nil {
		t.Fatal("expected path escape error")
	}
}

func TestResolveDockerCopyHostPathAllowsProjectPath(t *testing.T) {
	root := t.TempDir()
	workspaceDir := filepath.Join(root, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}

	got, err := resolveDockerCopyHostPath(DockerConfig{WorkspaceDir: workspaceDir}, filepath.Join("..", "..", "data", "artifact.txt"))
	if err != nil {
		t.Fatalf("resolveDockerCopyHostPath returned error: %v", err)
	}
	want := filepath.Join(root, "data", "artifact.txt")
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
	} {
		if err := validateDockerComposeArgs(command); err == nil {
			t.Fatalf("expected compose command %q to be rejected", command)
		}
	}
}

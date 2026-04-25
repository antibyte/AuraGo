package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

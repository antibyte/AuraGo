package tools

import (
	"strings"
	"testing"
)

func TestContainerSandboxLandlockGrantAddsSocketAndReadOnlyWorkspace(t *testing.T) {
	const (
		rawWorkspace = "/opt/aurago/workspace"
		rawSocket    = "/var/run/docker.sock"
		rawCustom    = "/custom/docker.sock"
	)
	workspace := grantedDockerSocketSegment(rawWorkspace)
	socket := grantedDockerSocketSegment(rawSocket)
	custom := grantedDockerSocketSegment(rawCustom)
	base := []string{
		"AURAGO_SBX_RW=/work",
		"AURAGO_SBX_RO=/usr",
		"AURAGO_SBX_RW_REQUIRED=/work",
		"AURAGO_SBX_RO_REQUIRED=/usr",
	}

	t.Cleanup(func() {
		SetShellSandboxDockerHost("")
		ConfigureRuntimePermissions(defaultRuntimePermissionsForTests())
	})
	ConfigureRuntimePermissions(RuntimePermissions{})
	SetShellSandboxDockerHost("unix://" + rawSocket)

	got := applyContainerSandboxLandlockGrant("llm-sandbox", append([]string{}, base...), rawWorkspace, "")
	if !grantListHas(got, "AURAGO_SBX_RW", socket) || !grantListHas(got, "AURAGO_SBX_RW_REQUIRED", socket) {
		t.Fatalf("socket missing without docker tool permission: %v", got)
	}
	if !grantListHas(got, "AURAGO_SBX_RO", workspace) || !grantListHas(got, "AURAGO_SBX_RO_REQUIRED", workspace) {
		t.Fatalf("workspace not read-only: %v", got)
	}
	if grantListHas(got, "AURAGO_SBX_RW", workspace) || grantListHas(got, "AURAGO_SBX_RW_REQUIRED", workspace) {
		t.Fatalf("workspace must stay read-only: %v", got)
	}

	other := applyContainerSandboxLandlockGrant("filesystem", append([]string{}, base...), rawWorkspace, "unix://"+rawSocket)
	if strings.Join(other, "\n") != strings.Join(base, "\n") {
		t.Fatalf("other MCP name changed env:\n%s", strings.Join(other, "\n"))
	}

	SetShellSandboxDockerHost("unix://" + rawSocket)
	customGrant := applyContainerSandboxLandlockGrant("llm-sandbox", append([]string{}, base...), rawWorkspace, "unix://"+rawCustom)
	if !grantListHas(customGrant, "AURAGO_SBX_RW", custom) || grantListHas(customGrant, "AURAGO_SBX_RW", socket) {
		t.Fatalf("sandbox unix host = %v, want %s", customGrant, custom)
	}

	tcp := applyContainerSandboxLandlockGrant("llm-sandbox", append([]string{}, base...), rawWorkspace, "tcp://127.0.0.1:2375")
	if grantListHas(tcp, "AURAGO_SBX_RW", socket) || grantListHas(tcp, "AURAGO_SBX_RW", custom) {
		t.Fatalf("tcp sandbox host added a socket: %v", tcp)
	}
	if !grantListHas(tcp, "AURAGO_SBX_RO", workspace) {
		t.Fatalf("tcp host dropped the workspace grant: %v", tcp)
	}

	SetShellSandboxDockerHost("tcp://127.0.0.1:2375")
	publishedTCP := applyContainerSandboxLandlockGrant("llm-sandbox", append([]string{}, base...), rawWorkspace, "")
	if grantListHas(publishedTCP, "AURAGO_SBX_RW", socket) {
		t.Fatalf("published tcp host added a socket: %v", publishedTCP)
	}
}

func TestSandboxNotInstalledStatusNamesPinnedRequirement(t *testing.T) {
	docker := sandboxNotInstalledStatus("docker")
	if !strings.Contains(docker, "llm-sandbox[mcp-docker]==0.3.45") {
		t.Fatalf("docker status = %q", docker)
	}
	podman := sandboxNotInstalledStatus("podman")
	if !strings.Contains(podman, "llm-sandbox[mcp-podman]==0.3.45") {
		t.Fatalf("podman status = %q", podman)
	}
}

func grantListHas(env []string, key, path string) bool {
	prefix := key + "="
	for _, kv := range env {
		if !strings.HasPrefix(kv, prefix) {
			continue
		}
		for _, entry := range strings.Split(strings.TrimPrefix(kv, prefix), ":") {
			if entry == path {
				return true
			}
		}
	}
	return false
}

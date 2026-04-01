package tools

import "testing"

func TestNormalizeDockerNamedPipeHost_DefaultPath(t *testing.T) {
	got, err := normalizeDockerNamedPipeHost("npipe:////./pipe/docker_engine")
	if err != nil {
		t.Fatalf("normalizeDockerNamedPipeHost returned error: %v", err)
	}
	if got != `\\.\pipe\docker_engine` {
		t.Fatalf("named pipe path = %q, want %q", got, `\\.\pipe\docker_engine`)
	}
}

func TestNormalizeDockerNamedPipeHost_ShorthandPath(t *testing.T) {
	got, err := normalizeDockerNamedPipeHost("npipe://./pipe/docker_engine")
	if err != nil {
		t.Fatalf("normalizeDockerNamedPipeHost returned error: %v", err)
	}
	if got != `\\.\pipe\docker_engine` {
		t.Fatalf("named pipe path = %q, want %q", got, `\\.\pipe\docker_engine`)
	}
}

func TestNormalizeDockerNamedPipeHost_RejectsInvalidPath(t *testing.T) {
	if _, err := normalizeDockerNamedPipeHost("npipe://docker_engine"); err == nil {
		t.Fatal("expected invalid named pipe path error")
	}
}

package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOllamaEmbeddingPayloadHasLimits(t *testing.T) {
	raw, err := ollamaEmbeddingsCreatePayload(11435, GPUInfo{})
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "CapDrop") || strings.Contains(text, "ReadonlyRootfs") {
		t.Fatalf("payload drops capabilities or sets a read-only root: %s", text)
	}
	var body ollamaCreateBody
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.HostConfig.Binds) != 1 || body.HostConfig.Binds[0] != ollamaEmbeddingsVolumeName+":/root/.ollama" {
		t.Fatalf("binds = %#v", body.HostConfig.Binds)
	}
	bind := body.HostConfig.PortBindings["11434/tcp"]
	if len(bind) != 1 || bind[0].HostIp != "127.0.0.1" || bind[0].HostPort != "11435" {
		t.Fatalf("port binding = %#v", bind)
	}
	if body.HostConfig.Memory != 8589934592 {
		t.Fatalf("memory = %d, want 8589934592", body.HostConfig.Memory)
	}
	if body.HostConfig.PidsLimit != 512 {
		t.Fatalf("pids = %d", body.HostConfig.PidsLimit)
	}
	if !stringSetEqual(body.HostConfig.SecurityOpt, []string{"no-new-privileges:true"}) {
		t.Fatalf("security = %#v", body.HostConfig.SecurityOpt)
	}
}

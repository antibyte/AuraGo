package tools

import (
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

func TestOllamaEmbeddingPullCollapsesOverlappingCalls(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	started := make(chan struct{})
	done := make(chan struct{})
	finished := make(chan struct{})
	var once sync.Once
	orig := ollamaModelPullFn
	ollamaModelPullFn = func(port int, model, label string, logger interface {
		Info(string, ...any)
		Warn(string, ...any)
		Error(string, ...any)
	}) {
		defer close(finished)
		calls.Add(1)
		once.Do(func() { close(started) })
		<-release
		close(done)
	}
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		select {
		case <-finished:
		case <-time.After(2 * time.Second):
		}
		ollamaModelPullFn = orig
	})

	startOllamaModelPull(11435, "nomic-embed-text", "Ollama Embeddings", ollamaTestLogger{})
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("pull did not start")
	}
	startOllamaModelPull(11435, "nomic-embed-text", "Ollama Embeddings", ollamaTestLogger{})
	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 1 {
		t.Fatalf("pulls = %d, want 1", calls.Load())
	}
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pull did not finish")
	}
}

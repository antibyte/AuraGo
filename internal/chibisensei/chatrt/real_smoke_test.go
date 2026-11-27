package chatrt

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/llamacpp"
	"aurago/internal/onnxrt"
)

// TestRealLlamaServerChat runs the pinned CPU llama-server against a real GGUF. It is the discovery run of the
// stream framing and the error answers; it downloads only the CPU runtime archive of the pinned build.
//
//	AURAGO_RUN_LLAMA_SMOKE=1 CHIBI_SMOKE_GGUF=<gguf> [AURAGO_LLAMA_SMOKE_CACHE=<dir>] [CHIBI_SMOKE_RECORD=<dir>] \
//	  go test ./internal/chibisensei/chatrt -run '^TestRealLlamaServerChat$' -count=1 -v -timeout 20m
func TestRealLlamaServerChat(t *testing.T) {
	if os.Getenv("AURAGO_RUN_LLAMA_SMOKE") != "1" {
		t.Skip("set AURAGO_RUN_LLAMA_SMOKE=1 to run the real llama-server smoke test")
	}
	gguf := os.Getenv("CHIBI_SMOKE_GGUF")
	if gguf == "" {
		t.Skip("set CHIBI_SMOKE_GGUF to a chat GGUF")
	}
	info, err := os.Stat(gguf)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("CHIBI_SMOKE_GGUF is not a regular file: %v", err)
	}
	cache := firstEnv("AURAGO_LLAMA_SMOKE_CACHE", "CHIBI_SMOKE_CACHE")
	if cache == "" {
		cache = t.TempDir()
	}
	recordDir := os.Getenv("CHIBI_SMOKE_RECORD")
	if recordDir != "" {
		if err := os.MkdirAll(recordDir, 0o755); err != nil {
			t.Fatalf("record dir: %v", err)
		}
	}
	record := func(name, content string) {
		if recordDir == "" {
			return
		}
		if err := os.WriteFile(filepath.Join(recordDir, name), []byte(content), 0o644); err != nil {
			t.Errorf("record %s: %v", name, err)
		}
	}
	launcher := NativeLauncher()
	assets, ok := launcher.Assets("cpu")
	if !ok {
		t.Skip("no llama.cpp CPU archive for this platform")
	}
	cpu := assets[:1]

	// Phase 1: install.
	t.Logf("phase 1: install %s (%d bytes)", cpu[0].ID, cpu[0].Size)
	var lastLogged int64
	err = launcher.Install(context.Background(), cache, cpu, func(id string, done int64) {
		if done-lastLogged >= 4<<20 || done == cpu[0].Size {
			lastLogged = done
			t.Logf("  install %s: %d bytes", id, done)
		}
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !launcher.Verify(context.Background(), cache, cpu) {
		t.Fatal("verify after install failed")
	}

	// Phase 2: version and help.
	assetCache := onnxrt.NewAssetCache(cache, assetUserAgent, nil)
	root, err := assetCache.VerifyRuntime(context.Background(), cpu[0])
	if err != nil {
		t.Fatalf("runtime root: %v", err)
	}
	server, err := llamacpp.FindServer(root)
	if err != nil {
		t.Fatal(err)
	}
	libs, err := (&nativeLauncher{}).libraryRoots(context.Background(), cache, cpu, []string{root})
	if err != nil {
		t.Fatalf("library roots: %v", err)
	}
	version := runServerOnce(t, server, libs, "--version")
	t.Logf("phase 2: --version:\n%s", version)
	record("version.txt", version)
	help := runServerOnce(t, server, libs, "--help")
	record("help.txt", help)
	filter := regexp.MustCompile(`(?m)^.*(--reasoning|--jinja|--no-webui|--webui|--api-key|--threads|--ctx-size|--parallel|--n-gpu-layers|--no-slots|--slots).*$`)
	t.Logf("phase 2: --help (filtered):\n%s", strings.Join(filter.FindAllString(help, -1), "\n"))

	// Phase 3: launch.
	t.Log("phase 3: launch")
	started := time.Now()
	proc, err := launcher.Launch(context.Background(), LaunchSpec{
		Key: "tuned:smoke", CacheDir: cache, Assets: cpu, Backend: "cpu", ModelPath: gguf,
		ContextSize: 1024, Threads: smokeThreads(),
	})
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	native := proc.(*nativeProcess)
	t.Logf("  llama-server pid %d healthy after %s", native.handle.pid, time.Since(started).Round(time.Millisecond))
	stopped := false
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("server log tail at failure: %s", native.logTail())
			record("log_tail_failure.txt", native.logTail())
		}
		if !stopped {
			_ = proc.Stop()
		}
	})

	req := ChatRequest{
		Messages: []Message{{Role: "user", Content: "Say hello in five words."}},
		Sampling: Sampling{Temperature: 0.7, TopP: 0.8, TopK: 20, RepeatPenalty: 1.0, PresencePenalty: 1.5, MaxTokens: 32},
	}

	// Phase 4: the raw stream.
	t.Log("phase 4: raw stream")
	raw := rawPost(t, native, buildChatBody(req), 0)
	t.Logf("  status %d content-type %q", raw.status, raw.contentType)
	for _, line := range raw.lines {
		t.Logf("  | %s", line)
	}
	record("stream.txt", strings.Join(raw.lines, "\n")+"\n")
	checkNoReasoning(t, raw.lines)

	// Phase 5: the parsed path.
	t.Log("phase 5: parsed chat")
	var text strings.Builder
	stats, err := proc.Chat(context.Background(), req, func(s string) { text.WriteString(s) })
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	t.Logf("  text %q stats %+v", text.String(), stats)
	if strings.TrimSpace(text.String()) == "" {
		t.Error("empty answer")
	}
	if strings.Contains(text.String(), "<think>") || strings.Contains(text.String(), "</think>") {
		t.Errorf("answer contains thinking text: %q", text.String())
	}
	if stats.FinishReason != "stop" && stats.FinishReason != "length" {
		t.Errorf("finish reason %q", stats.FinishReason)
	}
	if stats.CompletionTokens <= 0 {
		t.Errorf("completion tokens %d", stats.CompletionTokens)
	}

	// Phase 6: over-long prompt.
	t.Log("phase 6: over-long prompt")
	long := ChatRequest{
		Messages: []Message{{Role: "user", Content: strings.Repeat("word ", 3000)}},
		Sampling: Sampling{Temperature: 0.7, TopP: 0.8, TopK: 20, MaxTokens: 16},
	}
	streamBody := buildChatBody(long)
	rs := rawPost(t, native, streamBody, 4096)
	t.Logf("  stream:true  status %d content-type %q body:\n%s", rs.status, rs.contentType, strings.Join(rs.lines, "\n"))
	record("overlong_stream.txt", strings.Join(rs.lines, "\n")+"\n")
	plainBody := streamBody
	plainBody.Stream = false
	plainBody.StreamOptions = nil
	rp := rawPost(t, native, plainBody, 4096)
	t.Logf("  stream:false status %d content-type %q body:\n%s", rp.status, rp.contentType, strings.Join(rp.lines, "\n"))
	record("overlong_plain.txt", strings.Join(rp.lines, "\n")+"\n")
	_, err = proc.Chat(context.Background(), long, nil)
	t.Logf("  proc.Chat error: %v", err)
	if !errors.Is(err, ErrContext) {
		t.Errorf("over-long prompt: want ErrContext, got %v", err)
	}

	// Phase 7: cancel at the first delta; the single slot must be free afterwards.
	t.Log("phase 7: cancel")
	ctx, cancel := context.WithCancel(context.Background())
	longAnswer := ChatRequest{
		Messages: []Message{{Role: "user", Content: "Write a very long story about a dragon and a knight."}},
		Sampling: Sampling{Temperature: 0.7, TopP: 0.8, TopK: 20, MaxTokens: 512},
	}
	deltas := 0
	cancelAt := time.Time{}
	_, err = proc.Chat(ctx, longAnswer, func(string) {
		deltas++
		if deltas == 1 {
			cancelAt = time.Now()
			cancel()
		}
	})
	cancel()
	t.Logf("  cancelled chat error: %v (deltas %d)", err, deltas)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled chat: want context.Canceled, got %v", err)
	}
	followStart := time.Now()
	short := ChatRequest{
		Messages: []Message{{Role: "user", Content: "Say ok."}},
		Sampling: Sampling{Temperature: 0.7, TopP: 0.8, TopK: 20, MaxTokens: 8},
	}
	followCtx, followCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer followCancel()
	_, err = proc.Chat(followCtx, short, nil)
	t.Logf("  follow-up chat after cancel: err=%v after %s (cancel to follow-up start %s)", err, time.Since(followStart).Round(time.Millisecond), followStart.Sub(cancelAt).Round(time.Millisecond))
	if err != nil {
		t.Errorf("follow-up chat after cancel: %v", err)
	}
	t.Logf("  log tail after cancel:\n%s", native.logTail())
	record("log_tail.txt", native.logTail())

	// Phase 8: stop.
	t.Log("phase 8: stop")
	if err := proc.Stop(); err != nil {
		t.Errorf("Stop: %v", err)
	}
	stopped = true
	select {
	case <-proc.Done():
		t.Logf("  pid %d exited", native.handle.pid)
	case <-time.After(5 * time.Second):
		t.Error("Done did not close within 5 s of Stop")
	}
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ""
}

func runServerOnce(t *testing.T, server string, libs []string, args ...string) string {
	t.Helper()
	cmd := newServerCommand(server, libs, "smoke-diagnostic", args)
	timer := time.AfterFunc(30*time.Second, func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	defer timer.Stop()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("llama-server %v: %v", args, err)
	}
	return string(out)
}

type rawAnswer struct {
	status      int
	contentType string
	lines       []string
}

// rawPost sends body to the child with its key and returns the status and the answer lines (all lines, or the
// first limitBytes bytes of the body when limitBytes > 0).
func rawPost(t *testing.T, p *nativeProcess, body chatBody, limitBytes int) rawAnswer {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.key)
	resp, err := p.client.Do(req)
	if err != nil {
		t.Fatalf("raw post: %v", err)
	}
	defer resp.Body.Close()
	answer := rawAnswer{status: resp.StatusCode, contentType: resp.Header.Get("Content-Type")}
	var reader io.Reader = resp.Body
	if limitBytes > 0 {
		reader = io.LimitReader(resp.Body, int64(limitBytes))
	}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for scanner.Scan() {
		answer.lines = append(answer.lines, scanner.Text())
	}
	return answer
}

// checkNoReasoning fails when the raw stream carries a non-empty reasoning_content or a think tag.
func checkNoReasoning(t *testing.T, lines []string) {
	t.Helper()
	for _, line := range lines {
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if !strings.HasPrefix(payload, "{") {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(payload), &chunk) != nil || len(chunk.Choices) == 0 {
			continue
		}
		if chunk.Choices[0].Delta.ReasoningContent != "" {
			t.Errorf("stream carries reasoning_content: %q", chunk.Choices[0].Delta.ReasoningContent)
		}
		if strings.Contains(chunk.Choices[0].Delta.Content, "<think>") {
			t.Errorf("stream content carries a think tag: %q", chunk.Choices[0].Delta.Content)
		}
	}
}

// smokeThreads is the instance thread count (instanceThreads), or CHIBI_SMOKE_THREADS (a busy machine needs fewer threads).
func smokeThreads() int {
	if v, err := strconv.Atoi(os.Getenv("CHIBI_SMOKE_THREADS")); err == nil && v > 0 {
		return v
	}
	return instanceThreads(runtime.NumCPU())
}

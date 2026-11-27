// Package chatrttest is an in-memory chatrt.Launcher for tests of the server and the browser smoke: no process, no
// network, no GPU. Replies stream word by word.
package chatrttest

import (
	"context"
	"strings"
	"sync"

	"aurago/internal/chibisensei/chatrt"
	"aurago/internal/llamacpp"
	"aurago/internal/onnxrt"
)

// Launcher is a scripted launcher. Exported fields are read under mu; set them before the first request or
// through the methods.
type Launcher struct {
	mu         sync.Mutex
	installed  bool
	Replies    map[string]string // by key prefix ("tuned:", "base:"); default "Fake answer."
	FailLaunch map[string]error  // by key prefix
	ChatErr    map[string]error  // by key prefix, returned after the reply was streamed
	Hold       chan struct{}     // non-nil: Chat waits for it or ctx before answering

	launches  []chatrt.LaunchSpec
	requests  map[string][]chatrt.ChatRequest
	stopped   int
	cancelled chan string
}

// New returns a launcher whose runtime is installed or not.
func New(installed bool) *Launcher {
	return &Launcher{
		installed: installed,
		Replies:   map[string]string{}, FailLaunch: map[string]error{}, ChatErr: map[string]error{},
		requests: map[string][]chatrt.ChatRequest{}, cancelled: make(chan string, 8),
	}
}

// SetInstalled changes what Verify reports.
func (l *Launcher) SetInstalled(v bool) {
	l.mu.Lock()
	l.installed = v
	l.mu.Unlock()
}

// Launches lists the specs of every Launch call.
func (l *Launcher) Launches() []chatrt.LaunchSpec {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]chatrt.LaunchSpec(nil), l.launches...)
}

// Requests lists the chat requests sent to instances whose key starts with prefix.
func (l *Launcher) Requests(prefix string) []chatrt.ChatRequest {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]chatrt.ChatRequest(nil), l.requests[prefix]...)
}

// Stopped counts the Stop calls that ended a running fake process.
func (l *Launcher) Stopped() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.stopped
}

// Cancelled yields the key of every Chat that saw its context end (buffer 8).
func (l *Launcher) Cancelled() <-chan string { return l.cancelled }

func (l *Launcher) Platform() (string, string) { return "linux", "amd64" }

func (l *Launcher) Hardware(context.Context) llamacpp.HardwareInfo { return llamacpp.HardwareInfo{} }

func (l *Launcher) Assets(string) ([]llamacpp.AssetSpec, bool) {
	return []llamacpp.AssetSpec{{
		ID: "fake-cpu", URL: "https://github.com/ggml-org/llama.cpp/releases/download/b9994/fake.tar.gz",
		Size: 15_857_307, SHA256: strings.Repeat("ab", 32), Kind: onnxrt.AssetTarGzip,
	}}, true
}

func (l *Launcher) Verify(context.Context, string, []llamacpp.AssetSpec) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.installed
}

func (l *Launcher) Install(_ context.Context, _ string, assets []llamacpp.AssetSpec, progress func(string, int64)) error {
	for _, a := range assets {
		if progress != nil {
			progress(a.ID, a.Size)
		}
	}
	l.SetInstalled(true)
	return nil
}

func (l *Launcher) Launch(_ context.Context, spec chatrt.LaunchSpec) (chatrt.Process, error) {
	l.mu.Lock()
	l.launches = append(l.launches, spec)
	err := byPrefix(l.FailLaunch, spec.Key)
	l.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return &process{l: l, key: spec.Key, done: make(chan struct{})}, nil
}

func byPrefix[V any](m map[string]V, key string) V {
	var zero V
	for prefix, v := range m {
		if strings.HasPrefix(key, prefix) {
			return v
		}
	}
	return zero
}

func prefixOf(key string) string {
	if i := strings.Index(key, ":"); i >= 0 {
		return key[:i+1]
	}
	return key
}

type process struct {
	l    *Launcher
	key  string
	once sync.Once
	done chan struct{}
}

func (p *process) GPUVerified() bool     { return false }
func (p *process) Done() <-chan struct{} { return p.done }

func (p *process) Stop() error {
	p.once.Do(func() {
		close(p.done)
		p.l.mu.Lock()
		p.l.stopped++
		p.l.mu.Unlock()
	})
	return nil
}

func (p *process) Chat(ctx context.Context, req chatrt.ChatRequest, onDelta func(string)) (chatrt.ChatStats, error) {
	p.l.mu.Lock()
	prefix := prefixOf(p.key)
	p.l.requests[prefix] = append(p.l.requests[prefix], req)
	reply, ok := "", false
	for pre, r := range p.l.Replies {
		if strings.HasPrefix(p.key, pre) {
			reply, ok = r, true
		}
	}
	if !ok {
		reply = "Fake answer."
	}
	chatErr := byPrefix(p.l.ChatErr, p.key)
	hold := p.l.Hold
	p.l.mu.Unlock()

	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
			p.noteCancelled()
			return chatrt.ChatStats{}, ctx.Err()
		case <-p.done:
			return chatrt.ChatStats{}, chatrt.ErrStopped
		}
	}
	words := strings.SplitAfter(reply, " ")
	count := 0
	for _, w := range words {
		if w == "" {
			continue
		}
		select {
		case <-ctx.Done():
			p.noteCancelled()
			return chatrt.ChatStats{}, ctx.Err()
		case <-p.done:
			return chatrt.ChatStats{}, chatrt.ErrStopped
		default:
		}
		count++
		if onDelta != nil {
			onDelta(w)
		}
	}
	if chatErr != nil {
		return chatrt.ChatStats{}, chatErr
	}
	return chatrt.ChatStats{FinishReason: "stop", PromptTokens: 10 * len(req.Messages), CompletionTokens: count, MS: 5, TokensPerSecond: 20}, nil
}

func (p *process) noteCancelled() {
	select {
	case p.l.cancelled <- p.key:
	default:
	}
}

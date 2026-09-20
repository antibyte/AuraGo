package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const backgroundCompletionLimit = 256

type backgroundCompletion struct {
	digest   [32]byte
	done     chan struct{}
	response *backgroundResponse
	finished time.Time
}

type backgroundCompletionCache struct {
	mu      sync.Mutex
	entries map[string]*backgroundCompletion
}

// Internal background retries poll the same execution, including after the
// original HTTP client disconnects. Entries live longer than the retry window;
// completed entries are not evicted early merely to admit more work.
func withBackgroundCompletionReplay(s *Server, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Background-Execution-ID"))
		if id == "" {
			next(w, r)
			return
		}
		followUp, missionID, valid := validateInternalChatHeaders(r, s)
		if !valid || !followUp || missionID != "" {
			writeInvalidInternalChatHeaders(w)
			return
		}
		if r.Method != http.MethodPost || len(id) > 160 || !internalMissionIDPattern.MatchString(id) {
			http.Error(w, "invalid background execution", http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			http.Error(w, "invalid background request body", http.StatusBadRequest)
			return
		}
		var options struct {
			Stream bool `json:"stream"`
		}
		if json.Unmarshal(body, &options) != nil || options.Stream {
			http.Error(w, "background execution requires non-streaming JSON", http.StatusBadRequest)
			return
		}
		digest := sha256.Sum256(append([]byte(r.Header.Get("X-Session-ID")+"\x00"), body...))
		cache := &s.backgroundCompletions
		cache.mu.Lock()
		if cache.entries == nil {
			cache.entries = make(map[string]*backgroundCompletion)
		}
		for key, entry := range cache.entries {
			if !entry.finished.IsZero() && time.Since(entry.finished) > 24*time.Hour {
				delete(cache.entries, key)
			}
		}
		entry := cache.entries[id]
		if entry != nil && entry.digest != digest {
			cache.mu.Unlock()
			http.Error(w, "background execution payload changed", http.StatusConflict)
			return
		}
		if entry == nil {
			if len(cache.entries) >= backgroundCompletionLimit {
				cache.mu.Unlock()
				http.Error(w, "background execution capacity reached", http.StatusServiceUnavailable)
				return
			}
			entry = &backgroundCompletion{digest: digest, done: make(chan struct{})}
			cache.entries[id] = entry
			request := r.Clone(context.WithoutCancel(r.Context()))
			request.Body = io.NopCloser(bytes.NewReader(body))
			go func() {
				response := &backgroundResponse{header: make(http.Header)}
				defer func() {
					if recover() != nil {
						response.fail("background execution failed")
					}
					cache.mu.Lock()
					entry.response, entry.finished = response, time.Now()
					close(entry.done)
					cache.mu.Unlock()
				}()
				next(response, request)
			}()
		}
		cache.mu.Unlock()
		select {
		case <-entry.done:
			entry.response.replay(w)
		case <-r.Context().Done():
			return
		default:
			w.Header().Set("Retry-After", "10")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = io.WriteString(w, `{"status":"running"}`)
		}
	}
}

// A background caller consumes only completion status. Bound the retained
// response so an unusually large completion cannot exhaust the replay cache.
type backgroundResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
	failed bool
}

func (w *backgroundResponse) Header() http.Header { return w.header }
func (w *backgroundResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *backgroundResponse) Write(p []byte) (int, error) {
	if w.failed {
		return 0, fmt.Errorf("background response limit exceeded")
	}
	if w.body.Len()+len(p) > 128*1024 {
		w.fail("background response limit exceeded")
		return 0, fmt.Errorf("background response limit exceeded")
	}
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(p)
}
func (w *backgroundResponse) fail(message string) {
	w.status, w.failed = http.StatusInternalServerError, true
	w.header = make(http.Header)
	w.header.Set("Content-Type", "application/json")
	w.body.Reset()
	_ = json.NewEncoder(&w.body).Encode(map[string]string{"error": message})
}
func (w *backgroundResponse) replay(dst http.ResponseWriter) {
	for key, values := range w.header {
		dst.Header()[key] = append([]string(nil), values...)
	}
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}
	dst.WriteHeader(status)
	_, _ = dst.Write(w.body.Bytes())
}

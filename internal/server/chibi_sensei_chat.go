package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"time"

	"aurago/internal/chibisensei"
	"aurago/internal/chibisensei/chatrt"
	"aurago/internal/security"
)

// chibiChatEvent is one event of the chat stream.
type chibiChatEvent struct {
	name string
	data any
}

// chibiChatTarget is one model of a request: its file, its prepared conversation and the sampling to use.
type chibiChatTarget struct {
	kind     string
	ref      chatrt.ModelRef
	messages []chatrt.Message
	req      chatrt.ChatRequest
}

// chibiChatMaxTokens is the profile's reply length for the run's recipe: the specialist value for a specialist
// project, the assistant value otherwise (also when job.json cannot be read).
func chibiChatMaxTokens(p *chibisensei.Profile, run chibisensei.Run) int {
	if recipe, _, _, err := chibisensei.RunTask(run); err == nil && recipe == chibisensei.RecipeSpecialist {
		return p.Eval.MaxNewTokensSpecialist
	}
	return p.Eval.MaxNewTokensAssistant
}

// chibiChatSampling merges the request overrides into the profile's values; the reply length is capped.
func chibiChatSampling(p *chibisensei.Profile, run chibisensei.Run, in *chibiChatSamplingIn) chatrt.Sampling {
	out := chibiProfileSampling(p, chibiChatMaxTokens(p, run))
	if in != nil {
		if in.Temperature != nil {
			out.Temperature = *in.Temperature
		}
		if in.TopP != nil {
			out.TopP = *in.TopP
		}
		if in.TopK != nil {
			out.TopK = *in.TopK
		}
		if in.MaxTokens != nil {
			out.MaxTokens = *in.MaxTokens
		}
		if in.Seed != nil {
			out.Seed = *in.Seed
		}
	}
	out.MaxTokens = min(max(out.MaxTokens, 1), chibiChatMaxTokensCap)
	return out
}

// handleChibiChat answers POST /runtime/chat. Everything that can fail before the first byte answers as an ordinary
// JSON error; afterwards the answer is a server-sent event stream with one done or error event per target and a
// final end event.
func handleChibiChat(s *Server, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeChibiMethodError(w, http.MethodPost)
		return
	}
	// A write-class request: a desktop that turns read-only cancels the request context and with it both answers.
	acc, ok := chibiSenseiAuthorize(s, w, r, desktopScopeWrite, desktopWrite, false)
	if !ok || !chibiSameOrigin(w, r) {
		return
	}
	if _, ok := w.(http.Flusher); !ok {
		writeChibiError(w, http.StatusInternalServerError, "streaming_unsupported", "Streaming is not supported.", nil)
		return
	}
	var req chibiChatRequest
	if !chibiDecode(w, r, &req, chibiJSONLimit) {
		return
	}
	if verr := validateChibiChat(&req); verr != nil {
		chibiDomainError(w, verr)
		return
	}
	run, ok := chibiLoadRun(w, r, acc, req.RunID)
	if !ok {
		return
	}
	if run.State != chibisensei.RunSucceeded {
		chibiDomainError(w, chibisensei.ErrRunNotSucceeded)
		return
	}
	prof, ok := acc.rt.catalog.Get(run.ProfileID)
	if !ok {
		chibiProfileUnknown(w)
		return
	}
	tuned, base, err := chibisensei.RunModels(r.Context(), acc.rt.store, acc.rt.dataDir, run)
	if err != nil {
		chibiDomainError(w, err)
		return
	}
	sampling := chibiChatSampling(prof, run, req.Sampling)
	var targets []chibiChatTarget
	var keys []string
	baseKey := ""
	for _, t := range req.Targets {
		mf := tuned
		if t.Kind == "base" {
			mf = base
		}
		if !mf.Available {
			writeChibiError(w, http.StatusNotFound, "model_missing", "The "+t.Kind+" model file is not available.", nil)
			return
		}
		if t.Kind == "base" {
			baseKey = mf.Key
		}
		msgs := make([]chatrt.Message, len(t.Messages))
		for i, m := range t.Messages {
			msgs[i] = chatrt.Message{Role: m.Role, Content: m.Content}
		}
		targets = append(targets, chibiChatTarget{kind: t.Kind,
			ref: chatrt.ModelRef{Key: mf.Key, Path: mf.Path, Bytes: mf.Bytes},
			req: chatrt.ChatRequest{Messages: chatrt.PrepareMessages(prof.Chat.SystemRole, msgs), Sampling: sampling, Stop: prof.Chat.Stop}})
		keys = append(keys, mf.Key)
	}
	if err := acc.rt.chat.Admit(keys...); err != nil {
		chibiRuntimeError(w, err)
		return
	}
	if baseKey != "" {
		acc.rt.chatMu.Lock()
		acc.rt.chatRuns[baseKey] = run.ID
		acc.rt.chatMu.Unlock()
	}
	streamChibiChat(s, w, r, acc.rt, targets)
}

// streamChibiChat runs the targets at the same time and writes their events through one writer.
func streamChibiChat(s *Server, w http.ResponseWriter, r *http.Request, rt *chibiSenseiRuntime, targets []chibiChatTarget) {
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	arm := func() { _ = rc.SetWriteDeadline(time.Now().Add(chibiEventWriteTimeout)) }
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")

	genCtx, cancelGen := context.WithCancel(r.Context()) // client close or a read-only switch end both generations
	defer cancelGen()
	events := make(chan chibiChatEvent, 64)
	quit := make(chan struct{}) // closed when the writer returns; emit never blocks after that
	emit := func(e chibiChatEvent) {
		select {
		case events <- e:
		case <-quit:
		}
	}
	var wg sync.WaitGroup
	for _, t := range targets {
		wg.Add(1)
		go func(t chibiChatTarget) {
			defer wg.Done()
			runChibiChatTarget(genCtx, rt, t, emit)
		}(t)
	}
	go func() { wg.Wait(); close(events) }()
	defer func() {
		close(quit)
		cancelGen()
		chibiWaitTimeout(&wg, 10*time.Second)
	}()

	write := func(e chibiChatEvent) bool {
		raw, err := json.Marshal(e.data)
		if err != nil {
			return true
		}
		_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", e.name, raw)
		return err == nil
	}
	arm()
	if _, err := io.WriteString(w, ": chibi-sensei chat\n\n"); err != nil || rc.Flush() != nil {
		return
	}
	retiring := rt.retiring
	var serverDone <-chan struct{}
	if s.integrationCtx != nil {
		serverDone = s.integrationCtx.Done()
	}
	heartbeat := time.NewTicker(chibiEventHeartbeat)
	defer heartbeat.Stop()
	for {
		select {
		case ev, open := <-events:
			arm()
			if !open {
				if write(chibiChatEvent{"end", struct{}{}}) {
					_ = rc.Flush()
				}
				return
			}
			if !write(ev) {
				return
			}
			for i := 0; i < 31; i++ { // batch what is already queued into one flush
				var more chibiChatEvent
				var got, stillOpen bool
				select {
				case more, stillOpen = <-events:
					got = true
				default:
				}
				if !got {
					break
				}
				if !stillOpen {
					if write(chibiChatEvent{"end", struct{}{}}) {
						_ = rc.Flush()
					}
					return
				}
				if !write(more) {
					return
				}
			}
			if rc.Flush() != nil {
				return
			}
		case <-heartbeat.C:
			arm()
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil || rc.Flush() != nil {
				return
			}
		case <-retiring:
			cancelGen() // the targets still send their error events and the stream its end
			retiring = nil
		case <-serverDone:
			cancelGen()
			serverDone = nil
		}
	}
}

// chibiWaitTimeout waits for wg for at most d.
func chibiWaitTimeout(wg *sync.WaitGroup, d time.Duration) {
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
	}
}

// Fixed English texts of the stream errors; upstream error text never reaches the client.
var chibiChatErrorText = map[string]string{
	"start_failed":      "The model could not be started.",
	"generation_failed": "The model failed while answering.",
	"context_exceeded":  "The conversation is longer than the model can read.",
	"runtime_busy":      "The local chat runtime is busy with other models. Try again in a moment.",
	"cancelled":         "The answer was cancelled.",
}

// runChibiChatTarget runs one target and always ends with exactly one done or error event.
func runChibiChatTarget(ctx context.Context, rt *chibiSenseiRuntime, t chibiChatTarget, emit func(chibiChatEvent)) {
	fail := func(code string, cause error) {
		if code != "cancelled" && cause != nil {
			slog.Warn("[ChibiSensei] Chat target failed", "target", t.kind, "code", code, "error", security.Scrub(cause.Error()))
		}
		emit(chibiChatEvent{"error", map[string]any{"target": t.kind, "code": code, "message": chibiChatErrorText[code]}})
	}
	cancelled := func(err error) bool {
		return ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(err, chatrt.ErrStopped) || errors.Is(err, chatrt.ErrClosed)
	}
	emit(chibiChatEvent{"status", map[string]any{"target": t.kind, "state": "loading"}})
	inst, err := rt.chat.Acquire(ctx, t.ref)
	switch {
	case err == nil:
	case errors.Is(err, chatrt.ErrBusy), errors.Is(err, chatrt.ErrInstalling):
		fail("runtime_busy", err)
		return
	case cancelled(err):
		fail("cancelled", err)
		return
	default:
		fail("start_failed", err)
		return
	}
	defer inst.Release()
	emit(chibiChatEvent{"status", map[string]any{"target": t.kind, "state": "generating"}})
	stats, err := inst.Chat(ctx, t.req, func(text string) {
		emit(chibiChatEvent{"delta", map[string]any{"target": t.kind, "text": text}})
	})
	switch {
	case err == nil:
	case errors.Is(err, chatrt.ErrContext):
		fail("context_exceeded", err)
		return
	case cancelled(err):
		fail("cancelled", err)
		return
	default:
		fail("generation_failed", err)
		return
	}
	emit(chibiChatEvent{"done", map[string]any{"target": t.kind, "finish_reason": stats.FinishReason,
		"prompt_tokens": stats.PromptTokens, "completion_tokens": stats.CompletionTokens, "ms": stats.MS,
		"tok_s": math.Round(stats.TokensPerSecond*10) / 10, "backend": stats.Backend}})
}

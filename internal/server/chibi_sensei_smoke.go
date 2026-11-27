package server

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"aurago/internal/chibisensei"
	"aurago/internal/chibisensei/chatrt"
	"aurago/internal/security"
)

const (
	chibiSmokePrompt   = "Hello! Please introduce yourself in one sentence."
	chibiSmokeTokens   = 16
	chibiSmokeMaxBytes = 2000
)

// chibiSmoker adapts the chat runtime to the orchestrator's Smoker seam: one short generation with the tuned
// model after a run's results are checked. It never downloads anything and has no read-only gate (it starts a
// local process and changes no user data).
type chibiSmoker struct {
	chat *chatrt.Manager
	log  *slog.Logger
}

// chibiSmokeSkip maps a runtime refusal to the skipped result code.
func chibiSmokeSkip(err error) (string, bool) {
	switch {
	case errors.Is(err, chatrt.ErrUnsupported), errors.Is(err, chatrt.ErrNotInstalled):
		return "runtime_not_installed", true
	case errors.Is(err, chatrt.ErrDisabled), errors.Is(err, chatrt.ErrClosed):
		return "runtime_disabled", true
	case errors.Is(err, chatrt.ErrBusy), errors.Is(err, chatrt.ErrInstalling):
		return "runtime_busy", true
	}
	return "", false
}

func (k chibiSmoker) warn(msg string, err error) {
	log := k.log
	if log == nil {
		log = slog.Default()
	}
	log.Warn(msg, "error", security.Scrub(err.Error()))
}

// Smoke implements chibisensei.Smoker.
func (k chibiSmoker) Smoke(ctx context.Context, in chibisensei.SmokeInput) chibisensei.SmokeResult {
	started := time.Now()
	if err := k.chat.Admit(in.ModelKey); err != nil {
		if code, ok := chibiSmokeSkip(err); ok {
			return chibisensei.SmokeResult{State: "skipped", Code: code}
		}
		k.warn("[ChibiSensei] Smoke admission failed", err)
		return chibisensei.SmokeResult{State: "failed", Code: "start_failed"}
	}
	inst, err := k.chat.Acquire(ctx, chatrt.ModelRef{Key: in.ModelKey, Path: in.ModelPath, Bytes: in.ModelBytes})
	if err != nil {
		if code, ok := chibiSmokeSkip(err); ok {
			return chibisensei.SmokeResult{State: "skipped", Code: code}
		}
		k.warn("[ChibiSensei] Smoke model start failed", err)
		return chibisensei.SmokeResult{State: "failed", Code: "start_failed"}
	}
	defer inst.Release()
	systemRole := ""
	var stop []string
	if in.Profile != nil {
		systemRole, stop = in.Profile.Chat.SystemRole, in.Profile.Chat.Stop
	}
	var text strings.Builder
	_, err = inst.Chat(ctx, chatrt.ChatRequest{
		Messages: chatrt.PrepareMessages(systemRole, []chatrt.Message{{Role: "user", Content: chibiSmokePrompt}}),
		Sampling: chibiProfileSampling(in.Profile, chibiSmokeTokens),
		Stop:     stop,
	}, func(t string) {
		if room := chibiSmokeMaxBytes - text.Len(); room > 0 {
			text.WriteString(t[:min(len(t), room)])
		}
	})
	if err != nil {
		k.warn("[ChibiSensei] Smoke generation failed", err)
		return chibisensei.SmokeResult{State: "failed", Code: "generation_failed"}
	}
	return chibisensei.SmokeResult{State: "ok", MS: max(1, time.Since(started).Milliseconds()), Text: strings.ToValidUTF8(text.String(), "")}
}

// chibiProfileSampling copies the five sampling values of a profile into a request.
func chibiProfileSampling(p *chibisensei.Profile, maxTokens int) chatrt.Sampling {
	out := chatrt.Sampling{MaxTokens: maxTokens}
	if p == nil {
		return out
	}
	out.Temperature, out.TopP, out.TopK = p.Sampling.Temperature, p.Sampling.TopP, p.Sampling.TopK
	out.RepeatPenalty, out.PresencePenalty = p.Sampling.RepeatPenalty, p.Sampling.PresencePenalty
	return out
}

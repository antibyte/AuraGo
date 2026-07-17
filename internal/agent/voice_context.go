package agent

import "context"

type voiceOutputSuppressedContextKey struct{}

// WithVoiceOutputSuppressed marks one request as already owning its spoken
// output. The marker is immutable, request-local, and safe for concurrent chat
// sessions; it never changes global speaker mode or RunConfig.
func WithVoiceOutputSuppressed(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, voiceOutputSuppressedContextKey{}, true)
}

// VoiceOutputSuppressed reports whether the current request must not expose or
// auto-run AuraGo's regular TTS path.
func VoiceOutputSuppressed(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	suppressed, _ := ctx.Value(voiceOutputSuppressedContextKey{}).(bool)
	return suppressed
}

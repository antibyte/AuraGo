package prompts

import "testing"

func TestCountTokens_EmptyString(t *testing.T) {
	if got := CountTokens(""); got != 0 {
		t.Fatalf("CountTokens(\"\") = %d, want 0", got)
	}
}

func TestCountTokens_DeterministicPositiveEstimate(t *testing.T) {
	text := "hello\nworld\nthis is a prompt"
	first := CountTokens(text)
	second := CountTokens(text)
	if first <= 0 {
		t.Fatalf("CountTokens returned %d, want > 0", first)
	}
	if first != second {
		t.Fatalf("CountTokens not deterministic: first=%d second=%d", first, second)
	}
}

func TestCountTokensForModel_AppliesMultiplier(t *testing.T) {
	text := "this is a medium length prompt used for testing token multipliers"
	base := CountTokens(text)
	adjusted := CountTokensForModel(text, "claude-3-7-sonnet")
	if adjusted <= base {
		t.Fatalf("CountTokensForModel() = %d, want > base %d for claude", adjusted, base)
	}
	if got := CountTokensForModel(text, "MiniMax-M2.7-highspeed"); got != base {
		t.Fatalf("CountTokensForModel() = %d, want base %d for default multiplier", got, base)
	}
}
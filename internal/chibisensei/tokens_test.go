package chibisensei

import (
	"strings"
	"testing"
)

func TestEstimateTokens(t *testing.T) {
	if got := EstimateTokens([]Message{{"user", "hello"}, {"assistant", "hi"}}, ""); got != 10 {
		t.Errorf("chat = %d, want 10", got)
	}
	if got := EstimateTokens(nil, "日本語です"); got != 5 {
		t.Errorf("cjk text = %d, want 5", got)
	}
	if got := EstimateTokens([]Message{{"user", "abc漢字"}}, ""); got != 7 {
		t.Errorf("mixed = %d, want 7", got)
	}
	if got := EstimateTokens(nil, ""); got != 0 {
		t.Errorf("empty = %d", got)
	}
	if got := EstimateTokens(nil, strings.Repeat("a", 36)); got != 10 {
		t.Errorf("36 a = %d, want 10", got)
	}
	if got := EstimateTokens(nil, strings.Repeat("a", 37)); got != 11 {
		t.Errorf("37 a = %d, want 11", got)
	}
	v := make([]int, 100)
	for i := range v {
		v[i] = i + 1
	}
	if got := Percentile(v, 0.95); got != 95 {
		t.Errorf("p95 = %d", got)
	}
	if got := Percentile(nil, 0.5); got != 0 {
		t.Errorf("empty percentile = %d", got)
	}
}

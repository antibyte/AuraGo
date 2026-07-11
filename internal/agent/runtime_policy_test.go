package agent

import "testing"

func TestHistoryCompressionPolicyRunsAtEntryOrAboveThreshold(t *testing.T) {
	const maxHistoryTokens = 1000
	threshold := int(float64(maxHistoryTokens) * compressionThresholdPct)
	tests := []struct {
		name      string
		iteration int
		tokens    int
		want      bool
	}{
		{name: "loop entry below threshold", iteration: 1, tokens: 1, want: true},
		{name: "later below threshold", iteration: 2, tokens: threshold, want: false},
		{name: "later above threshold", iteration: 2, tokens: threshold + 1, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRunHistoryCompression(tt.iteration, tt.tokens, maxHistoryTokens); got != tt.want {
				t.Fatalf("shouldRunHistoryCompression(%d, %d, %d) = %v, want %v", tt.iteration, tt.tokens, maxHistoryTokens, got, tt.want)
			}
		})
	}
}

func TestDefaultAgentLoopConcurrencyIsEight(t *testing.T) {
	if defaultMaxConcurrentAgentLoops != 8 {
		t.Fatalf("defaultMaxConcurrentAgentLoops = %d, want 8", defaultMaxConcurrentAgentLoops)
	}
}

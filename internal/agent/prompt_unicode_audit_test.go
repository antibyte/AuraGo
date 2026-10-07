package agent

import (
	"testing"
	"unicode/utf8"
)

func TestPromptTruncationWrappersPreserveUTF8WithinTinyByteCaps(t *testing.T) {
	for maxBytes := 1; maxBytes <= 5; maxBytes++ {
		coagent := truncatePromptBlock("🌍任务を確認する", maxBytes)
		if len(coagent) > maxBytes || !utf8.ValidString(coagent) {
			t.Fatalf("co-agent truncation max=%d result=%q bytes=%d valid=%v", maxBytes, coagent, len(coagent), utf8.ValidString(coagent))
		}
		activity := truncateActivityDigestInput("🌍任务を確認する", maxBytes)
		if len(activity) > maxBytes || !utf8.ValidString(activity) {
			t.Fatalf("activity truncation max=%d result=%q bytes=%d valid=%v", maxBytes, activity, len(activity), utf8.ValidString(activity))
		}
	}
}

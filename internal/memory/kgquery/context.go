package kgquery

import (
	"fmt"
	"strings"

	"aurago/internal/security"
)

// SensitivePropertyKey reports whether a node property key must not appear in prompt context.
type SensitivePropertyKey func(key string) bool

// AppendContextProperties writes non-sensitive node properties into the builder.
func AppendContextProperties(sb *strings.Builder, properties map[string]string, isSensitive SensitivePropertyKey) {
	for k, v := range properties {
		if k == "access_count" || k == "protected" || k == "source" || k == "extracted_at" {
			continue
		}
		if isSensitive != nil && isSensitive(k) {
			continue
		}
		sb.WriteString(fmt.Sprintf(" | %s: %s", k, v))
	}
}

// FinalizeContextResult truncates and scrubs a knowledge-graph context string.
func FinalizeContextResult(sb strings.Builder, maxChars int) string {
	result := sb.String()
	if len(result) > maxChars {
		result = TruncateUTF8Safe(result, maxChars)
	}
	return security.Scrub(result)
}
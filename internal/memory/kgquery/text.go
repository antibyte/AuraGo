package kgquery

import "strings"

// FitEntries keeps complete entries and makes omission explicit within a byte budget.
func FitEntries(entries []string, separator string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	const omitted = "[additional entries omitted]"
	kept := make([]string, 0, len(entries))
	size := 0
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		added := len(entry)
		if len(kept) > 0 {
			added += len(separator)
		}
		if size+added <= maxBytes {
			kept = append(kept, entry)
			size += added
			continue
		}
		for len(kept) > 0 && size+len(separator)+len(omitted) > maxBytes {
			size -= len(kept[len(kept)-1])
			kept = kept[:len(kept)-1]
			if len(kept) > 0 {
				size -= len(separator)
			}
		}
		if len(omitted) <= maxBytes {
			kept = append(kept, omitted)
		}
		break
	}
	return strings.Join(kept, separator)
}

// TruncateUTF8Safe shortens text to maxLen runes without splitting multibyte characters.
func TruncateUTF8Safe(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	truncated := string(runes[:maxLen])
	if idx := strings.LastIndex(truncated, "\n"); idx > 0 {
		truncated = truncated[:idx]
	}
	return truncated
}

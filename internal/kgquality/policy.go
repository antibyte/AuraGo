package kgquality

import (
	"crypto/sha1"
	"encoding/hex"
	"path"
	"strconv"
	"strings"
	"unicode"
)

const DefaultPendingCoMentionTTLDays = 7
const DefaultLowConfidenceCoMentionMinWeight = 2

type Policy struct {
	PendingCoMentionTTLDays         int
	LowConfidenceCoMentionMinWeight int
	HideLowConfidenceByDefault      bool
}

func DefaultPolicy() Policy {
	return Policy{
		PendingCoMentionTTLDays:         DefaultPendingCoMentionTTLDays,
		LowConfidenceCoMentionMinWeight: DefaultLowConfidenceCoMentionMinWeight,
		HideLowConfidenceByDefault:      true,
	}
}

func NormalizePolicy(policy Policy) Policy {
	defaults := DefaultPolicy()
	if policy.PendingCoMentionTTLDays <= 0 {
		policy.PendingCoMentionTTLDays = defaults.PendingCoMentionTTLDays
	}
	if policy.LowConfidenceCoMentionMinWeight <= 0 {
		policy.LowConfidenceCoMentionMinWeight = defaults.LowConfidenceCoMentionMinWeight
	}
	return policy
}

var genericTokens = map[string]struct{}{
	"png": {}, "jpg": {}, "jpeg": {}, "gif": {}, "webp": {}, "svg": {},
	"pdf": {}, "doc": {}, "docx": {}, "txt": {}, "md": {},
	"rgb": {}, "rgba": {}, "rgba8": {},
	"x86": {}, "x86_64": {}, "amd64": {}, "arm64": {},
	"attachment": {}, "attachments": {}, "attachment_folder": {}, "attachment_folders": {},
	"file": {}, "files": {}, "folder": {}, "folders": {},
	"image": {}, "images": {}, "photo": {}, "photos": {},
	"document": {}, "documents": {}, "unknown": {},
}

func IsGenericEntity(value string) bool {
	token := normalizeEntityToken(value)
	if token == "" || len(token) < 2 {
		return true
	}
	_, ok := genericTokens[token]
	return ok
}

func normalizeEntityToken(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.Trim(value, ".:/\\-_ ")
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastUnderscore = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '.':
			if b.Len() > 0 && !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

func IsPathLike(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "://") {
		return false
	}
	if len(value) >= 3 && ((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z')) && value[1] == ':' && (value[2] == '\\' || value[2] == '/') {
		return true
	}
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, `\`) || strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") {
		return true
	}
	if strings.Contains(value, `\`) {
		return true
	}
	if strings.Contains(value, "/") {
		return path.Ext(CanonicalPath(value)) != ""
	}
	return false
}

func CanonicalPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.ReplaceAll(value, `\`, "/")
	return path.Clean(value)
}

func FileNodeID(pathValue string) string {
	clean := CanonicalPath(pathValue)
	sum := sha1.Sum([]byte(clean))
	return "file_" + hex.EncodeToString(sum[:])[:12]
}

func PathBase(pathValue string) string {
	clean := CanonicalPath(pathValue)
	if clean == "" {
		return ""
	}
	return path.Base(clean)
}

func LowConfidenceCoMention(relation string, properties map[string]string, policy Policy) bool {
	if strings.TrimSpace(relation) != "co_mentioned_with" {
		return false
	}
	policy = NormalizePolicy(policy)
	source := strings.TrimSpace(properties["source"])
	if source == "manual" {
		return false
	}
	if source == "pending" || source == "" {
		return true
	}
	weight, _ := strconv.Atoi(strings.TrimSpace(properties["weight"]))
	return weight < policy.LowConfidenceCoMentionMinWeight
}

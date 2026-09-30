package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

const MemoryAnalysisSourceMarker = "source:memory_analysis"

// MemoryAnalysisSession recognizes only the complete, known provenance body.
// Arbitrary source-looking text remains factual content.
func MemoryAnalysisSession(content string) (string, bool) {
	content = normalizeMemoryDocumentPart(content)
	if content == MemoryAnalysisSourceMarker {
		return "", true
	}
	const prefix = MemoryAnalysisSourceMarker + " session:"
	if !strings.HasPrefix(content, prefix) || strings.Contains(content, "\n") {
		return "", false
	}
	session := strings.TrimSpace(strings.TrimPrefix(content, prefix))
	return session, session != ""
}

// AnalysisMemoryIdentity retains the entire factual concept (including kind
// and category) and domain; only recognized session provenance is excluded.
func AnalysisMemoryIdentity(concept, content, domain string) (string, bool) {
	concept = normalizeMemoryDocumentPart(concept)
	end := strings.Index(concept, "]")
	if !strings.HasPrefix(concept, "[") || end <= 1 || strings.TrimSpace(concept[end+1:]) == "" {
		return "", false
	}
	if _, ok := MemoryAnalysisSession(content); !ok {
		return "", false
	}
	encoded, _ := json.Marshal([]string{concept, normalizeMemoryDocumentPart(domain)})
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), true
}

// AnalysisDocumentParts unwraps a proven analysis envelope, including a
// multiline factual concept, without treating an arbitrary blank line as proof.
func AnalysisDocumentParts(document string) (concept, content string, ok bool) {
	document = normalizeMemoryDocumentPart(document)
	boundary := strings.LastIndex(document, "\n\n"+MemoryAnalysisSourceMarker)
	if boundary < 0 {
		return "", "", false
	}
	concept, content = document[:boundary], document[boundary+2:]
	_, ok = AnalysisMemoryIdentity(concept, content, "")
	return concept, content, ok
}

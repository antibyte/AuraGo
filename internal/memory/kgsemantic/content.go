package kgsemantic

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"context"
)

// NodeContent carries the fields needed to build semantic node documents.
type NodeContent struct {
	ID         string
	Label      string
	Properties map[string]string
}

// EdgeContent carries the fields needed to build semantic edge documents.
type EdgeContent struct {
	Source     string
	Target     string
	Relation   string
	Properties map[string]string
}

// BuildNodeContent renders a node into embedding-friendly text.
func BuildNodeContent(node NodeContent) string {
	var parts []string
	if strings.TrimSpace(node.Label) != "" {
		parts = append(parts, node.Label)
	}

	keys := make([]string, 0, len(node.Properties))
	for key := range node.Properties {
		switch key {
		case "source", "extracted_at", "last_seen", "session_id", "date", "channel", "protected":
			continue
		default:
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := strings.TrimSpace(node.Properties[key])
		if value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", key, value))
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// BuildEdgeContent renders an edge into embedding-friendly text.
func BuildEdgeContent(edge EdgeContent) string {
	var parts []string
	if strings.TrimSpace(edge.Relation) != "" {
		parts = append(parts, edge.Relation)
	}
	srcLabel := strings.TrimSpace(edge.Source)
	tgtLabel := strings.TrimSpace(edge.Target)
	if srcLabel != "" && tgtLabel != "" {
		parts = append(parts, srcLabel+" "+edge.Relation+" "+tgtLabel)
	}
	keys := make([]string, 0, len(edge.Properties))
	for key := range edge.Properties {
		switch key {
		case "source", "extracted_at", "last_seen", "session_id", "date", "channel", "protected":
			continue
		default:
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := strings.TrimSpace(edge.Properties[key])
		if value != "" {
			parts = append(parts, key+": "+value)
		}
	}
	return strings.TrimSpace(strings.Join(parts, ". "))
}

// ShouldIndexNode reports whether a node should be indexed semantically.
func ShouldIndexNode(node NodeContent) bool {
	if strings.TrimSpace(node.ID) == "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(node.Label), "Unknown") {
		return false
	}
	return BuildNodeContent(node) != ""
}

// ShouldSkipQuery reports whether semantic search should be skipped for a query.
func ShouldSkipQuery(query string) bool {
	query = strings.TrimSpace(query)
	if query == "" || query == "*" {
		return true
	}
	runeLen := len([]rune(query))
	if runeLen >= 8 {
		return false
	}
	if runeLen < 2 {
		return true
	}
	if runeLen >= 2 && LooksLikeCompactEntityQuery(query) {
		return false
	}
	if runeLen >= 3 && LooksLikeSlug(query) {
		return false
	}
	return true
}

// LooksLikeSlug reports whether a query resembles a knowledge-graph node ID slug.
func LooksLikeSlug(query string) bool {
	for _, r := range query {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

// LooksLikeCompactEntityQuery reports whether a short query looks entity-like.
func LooksLikeCompactEntityQuery(query string) bool {
	hasUpperOrDigit := false
	for _, r := range query {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpperOrDigit = true
		case r >= '0' && r <= '9':
			hasUpperOrDigit = true
		case r >= 'a' && r <= 'z':
		case r == '-' || r == '_' || r == '.':
		default:
			return false
		}
	}
	return hasUpperOrDigit
}

// ShouldRetryEmbeddingErr reports whether an embedding provider error is transient.
func ShouldRetryEmbeddingErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "rate limit") || strings.Contains(msg, "too many requests") {
		return true
	}
	if strings.Contains(msg, " 429 ") || strings.Contains(msg, "429") {
		return true
	}
	if strings.Contains(msg, " 5") && strings.Contains(msg, "http") {
		return true
	}
	return false
}
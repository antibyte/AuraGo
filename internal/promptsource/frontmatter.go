// Package promptsource validates source framing shared by prompt loading and indexing.
package promptsource

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Split accepts plain Markdown, but never treats malformed frontmatter as body.
func Split(raw string) (frontmatter, body string, present bool, err error) {
	normalized := strings.TrimLeft(strings.ReplaceAll(strings.TrimPrefix(raw, "\xef\xbb\xbf"), "\r\n", "\n"), "\r\n ")
	if !strings.HasPrefix(normalized, "---") {
		return "", raw, false, nil
	}
	if !strings.HasPrefix(normalized, "---\n") {
		return "", "", true, fmt.Errorf("invalid frontmatter opening delimiter")
	}
	inner := normalized[4:]
	if inner == "---" || strings.HasPrefix(inner, "---\n") {
		return "", strings.TrimSpace(strings.TrimPrefix(inner, "---")), true, nil
	}
	// A closing delimiter at EOF is valid for an empty body.
	if strings.HasSuffix(inner, "\n---") {
		inner += "\n"
	}
	idx := strings.Index(inner, "\n---\n")
	if idx < 0 {
		return "", "", true, fmt.Errorf("missing frontmatter closing delimiter")
	}
	frontmatter, body = inner[:idx], strings.TrimSpace(inner[idx+5:])
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(frontmatter), &doc); err != nil {
		return "", "", true, err
	}
	if len(doc.Content) > 0 && doc.Content[0].Kind != yaml.MappingNode {
		return "", "", true, fmt.Errorf("frontmatter must be a mapping")
	}
	return frontmatter, body, true, nil
}

package huggingface

import (
	"fmt"
	"regexp"
	"strings"
)

var repoComponent = regexp.MustCompile(`^[A-Za-z0-9_](?:[A-Za-z0-9_.-]{0,94}[A-Za-z0-9_])?$`)

// CanonicalRepoID returns the one identity used for both policy and HTTP paths.
// Reject encoded input instead of decoding it a different number of times at
// the policy, URL builder and remote server boundaries.
func CanonicalRepoID(raw string) (string, error) {
	id := strings.TrimSpace(raw)
	if id != raw {
		return "", fmt.Errorf("Hugging Face repository ID must not contain surrounding whitespace")
	}
	parts := strings.Split(id, "/")
	if len(parts) < 1 || len(parts) > 2 {
		return "", fmt.Errorf("invalid Hugging Face repository ID")
	}
	for _, p := range parts {
		if !repoComponent.MatchString(p) || strings.Contains(p, "..") || strings.Contains(p, "--") || strings.HasSuffix(p, ".git") {
			return "", fmt.Errorf("invalid Hugging Face repository ID")
		}
	}
	return id, nil
}

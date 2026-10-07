package tools

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// Reject root aliases before the provider can normalize a destructive request.
func validateCloudDeletePath(raw string) error {
	p := strings.TrimSpace(raw)
	for i := 0; i < 8; i++ {
		decoded, err := url.PathUnescape(p)
		if err != nil {
			return fmt.Errorf("ambiguous deletion path")
		}
		if decoded == p {
			break
		}
		p = decoded
		if i == 7 {
			return fmt.Errorf("ambiguous deletion path")
		}
	}
	p = strings.ReplaceAll(p, "\\", "/")
	if strings.ContainsAny(p, "\x00?#") {
		return fmt.Errorf("invalid deletion path")
	}
	if path.Clean("/"+p) == "/" {
		return fmt.Errorf("deleting the storage root is forbidden")
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return fmt.Errorf("deletion path traversal is forbidden")
		}
	}
	return nil
}

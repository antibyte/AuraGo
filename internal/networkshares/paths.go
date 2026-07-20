package networkshares

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

func normalizeOptions(options Options) Options {
	options.AllowedRoots = normalizeStrings(options.AllowedRoots, runtime.GOOS == "windows")
	options.SMBAllowedPrincipals = normalizeStrings(options.SMBAllowedPrincipals, true)
	options.NFSAllowedClients = normalizeClients(options.NFSAllowedClients)
	return options
}

func normalizeStrings(values []string, caseInsensitive bool) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := value
		if caseInsensitive {
			key = strings.ToLower(key)
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeClients(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if canonical, ok := canonicalClient(value); ok {
			if _, exists := seen[canonical]; exists {
				continue
			}
			seen[canonical] = struct{}{}
			out = append(out, canonical)
		}
	}
	sort.Strings(out)
	return out
}

func canonicalClient(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "*" {
		return "", false
	}
	if ip := net.ParseIP(raw); ip != nil {
		return ip.String(), true
	}
	ip, network, err := net.ParseCIDR(raw)
	if err != nil {
		return "", false
	}
	network.IP = ip.Mask(network.Mask)
	return network.String(), true
}

func rootStatuses(roots []string) []RootStatus {
	statuses := make([]RootStatus, 0, len(roots))
	for _, root := range roots {
		status := RootStatus{Path: root}
		if !filepath.IsAbs(root) {
			status.ReasonCode = "root_not_absolute"
			status.Reason = "The allowed root is not absolute."
		} else if info, err := os.Stat(root); err != nil {
			status.ReasonCode = "root_unavailable"
			status.Reason = "The allowed root does not exist or is not accessible."
		} else if !info.IsDir() {
			status.ReasonCode = "root_not_directory"
			status.Reason = "The allowed root is not a directory."
		} else if _, err := filepath.EvalSymlinks(root); err != nil {
			status.ReasonCode = "root_unresolvable"
			status.Reason = "The allowed root cannot be resolved safely."
		} else {
			status.Available = true
		}
		statuses = append(statuses, status)
	}
	return statuses
}

func canonicalAllowedPath(raw string, roots []string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || !filepath.IsAbs(raw) {
		return "", codedError(ErrorOutsideRoot, "The share path must be an absolute path inside an allowed root.", nil)
	}
	absolute, err := filepath.Abs(raw)
	if err != nil {
		return "", codedError(ErrorOutsideRoot, "The share path cannot be resolved.", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", codedError(ErrorOutsideRoot, "The share path must already exist and be resolvable.", err)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", codedError(ErrorOutsideRoot, "The share path must be an existing directory.", err)
	}
	for _, root := range roots {
		rootResolved, rootErr := filepath.EvalSymlinks(root)
		if rootErr != nil {
			continue
		}
		if pathWithinRoot(resolved, rootResolved) {
			return filepath.Clean(resolved), nil
		}
	}
	return "", codedError(ErrorOutsideRoot, "The share path is outside all configured allowed roots.", nil)
}

func pathWithinRoot(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if runtime.GOOS == "windows" {
		if !strings.EqualFold(filepath.VolumeName(path), filepath.VolumeName(root)) {
			return false
		}
		path = strings.ToLower(path)
		root = strings.ToLower(root)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func isPathInScope(path string, roots []string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	for _, root := range roots {
		rootResolved, rootErr := filepath.EvalSymlinks(root)
		if rootErr == nil && pathWithinRoot(resolved, rootResolved) {
			return true
		}
	}
	return false
}

func validateConfiguredRoots(roots []string) error {
	seen := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		root = strings.TrimSpace(root)
		if root == "" || !filepath.IsAbs(root) {
			return fmt.Errorf("network_shares.allowed_roots entries must be non-empty absolute paths")
		}
		cleaned := filepath.Clean(root)
		key := cleaned
		if runtime.GOOS == "windows" {
			key = strings.ToLower(key)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("network_shares.allowed_roots contains duplicate path %q", root)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// ValidateConfiguredRoots validates persisted roots without requiring them to be mounted.
func ValidateConfiguredRoots(roots []string) error {
	return validateConfiguredRoots(roots)
}

// CanonicalClient validates and canonicalizes one configured NFS client.
func CanonicalClient(raw string) (string, bool) {
	return canonicalClient(raw)
}

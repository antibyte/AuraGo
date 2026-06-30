package tools

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func isValidHomepageURL(u string) bool {
	return isValidHomepageURLInternal(u, false)
}

// isValidHomepageURLAllowPrivate is like isValidHomepageURL but permits private /
// loopback IP addresses. Used for homepage operations (screenshot, lighthouse) that
// intentionally target the local dev server (e.g. 192.168.x.x, localhost:3000).
func isValidHomepageURLAllowPrivate(u string) bool {
	return isValidHomepageURLInternal(u, true)
}

func isValidHomepageURLInternal(u string, allowPrivate bool) bool {
	if u == "" {
		return false
	}
	// Must start with http:// or https://
	lower := strings.ToLower(u)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}
	// Reject shell metacharacters
	for _, c := range u {
		switch c {
		case ';', '|', '&', '`', '$', '(', ')', '{', '}', '<', '>', '\\', '!', '\n', '\r', '"', '\'':
			return false
		}
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return false
	}
	hostname := parsed.Hostname()
	if hostname == "" {
		return false
	}
	// SSRF protection: reject private/loopback IPs unless caller explicitly permits them
	if !allowPrivate && isPrivateHost(hostname) {
		return false
	}
	return true
}

// isPrivateHost checks if a hostname resolves to a private or loopback IP address.
func isPrivateHost(hostname string) bool {
	// Check if it's a direct IP
	if ip := net.ParseIP(hostname); ip != nil {
		return isPrivateIP(ip)
	}
	// Resolve hostname and check all IPs
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return true // fail closed: unresolvable hosts are rejected
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return true
		}
	}
	return false
}

// isPrivateIP returns true for loopback, private, and link-local addresses.
func isPrivateIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

// extractHostFromURL extracts the hostname from a URL string.
func extractHostFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// rewriteLocalhostForContainer rewrites localhost / 127.0.0.1 URLs to
// host.docker.internal so that Chrome running inside the homepage container
// can reach services (e.g. Caddy) listening on the Docker host.
func rewriteLocalhostForContainer(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return u
	}
	host := parsed.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		port := parsed.Port()
		if port != "" {
			parsed.Host = "host.docker.internal:" + port
		} else {
			parsed.Host = "host.docker.internal"
		}
		return parsed.String()
	}
	return u
}

// sanitizeProjectDir validates a project directory name for use in shell commands.
// It rejects path traversal, shell metacharacters, and absolute paths.
func sanitizeProjectDir(projectDir string) error {
	if strings.Contains(projectDir, "..") {
		return fmt.Errorf("path traversal detected in project_dir %q. Use a simple relative homepage workspace path such as 'my-site'", projectDir)
	}
	if strings.HasPrefix(projectDir, "/") || strings.HasPrefix(projectDir, "\\") {
		return fmt.Errorf("absolute paths not allowed for project_dir %q. project_dir must be relative to the homepage workspace, e.g. 'ki-news' instead of '/workspace/ki-news'", projectDir)
	}
	for _, c := range projectDir {
		switch c {
		case ';', '|', '&', '`', '$', '(', ')', '{', '}', '<', '>', '\\', '!', '"', '\'', '\n', '\r', ' ':
			return fmt.Errorf("invalid character %q in project directory %q. Use a simple relative directory name like 'my-site' or 'sites/landing-page'", c, projectDir)
		}
	}
	return nil
}

// NormalizeHomepageProjectIdentity returns the canonical registry identity for a
// homepage project. It accepts only workspace-relative project directories.
func NormalizeHomepageProjectIdentity(projectDir string, allowRoot bool) (string, error) {
	projectDir = strings.TrimSpace(filepath.ToSlash(projectDir))
	if projectDir == "" {
		return "", fmt.Errorf("project_dir is required to register a homepage project")
	}
	if filepath.IsAbs(projectDir) || strings.HasPrefix(projectDir, "/") || strings.HasPrefix(projectDir, "\\") {
		return "", fmt.Errorf("project_dir must be relative to the homepage workspace")
	}
	projectDir = strings.Trim(projectDir, "/")
	if err := sanitizeProjectDir(projectDir); err != nil {
		return "", err
	}
	projectDir = strings.Trim(filepath.ToSlash(filepath.Clean(filepath.FromSlash(projectDir))), "/")
	if projectDir == "" || projectDir == "." {
		if allowRoot {
			return ".", nil
		}
		return "", fmt.Errorf(`project_dir "." is ambiguous for new homepage projects`)
	}
	return projectDir, nil
}

func validateHomepageProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name is required")
	}
	for _, c := range name {
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '_' || c == '-':
		default:
			return fmt.Errorf("invalid character %q in project name %q. Use only letters, numbers, '-' and '_'", c, name)
		}
	}
	return nil
}

func homepageWorkspacePathGuidance() string {
	return "Configure homepage.workspace_path as the absolute host directory mounted as /workspace. In homepage tool calls, use relative project_dir/path values like 'my-site' or 'my-site/src/app/page.tsx', never '/workspace/my-site' or host filesystem paths."
}

func homepageWorkspacePathNotConfiguredJSON() string {
	return errJSON("workspace_path not configured. %s", homepageWorkspacePathGuidance())
}

func validateHomepageRelativePathArg(path, field string) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return fmt.Errorf("%s is required", field)
	}
	if strings.Contains(trimmed, "..") {
		return fmt.Errorf("path traversal not allowed in %s %q", field, path)
	}
	// Reject shell metacharacters that could be used for command injection.
	const shellMetachars = ";|&`$(){}[]<>!\\'\""
	for _, ch := range shellMetachars {
		if strings.ContainsRune(trimmed, ch) {
			return fmt.Errorf("invalid character %q in %s %q", ch, field, path)
		}
	}
	normalized := filepath.ToSlash(trimmed)
	if filepath.IsAbs(trimmed) || strings.HasPrefix(normalized, "/") || strings.HasPrefix(normalized, homepageWorkspaceMount+"/") {
		return fmt.Errorf("%s must be relative to the homepage workspace, e.g. 'my-site/src/app/page.tsx' not %q", field, path)
	}
	return nil
}

func validateHomepageSourceEditPath(path string) error {
	normalized := filepath.ToSlash(strings.TrimSpace(path))
	parts := strings.Split(normalized, "/")
	if len(parts) < 2 {
		return nil
	}
	switch strings.ToLower(parts[1]) {
	case "dist", "build", "out":
		return fmt.Errorf("direct edits to generated output directory %q are blocked. Edit source files with homepage write_file/edit_file, run the project build, then deploy the generated output", parts[1])
	default:
		return nil
	}
}

// resolveHomepagePath resolves a workspace-relative path and validates that the
// result stays within the workspace root. Does not allow the workspace root itself.
// Returns (fullPath, nil) on success, or ("", error) on path traversal.
func resolveHomepagePath(workspacePath, relPath string) (string, error) {
	fullPath := filepath.Join(workspacePath, filepath.FromSlash(relPath))
	cleanWS := filepath.Clean(workspacePath)
	cleanFull := filepath.Clean(fullPath)
	if cleanFull == cleanWS || !strings.HasPrefix(cleanFull, cleanWS+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal not allowed in path %q", relPath)
	}
	return fullPath, nil
}

// truncateStr returns s truncated to maxLen characters with "…" suffix.
func truncateStr(s string, maxLen int) string {
	if maxLen <= 0 {
		return "\u2026"
	}
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	i := 0
	for _, r := range s {
		if i >= maxLen {
			break
		}
		sb.WriteRune(r)
		i++
	}
	return sb.String() + "\u2026"
}

// maxExtractOutputSize is the maximum size of a JSON result to parse.
// Large outputs that exceed this limit are returned as-is without parsing
// to prevent memory exhaustion from json.Unmarshal on huge Docker outputs.

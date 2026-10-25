package tools

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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

// Quoted paths support Unicode names, spaces and Next.js route segments.
// Controls, quotes, substitutions and shell operators remain excluded.
var homepagePathAllowlist = regexp.MustCompile(`^[\p{L}\p{M}\p{N} ._/@()\[\]-]+$`)

// validateHomepagePathSegment validates a workspace-relative path against the
// Homepage path allowlist before the path can reach a container command string.
//
// The allowlist is checked first and quoting second: a path that is not on the
// allowlist is rejected outright, and every path that survives is still quoted
// with shellSingleQuote before it is interpolated into a command.
func validateHomepagePathSegment(value string) error {
	if value == "" {
		return fmt.Errorf("path is required")
	}
	if value == "." {
		// The workspace root itself is a valid container path.
		return nil
	}
	if !homepagePathAllowlist.MatchString(value) {
		return fmt.Errorf("invalid path %q. Use letters, numbers, spaces, '.', '_', '-', '@', parentheses or brackets in workspace-relative paths", value)
	}
	for _, segment := range strings.Split(value, "/") {
		if strings.TrimSpace(segment) != segment {
			return fmt.Errorf("invalid path %q. Path segments must not start or end with whitespace", value)
		}
		switch segment {
		case "":
			return fmt.Errorf("invalid path %q. Path segments must not be empty", value)
		case ".", "..":
			return fmt.Errorf("invalid path %q. Path segments must not be '.' or '..'", value)
		}
		if strings.HasPrefix(segment, "-") {
			return fmt.Errorf("invalid path %q. Path segments must not start with '-'", value)
		}
	}
	return nil
}

// homepagePackageNamePattern matches an npm package name: an optional scope,
// a name that starts with a letter, a digit, '-' or '~'.
var homepagePackageNamePattern = regexp.MustCompile(`^(?:@[a-z0-9-~][a-z0-9-._~]*/)?[a-z0-9-~][a-z0-9-._~]*$`)

// homepagePackageVersionPattern matches the version range of a package
// requirement. Whitespace and every shell metacharacter are excluded.
var homepagePackageVersionPattern = regexp.MustCompile(`^[a-zA-Z0-9.*~^><=\-]+$`)

// validateHomepagePackageName validates an npm package requirement against the
// Homepage dependency allowlist. Only a package requirement is accepted: no
// URL, no local path, no flag, no whitespace and no shell metacharacter.
func validateHomepagePackageName(name string) error {
	if name == "" {
		return fmt.Errorf("package name is required")
	}
	if strings.TrimSpace(name) != name {
		return fmt.Errorf("invalid package %q. Surrounding whitespace is not allowed", name)
	}
	base := name
	version := ""
	if idx := strings.Index(name, "@"); idx > 0 {
		base = name[:idx]
		version = name[idx+1:]
	} else if strings.HasPrefix(name, "@") {
		// A scoped package: the version separator is the second '@'.
		if idx := strings.Index(name[1:], "@"); idx > 0 {
			base = name[:idx+1]
			version = name[idx+2:]
		}
	}
	if !homepagePackageNamePattern.MatchString(base) {
		return fmt.Errorf("invalid package %q. Use a package requirement such as 'left-pad', '@scope/pkg' or 'pkg@1.2.3'", name)
	}
	if version != "" && !homepagePackageVersionPattern.MatchString(version) {
		return fmt.Errorf("invalid version range in package %q. Use a version such as '1.2.3' or '^1.0.0'", name)
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
	if filepath.IsAbs(projectDir) || isWindowsDriveAbsolutePath(projectDir) || strings.HasPrefix(projectDir, "/") || strings.HasPrefix(projectDir, "\\") {
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

func isWindowsDriveAbsolutePath(path string) bool {
	if len(path) < 3 {
		return false
	}
	drive := path[0]
	return ((drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z')) && path[1] == ':' && (path[2] == '/' || path[2] == '\\')
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
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == ".." {
			return fmt.Errorf("path traversal not allowed in %s %q", field, path)
		}
	}
	// Brackets and parentheses in route segments are protected by shell quoting.
	const shellMetachars = ";|&`${}<>!\\'\""
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
	// A generated output directory is blocked at the workspace root ("dist/index.html")
	// and directly below the project directory ("site/dist/index.html"). A deeper
	// source folder ("site/src/build/helpers.ts") is a source file, not build output.
	for _, index := range []int{0, 1} {
		if index >= len(parts) {
			break
		}
		switch strings.ToLower(parts[index]) {
		case "dist", "build", "out":
			return fmt.Errorf("direct edits to generated output directory %q are blocked. Edit source files with homepage write_file/edit_file, run the project build, then deploy the generated output", parts[index])
		}
	}
	return nil
}

// resolveHomepagePath resolves a workspace-relative path and validates that the
// result stays within the workspace root. Does not allow the workspace root itself.
// Symlinks are resolved before the check, so a link inside the workspace that
// points outside of it is rejected.
// Returns (fullPath, nil) on success, or ("", error) on path traversal.
func resolveHomepagePath(workspacePath, relPath string) (string, error) {
	fullPath := filepath.Join(workspacePath, filepath.FromSlash(relPath))
	cleanWS := filepath.Clean(workspacePath)
	cleanFull := filepath.Clean(fullPath)
	if cleanFull == cleanWS || !strings.HasPrefix(cleanFull, cleanWS+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal not allowed in path %q", relPath)
	}
	resolvedWS := cleanWS
	if resolved, err := filepath.EvalSymlinks(cleanWS); err == nil {
		resolvedWS = resolved
	}
	resolvedFull := resolveHomepageExistingPrefix(cleanFull)
	if resolvedFull == resolvedWS || !strings.HasPrefix(resolvedFull, resolvedWS+string(os.PathSeparator)) {
		return "", fmt.Errorf("path %q resolves outside the homepage workspace", relPath)
	}
	return fullPath, nil
}

// resolveHomepageExistingPrefix resolves the symlinks of the longest existing
// prefix of path and re-appends the segments that do not exist yet.
func resolveHomepageExistingPrefix(path string) string {
	current := path
	rest := []string{}
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			parts := append([]string{resolved}, rest...)
			return filepath.Join(parts...)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return path
		}
		rest = append([]string{filepath.Base(current)}, rest...)
		current = parent
	}
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

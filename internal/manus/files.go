package manus

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var unsafeFilenameChars = regexp.MustCompile(`[^A-Za-z0-9._() -]+`)

var blockedUploadExtensions = map[string]struct{}{
	".bat": {}, ".cmd": {}, ".com": {}, ".dll": {}, ".exe": {}, ".js": {}, ".msi": {},
	".ps1": {}, ".py": {}, ".scr": {}, ".sh": {}, ".ts": {},
	".db": {}, ".db3": {}, ".sqlite": {}, ".sqlite3": {},
}

var blockedUploadNames = map[string]struct{}{
	".env": {}, "config.yaml": {}, "config.yml": {}, "vault.bin": {}, "aurago_master_key": {},
}

// LocalFile is a validated regular workspace file.
type LocalFile struct {
	Path     string
	Filename string
	Size     int64
}

// ResolveUploadPath resolves symlinks and enforces the AuraGo workspace boundary.
func ResolveUploadPath(workspaceDir, requestedPath string, maxBytes int64) (LocalFile, error) {
	workspaceDir = strings.TrimSpace(workspaceDir)
	requestedPath = strings.TrimSpace(requestedPath)
	if workspaceDir == "" || requestedPath == "" {
		return LocalFile{}, fmt.Errorf("Manus upload requires a workspace and file path")
	}
	workspaceAbs, err := filepath.Abs(workspaceDir)
	if err != nil {
		return LocalFile{}, fmt.Errorf("resolve Manus workspace: %w", err)
	}
	workspaceResolved, err := filepath.EvalSymlinks(workspaceAbs)
	if err != nil {
		return LocalFile{}, fmt.Errorf("resolve Manus workspace symlinks: %w", err)
	}
	candidate := requestedPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(workspaceAbs, candidate)
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return LocalFile{}, fmt.Errorf("resolve Manus upload path: %w", err)
	}
	if !pathWithin(workspaceAbs, candidateAbs) {
		return LocalFile{}, fmt.Errorf("Manus upload path leaves the agent workspace")
	}
	if err := rejectSymlinkComponents(workspaceAbs, candidateAbs); err != nil {
		return LocalFile{}, err
	}
	candidateResolved, err := filepath.EvalSymlinks(candidateAbs)
	if err != nil {
		return LocalFile{}, fmt.Errorf("resolve Manus upload symlinks: %w", err)
	}
	if !pathWithin(workspaceResolved, candidateResolved) {
		return LocalFile{}, fmt.Errorf("Manus upload path leaves the agent workspace")
	}
	info, err := os.Stat(candidateResolved)
	if err != nil {
		return LocalFile{}, fmt.Errorf("stat Manus upload file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return LocalFile{}, fmt.Errorf("Manus upload path is not a regular file")
	}
	if maxBytes <= 0 || info.Size() > maxBytes {
		return LocalFile{}, fmt.Errorf("Manus upload file exceeds the configured size limit")
	}
	if err := validateUploadName(candidateResolved); err != nil {
		return LocalFile{}, err
	}
	return LocalFile{Path: candidateResolved, Filename: filepath.Base(candidateResolved), Size: info.Size()}, nil
}

func rejectSymlinkComponents(root, candidate string) error {
	relative, err := filepath.Rel(root, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("Manus upload path leaves the agent workspace")
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("inspect Manus upload path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("Manus upload blocks symlinks")
		}
	}
	return nil
}

func validateUploadName(path string) error {
	lowerName := strings.ToLower(filepath.Base(path))
	if _, blocked := blockedUploadNames[lowerName]; blocked {
		return fmt.Errorf("Manus upload blocks sensitive file %q", lowerName)
	}
	if _, blocked := blockedUploadExtensions[strings.ToLower(filepath.Ext(lowerName))]; blocked {
		return fmt.Errorf("Manus upload blocks scripts, executables, and databases")
	}
	for _, part := range strings.FieldsFunc(strings.ToLower(filepath.Clean(path)), func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if part == ".git" || part == ".ssh" || part == "vault" {
			return fmt.Errorf("Manus upload blocks sensitive directory %q", part)
		}
	}
	return nil
}

func pathWithin(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// ValidateRemoteFileURL accepts only credential-free public HTTPS URLs.
func ValidateRemoteFileURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Hostname() == "" {
		return nil, fmt.Errorf("Manus file URL must use HTTPS")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("Manus file URL must not contain credentials")
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && !isPublicIP(ip) {
		return nil, fmt.Errorf("Manus file URL resolves to a private or local address")
	}
	return parsed, nil
}

func isPublicIP(ip net.IP) bool {
	return !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() &&
		!ip.IsUnspecified() && !ip.IsMulticast()
}

// SafeAttachmentFilename converts untrusted attachment names into one local basename.
func SafeAttachmentFilename(raw string) string {
	raw = strings.ReplaceAll(raw, "\\", "/")
	name := filepath.Base(strings.TrimSpace(raw))
	name = strings.TrimSpace(unsafeFilenameChars.ReplaceAllString(name, "_"))
	name = strings.Trim(name, ". ")
	if name == "" {
		return "attachment.bin"
	}
	return name
}

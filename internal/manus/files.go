package manus

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var unsafeFilenameChars = regexp.MustCompile(`[^A-Za-z0-9._() -]+`)

var blockedUploadExtensions = map[string]struct{}{
	".apk": {}, ".app": {}, ".bat": {}, ".bin": {}, ".class": {}, ".cmd": {}, ".com": {}, ".deb": {},
	".dll": {}, ".dmg": {}, ".dylib": {}, ".elf": {}, ".exe": {}, ".hta": {}, ".ipa": {}, ".jar": {},
	".js": {}, ".jse": {}, ".msi": {}, ".msp": {}, ".out": {}, ".rpm": {}, ".run": {}, ".scr": {},
	".so": {}, ".vb": {}, ".vbe": {}, ".vbs": {}, ".wsf": {}, ".wsh": {},
	".bash": {}, ".csh": {}, ".fish": {}, ".ksh": {}, ".lua": {}, ".php": {}, ".pl": {}, ".ps1": {},
	".py": {}, ".pyc": {}, ".pyo": {}, ".rb": {}, ".sh": {}, ".ts": {}, ".zsh": {},
	".cfg": {}, ".conf": {}, ".ini": {}, ".toml": {}, ".yaml": {}, ".yml": {},
	".cer": {}, ".crt": {}, ".der": {}, ".key": {}, ".p12": {}, ".pfx": {}, ".pem": {},
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
	handle   *os.File
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
	workspaceInfo, err := os.Lstat(workspaceAbs)
	if err != nil {
		return LocalFile{}, fmt.Errorf("inspect Manus workspace: %w", err)
	}
	if workspaceInfo.Mode()&os.ModeSymlink != 0 {
		return LocalFile{}, fmt.Errorf("Manus upload blocks a symlinked workspace root")
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
	relative, err := filepath.Rel(workspaceAbs, candidateAbs)
	if err != nil {
		return LocalFile{}, fmt.Errorf("resolve Manus upload relative path: %w", err)
	}
	root, err := os.OpenRoot(workspaceAbs)
	if err != nil {
		return LocalFile{}, fmt.Errorf("open Manus workspace root: %w", err)
	}
	defer root.Close()
	file, err := root.Open(relative)
	if err != nil {
		return LocalFile{}, fmt.Errorf("open Manus upload file within workspace root: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return LocalFile{}, fmt.Errorf("stat Manus upload file: %w", err)
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return LocalFile{}, fmt.Errorf("Manus upload path is not a regular file")
	}
	if maxBytes <= 0 || info.Size() > maxBytes {
		_ = file.Close()
		return LocalFile{}, fmt.Errorf("Manus upload file exceeds the configured size limit")
	}
	if info.Mode().Perm()&0o111 != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 {
		_ = file.Close()
		return LocalFile{}, fmt.Errorf("Manus upload blocks executable file modes")
	}
	if err := validateUploadName(candidateAbs); err != nil {
		_ = file.Close()
		return LocalFile{}, err
	}
	if err := validateUploadContent(file); err != nil {
		_ = file.Close()
		return LocalFile{}, err
	}
	return LocalFile{Path: candidateAbs, Filename: filepath.Base(candidateAbs), Size: info.Size(), handle: file}, nil
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

func validateUploadContent(file *os.File) error {
	header := make([]byte, 8)
	read, err := file.Read(header)
	if err != nil && err != io.EOF {
		return fmt.Errorf("inspect Manus upload content: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("reset Manus upload file: %w", err)
	}
	header = header[:read]
	if bytes.HasPrefix(header, []byte("#!")) || bytes.HasPrefix(header, []byte{0xef, 0xbb, 0xbf, '#', '!'}) {
		return fmt.Errorf("Manus upload blocks script shebangs")
	}
	blockedMagic := [][]byte{
		{'M', 'Z'}, {0x7f, 'E', 'L', 'F'},
		{0xfe, 0xed, 0xfa, 0xce}, {0xce, 0xfa, 0xed, 0xfe},
		{0xfe, 0xed, 0xfa, 0xcf}, {0xcf, 0xfa, 0xed, 0xfe},
		{0xca, 0xfe, 0xba, 0xbe}, {0xbe, 0xba, 0xfe, 0xca},
	}
	for _, magic := range blockedMagic {
		if bytes.HasPrefix(header, magic) {
			return fmt.Errorf("Manus upload blocks executable file content")
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

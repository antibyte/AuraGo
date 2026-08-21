package tools

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var hereNowPrivateKeyMarkers = [][]byte{
	[]byte("-----BEGIN PRIVATE KEY-----"),
	[]byte("-----BEGIN ENCRYPTED PRIVATE KEY-----"),
	[]byte("-----BEGIN RSA PRIVATE KEY-----"),
	[]byte("-----BEGIN DSA PRIVATE KEY-----"),
	[]byte("-----BEGIN EC PRIVATE KEY-----"),
	[]byte("-----BEGIN OPENSSH PRIVATE KEY-----"),
	[]byte("-----BEGIN PGP PRIVATE KEY BLOCK-----"),
}

type hereNowPublishSnapshot struct {
	root  string
	files []hereNowPublishFile
}

func (s *hereNowPublishSnapshot) Close() error {
	if s == nil || s.root == "" {
		return nil
	}
	err := os.RemoveAll(s.root)
	s.root = ""
	return err
}

func normalizeHereNowDeployRelative(value string) (string, error) {
	value = strings.TrimSpace(filepath.ToSlash(value))
	if value == "" {
		value = "."
	}
	if filepath.IsAbs(value) || filepath.VolumeName(value) != "" {
		return "", fmt.Errorf("here.now deploy path must be relative to the Homepage workspace")
	}
	clean := path.Clean(value)
	if clean == ".." || strings.HasPrefix(clean, "../") || (clean != "." && !fs.ValidPath(clean)) {
		return "", fmt.Errorf("here.now deploy path escapes the Homepage workspace")
	}
	return clean, nil
}

func validateHereNowDirectoryComponents(root *os.Root, relative string) error {
	if relative == "." {
		return nil
	}
	current := ""
	for _, component := range strings.Split(relative, "/") {
		current = path.Join(current, component)
		info, err := root.Lstat(current)
		if err != nil {
			return fmt.Errorf("inspect here.now deploy path %q: %w", current, err)
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return fmt.Errorf("symbolic links and reparse points are not allowed in here.now deploy paths: %s", current)
		}
		if !info.IsDir() {
			return fmt.Errorf("here.now deploy path component is not a directory: %s", current)
		}
	}
	return nil
}

func hereNowSkipDirectory(rel string) bool {
	for _, component := range strings.Split(strings.ToLower(rel), "/") {
		switch component {
		case ".aurago", ".cache", ".git", ".next", ".netlify", ".parcel-cache", ".turbo", ".vercel", ".vite", "node_modules":
			return true
		}
	}
	return false
}

func hereNowSensitivePath(rel string) bool {
	lower := strings.Trim(strings.ToLower(filepath.ToSlash(rel)), "/")
	parts := strings.Split(lower, "/")
	for i, component := range parts {
		switch component {
		case ".aws", ".azure", ".direnv", ".gnupg", ".kube", ".ssh", ".vault":
			return true
		case "gcloud":
			if i > 0 && parts[i-1] == ".config" {
				return true
			}
		}
	}
	base := path.Base(lower)
	if base == ".env" || base == ".envrc" || strings.HasPrefix(base, ".env.") {
		return true
	}
	blocked := map[string]struct{}{
		".dockercfg": {}, ".git-credentials": {}, ".htpasswd": {}, ".netrc": {},
		".npmrc": {}, ".pnpmrc": {}, ".yarnrc": {}, ".yarnrc.yml": {},
		"application_default_credentials.json": {}, "authorized_keys": {},
		"config.yaml": {}, "config.yml": {}, "credentials": {}, "credentials.ini": {},
		"credentials.json": {}, "id_dsa": {}, "id_ecdsa": {}, "id_ed25519": {},
		"id_rsa": {}, "known_hosts": {}, "secret.json": {}, "secrets.json": {},
		"service-account.json": {}, "vault.bin": {}, "vault.json": {}, "vault.yaml": {}, "vault.yml": {},
	}
	if _, exists := blocked[base]; exists {
		return true
	}
	if strings.HasPrefix(base, "deploy_key") || strings.HasPrefix(base, "service-account-") || strings.HasPrefix(base, "service_account_") {
		return true
	}
	for _, suffix := range []string{
		".age", ".db", ".gpg", ".jks", ".kdbx", ".key", ".keystore", ".log",
		".p12", ".p8", ".pem", ".pfx", ".pkcs12", ".ppk", ".sqlite", ".sqlite3",
		".sqlite-shm", ".sqlite-wal",
	} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

type hereNowSecretScanner struct {
	tail   []byte
	prefix []byte
}

func (s *hereNowSecretScanner) Write(chunk []byte) (int, error) {
	if len(s.prefix) < 4 {
		needed := 4 - len(s.prefix)
		if needed > len(chunk) {
			needed = len(chunk)
		}
		s.prefix = append(s.prefix, chunk[:needed]...)
		if len(s.prefix) == 4 && (bytes.Equal(s.prefix, []byte{0xfe, 0xed, 0xfe, 0xed}) || bytes.Equal(s.prefix, []byte{0xce, 0xce, 0xce, 0xce})) {
			return 0, fmt.Errorf("Java keystore material detected")
		}
	}
	window := make([]byte, 0, len(s.tail)+len(chunk))
	window = append(window, s.tail...)
	window = append(window, chunk...)
	upper := bytes.ToUpper(window)
	for _, marker := range hereNowPrivateKeyMarkers {
		if bytes.Contains(upper, marker) {
			return 0, fmt.Errorf("private key material detected")
		}
	}
	const tailSize = 64
	if len(window) > tailSize {
		s.tail = append(s.tail[:0], window[len(window)-tailSize:]...)
	} else {
		s.tail = append(s.tail[:0], window...)
	}
	return len(chunk), nil
}

func copyHereNowSnapshotFile(sourceRoot *os.Root, snapshotRoot, rel string, expected os.FileInfo) (hereNowPublishFile, error) {
	source, err := sourceRoot.Open(rel)
	if err != nil {
		return hereNowPublishFile{}, fmt.Errorf("open %s: %w", rel, err)
	}
	defer source.Close()
	opened, err := source.Stat()
	if err != nil {
		return hereNowPublishFile{}, fmt.Errorf("inspect opened file %s: %w", rel, err)
	}
	if !opened.Mode().IsRegular() || !os.SameFile(expected, opened) {
		return hereNowPublishFile{}, fmt.Errorf("here.now source changed while opening: %s", rel)
	}

	destinationPath := filepath.Join(snapshotRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o700); err != nil {
		return hereNowPublishFile{}, fmt.Errorf("create snapshot directory for %s: %w", rel, err)
	}
	destination, err := os.OpenFile(destinationPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return hereNowPublishFile{}, fmt.Errorf("create snapshot file %s: %w", rel, err)
	}
	hash := sha256.New()
	scanner := &hereNowSecretScanner{}
	written, copyErr := io.Copy(io.MultiWriter(destination, hash, scanner), source)
	closeErr := destination.Close()
	if copyErr != nil {
		return hereNowPublishFile{}, fmt.Errorf("snapshot %s: %w", rel, copyErr)
	}
	if closeErr != nil {
		return hereNowPublishFile{}, fmt.Errorf("close snapshot file %s: %w", rel, closeErr)
	}
	after, err := source.Stat()
	if err != nil {
		return hereNowPublishFile{}, fmt.Errorf("reinspect source file %s: %w", rel, err)
	}
	if !os.SameFile(opened, after) || opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) || written != opened.Size() {
		return hereNowPublishFile{}, fmt.Errorf("here.now source changed while snapshotting: %s", rel)
	}
	contentType := mime.TypeByExtension(strings.ToLower(path.Ext(rel)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return hereNowPublishFile{
		Path: rel, Size: written, ContentType: contentType,
		Hash: hex.EncodeToString(hash.Sum(nil)), SourcePath: destinationPath,
	}, nil
}

func buildHereNowSnapshot(workspaceRoot, deployRelative string) (_ *hereNowPublishSnapshot, err error) {
	workspaceRoot, err = filepath.Abs(strings.TrimSpace(workspaceRoot))
	if err != nil {
		return nil, fmt.Errorf("resolve Homepage workspace: %w", err)
	}
	rootInfo, err := os.Lstat(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("inspect Homepage workspace: %w", err)
	}
	if !rootInfo.IsDir() || rootInfo.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return nil, fmt.Errorf("Homepage workspace must be a real directory without symlinks or reparse points")
	}
	deployRelative, err = normalizeHereNowDeployRelative(deployRelative)
	if err != nil {
		return nil, err
	}
	workspace, err := os.OpenRoot(workspaceRoot)
	if err != nil {
		return nil, fmt.Errorf("open Homepage workspace: %w", err)
	}
	defer workspace.Close()
	if err := validateHereNowDirectoryComponents(workspace, deployRelative); err != nil {
		return nil, err
	}
	deployRoot, err := workspace.OpenRoot(deployRelative)
	if err != nil {
		return nil, fmt.Errorf("open here.now deploy directory: %w", err)
	}
	defer deployRoot.Close()

	snapshotPath, err := os.MkdirTemp("", "aurago-here-now-*")
	if err != nil {
		return nil, fmt.Errorf("create private here.now snapshot: %w", err)
	}
	if err := os.Chmod(snapshotPath, 0o700); err != nil {
		_ = os.RemoveAll(snapshotPath)
		return nil, fmt.Errorf("secure private here.now snapshot: %w", err)
	}
	snapshot := &hereNowPublishSnapshot{root: snapshotPath, files: make([]hereNowPublishFile, 0, 32)}
	defer func() {
		if err != nil {
			_ = snapshot.Close()
		}
	}()

	err = fs.WalkDir(deployRoot.FS(), ".", func(rel string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if rel == "." {
			return nil
		}
		rel = path.Clean(filepath.ToSlash(rel))
		info, infoErr := deployRoot.Lstat(rel)
		if infoErr != nil {
			return fmt.Errorf("inspect here.now source %s: %w", rel, infoErr)
		}
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return fmt.Errorf("symbolic links, reparse points, and special files are not allowed in here.now deployments: %s", rel)
		}
		if info.IsDir() {
			if hereNowSensitivePath(rel) {
				return fmt.Errorf("sensitive directory is not allowed in here.now deployments: %s", rel)
			}
			if hereNowSkipDirectory(rel) {
				return fs.SkipDir
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("special files are not allowed in here.now deployments: %s", rel)
		}
		if path.Base(strings.ToLower(rel)) == homepageArtifactManifestName {
			return nil
		}
		if hereNowSkipDirectory(rel) {
			return nil
		}
		if hereNowSensitivePath(rel) {
			return fmt.Errorf("sensitive file is not allowed in here.now deployments: %s", rel)
		}
		if len(snapshot.files) >= hereNowMaxFiles {
			return fmt.Errorf("here.now deployments support at most %d files", hereNowMaxFiles)
		}
		file, copyErr := copyHereNowSnapshotFile(deployRoot, snapshotPath, rel, info)
		if copyErr != nil {
			return copyErr
		}
		snapshot.files = append(snapshot.files, file)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(snapshot.files) == 0 {
		return nil, fmt.Errorf("deploy path contains no publishable files")
	}
	return snapshot, nil
}

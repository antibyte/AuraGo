package embeddings

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofrs/flock"
)

type assetCache struct {
	root       string
	client     *http.Client
	onProgress func(DownloadStatus)
}

const runtimeManifestVersion = 1

type runtimeManifestFile struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type runtimeCompleteManifest struct {
	Version int                   `json:"version"`
	Asset   string                `json:"asset"`
	SHA256  string                `json:"sha256"`
	Size    int64                 `json:"size"`
	Files   []runtimeManifestFile `json:"files"`
}

func newAssetCache(root string, onProgress func(DownloadStatus)) *assetCache {
	return &assetCache{
		root: root,
		client: &http.Client{
			Timeout: 30 * time.Minute,
			CheckRedirect: func(request *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				if request.URL.Scheme != "https" {
					return fmt.Errorf("refusing non-HTTPS redirect to %s", request.URL.Redacted())
				}
				return nil
			},
		},
		onProgress: onProgress,
	}
}

func (c *assetCache) withProcessLock(ctx context.Context, operation func() error) error {
	if err := os.MkdirAll(c.root, 0o750); err != nil {
		return fmt.Errorf("create embeddings cache: %w", err)
	}
	lock := flock.New(filepath.Join(c.root, ".download.lock"))
	locked, err := lock.TryLockContext(ctx, 250*time.Millisecond)
	if err != nil {
		return fmt.Errorf("lock embeddings cache: %w", err)
	}
	if !locked {
		return fmt.Errorf("lock embeddings cache: %w", ctx.Err())
	}
	defer func() {
		_ = lock.Unlock()
	}()
	return operation()
}

func (c *assetCache) ensureDirectAsset(ctx context.Context, spec assetSpec, destination string) error {
	return c.withProcessLock(ctx, func() error {
		return c.ensureFileUnlocked(ctx, spec, destination)
	})
}

func (c *assetCache) ensureRuntimeAsset(ctx context.Context, spec assetSpec) (string, error) {
	if spec.ID == "" {
		return "", fmt.Errorf("runtime asset is not available for this platform")
	}
	target := runtimeAssetPath(c.root, spec)
	err := c.withProcessLock(ctx, func() error {
		archivePath := filepath.Join(c.root, "downloads", spec.ID+archiveSuffix(spec.Kind))
		if err := c.ensureFileUnlocked(ctx, spec, archivePath); err != nil {
			return err
		}
		if runtimeManifestMatches(target, spec) {
			return nil
		}

		if err := safeRemoveAll(filepath.Join(c.root, "runtimes"), target); err != nil {
			return fmt.Errorf("clear incomplete runtime target: %w", err)
		}
		suffix, err := randomSuffix()
		if err != nil {
			return err
		}
		tempTarget := target + ".tmp-" + suffix
		if err := safeRemoveAll(filepath.Join(c.root, "runtimes"), tempTarget); err != nil {
			return fmt.Errorf("clear runtime extraction temp: %w", err)
		}
		if err := os.MkdirAll(tempTarget, 0o750); err != nil {
			return fmt.Errorf("create runtime extraction target: %w", err)
		}
		cleanup := true
		defer func() {
			if cleanup {
				_ = safeRemoveAll(filepath.Join(c.root, "runtimes"), tempTarget)
			}
		}()

		switch spec.Kind {
		case assetZip:
			err = extractZipSecure(archivePath, tempTarget)
		case assetTarGzip:
			err = extractTarGzipSecure(archivePath, tempTarget)
		default:
			err = fmt.Errorf("runtime asset %s has unsupported archive kind %q", spec.ID, spec.Kind)
		}
		if err != nil {
			return fmt.Errorf("extract runtime %s: %w", spec.ID, err)
		}
		files, err := buildRuntimeFileManifest(tempTarget)
		if err != nil {
			return fmt.Errorf("inventory runtime %s: %w", spec.ID, err)
		}
		marker := runtimeCompleteManifest{
			Version: runtimeManifestVersion,
			Asset:   spec.ID,
			SHA256:  spec.SHA256,
			Size:    spec.Size,
			Files:   files,
		}
		raw, err := json.Marshal(marker)
		if err != nil {
			return fmt.Errorf("marshal runtime marker: %w", err)
		}
		if err := os.WriteFile(filepath.Join(tempTarget, ".complete.json"), raw, 0o600); err != nil {
			return fmt.Errorf("write runtime marker: %w", err)
		}
		if err := os.Rename(tempTarget, target); err != nil {
			return fmt.Errorf("activate runtime directory: %w", err)
		}
		cleanup = false
		return nil
	})
	if err != nil {
		return "", err
	}
	return target, nil
}

func (c *assetCache) ensureFileUnlocked(ctx context.Context, spec assetSpec, destination string) error {
	if err := validateAssetSpec(spec); err != nil {
		return err
	}
	if fileMatches(destination, spec.Size, spec.SHA256) {
		if c.onProgress != nil {
			c.onProgress(DownloadStatus{Asset: spec.ID, Downloaded: spec.Size, Total: spec.Size, Percent: 100})
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
		return fmt.Errorf("create asset directory: %w", err)
	}
	partPath := destination + ".part"
	if err := os.Remove(partPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear partial asset: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, spec.URL, nil)
	if err != nil {
		return fmt.Errorf("create asset request: %w", err)
	}
	request.Header.Set("User-Agent", "AuraGo/"+LocalGraniteProvider)
	response, err := c.client.Do(request)
	if err != nil {
		return fmt.Errorf("download %s: %w", spec.ID, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: unexpected HTTP status %s", spec.ID, response.Status)
	}
	if response.ContentLength >= 0 && response.ContentLength != spec.Size {
		return fmt.Errorf("download %s: content length %d, want %d", spec.ID, response.ContentLength, spec.Size)
	}

	file, err := os.OpenFile(partPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create partial asset: %w", err)
	}
	progress := &progressWriter{asset: spec.ID, total: spec.Size, callback: c.onProgress}
	hash := sha256.New()
	limited := io.LimitReader(response.Body, spec.Size+1)
	written, copyErr := io.Copy(io.MultiWriter(file, hash, progress), limited)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(partPath)
		return fmt.Errorf("download %s: %w", spec.ID, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(partPath)
		return fmt.Errorf("close %s partial file: %w", spec.ID, closeErr)
	}
	if written != spec.Size {
		_ = os.Remove(partPath)
		return fmt.Errorf("download %s: received %d bytes, want %d", spec.ID, written, spec.Size)
	}
	actualHash := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actualHash, spec.SHA256) {
		_ = os.Remove(partPath)
		return fmt.Errorf("download %s: SHA-256 %s, want %s", spec.ID, actualHash, spec.SHA256)
	}
	if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
		_ = os.Remove(partPath)
		return fmt.Errorf("replace invalid cached asset: %w", err)
	}
	if err := os.Rename(partPath, destination); err != nil {
		_ = os.Remove(partPath)
		return fmt.Errorf("activate downloaded asset: %w", err)
	}
	return nil
}

func validateAssetSpec(spec assetSpec) error {
	parsed, err := url.Parse(spec.URL)
	if err != nil {
		return fmt.Errorf("parse asset URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("asset %s must use an absolute HTTPS URL", spec.ID)
	}
	if spec.Size <= 0 {
		return fmt.Errorf("asset %s has invalid expected size", spec.ID)
	}
	if len(spec.SHA256) != sha256.Size*2 {
		return fmt.Errorf("asset %s has invalid SHA-256", spec.ID)
	}
	return nil
}

func fileMatches(path string, size int64, expectedHash string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != size {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false
	}
	return strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), expectedHash)
}

func runtimeManifestMatches(root string, spec assetSpec) bool {
	raw, err := os.ReadFile(filepath.Join(root, ".complete.json"))
	if err != nil {
		return false
	}
	var marker runtimeCompleteManifest
	if err := json.Unmarshal(raw, &marker); err != nil {
		return false
	}
	if marker.Version != runtimeManifestVersion ||
		marker.Asset != spec.ID ||
		marker.Size != spec.Size ||
		!strings.EqualFold(marker.SHA256, spec.SHA256) ||
		len(marker.Files) == 0 {
		return false
	}
	expected := make(map[string]runtimeManifestFile, len(marker.Files))
	for _, file := range marker.Files {
		path := filepath.Clean(filepath.FromSlash(file.Path))
		if path == "." || path == ".complete.json" || filepath.IsAbs(path) ||
			path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) ||
			file.Size < 0 || len(file.SHA256) != sha256.Size*2 {
			return false
		}
		normalized := filepath.ToSlash(path)
		if _, duplicate := expected[normalized]; duplicate {
			return false
		}
		expected[normalized] = file
	}
	seen := make(map[string]bool, len(expected))
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("runtime entry %s is a symlink", relative)
		}
		if relative == ".complete.json" {
			if !info.Mode().IsRegular() {
				return fmt.Errorf("runtime marker is not a regular file")
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		file, ok := expected[relative]
		if !ok || !info.Mode().IsRegular() || info.Size() != file.Size ||
			!fileMatches(path, file.Size, file.SHA256) {
			return fmt.Errorf("runtime entry %s failed manifest validation", relative)
		}
		seen[relative] = true
		return nil
	})
	return err == nil && len(seen) == len(expected)
}

func buildRuntimeFileManifest(root string) ([]runtimeManifestFile, error) {
	var files []runtimeManifestFile
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("runtime entry %s is not a regular file", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		hash, err := hashFile(path)
		if err != nil {
			return err
		}
		files = append(files, runtimeManifestFile{
			Path:   filepath.ToSlash(relative),
			Size:   info.Size(),
			SHA256: hash,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("runtime archive contained no regular files")
	}
	return files, nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

type progressWriter struct {
	asset    string
	total    int64
	written  int64
	lastSent time.Time
	callback func(DownloadStatus)
}

func (w *progressWriter) Write(data []byte) (int, error) {
	w.written += int64(len(data))
	if w.callback != nil && (w.lastSent.IsZero() || time.Since(w.lastSent) >= 200*time.Millisecond || w.written == w.total) {
		percent := float64(w.written) * 100 / float64(w.total)
		w.callback(DownloadStatus{Asset: w.asset, Downloaded: w.written, Total: w.total, Percent: percent})
		w.lastSent = time.Now()
	}
	return len(data), nil
}

func archiveSuffix(kind assetKind) string {
	switch kind {
	case assetZip:
		return ".zip"
	case assetTarGzip:
		return ".tar.gz"
	default:
		return ".bin"
	}
}

func extractZipSecure(archivePath, target string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer reader.Close()
	for _, entry := range reader.File {
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("zip entry %q is a symlink", entry.Name)
		}
		destination, err := secureArchiveDestination(target, entry.Name)
		if err != nil {
			return err
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0o750); err != nil {
				return fmt.Errorf("create zip directory: %w", err)
			}
			continue
		}
		if !entry.Mode().IsRegular() {
			return fmt.Errorf("zip entry %q is not a regular file", entry.Name)
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
			return fmt.Errorf("create zip parent: %w", err)
		}
		source, err := entry.Open()
		if err != nil {
			return fmt.Errorf("open zip entry: %w", err)
		}
		mode := sanitizedArchiveMode(entry.Mode(), entry.Name)
		destinationFile, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		if err != nil {
			source.Close()
			return fmt.Errorf("create zip output: %w", err)
		}
		_, copyErr := io.Copy(destinationFile, source)
		closeDestinationErr := destinationFile.Close()
		closeSourceErr := source.Close()
		if copyErr != nil {
			return fmt.Errorf("extract zip entry: %w", copyErr)
		}
		if closeDestinationErr != nil {
			return fmt.Errorf("close zip output: %w", closeDestinationErr)
		}
		if closeSourceErr != nil {
			return fmt.Errorf("close zip entry: %w", closeSourceErr)
		}
	}
	return nil
}

func extractTarGzipSecure(archivePath, target string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open tar.gz: %w", err)
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip stream: %w", err)
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}
		destination, err := secureArchiveDestination(target, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destination, 0o750); err != nil {
				return fmt.Errorf("create tar directory: %w", err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(destination), 0o750); err != nil {
				return fmt.Errorf("create tar parent: %w", err)
			}
			mode := sanitizedArchiveMode(os.FileMode(header.Mode), header.Name)
			destinationFile, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
			if err != nil {
				return fmt.Errorf("create tar output: %w", err)
			}
			_, copyErr := io.Copy(destinationFile, reader)
			closeErr := destinationFile.Close()
			if copyErr != nil {
				return fmt.Errorf("extract tar entry: %w", copyErr)
			}
			if closeErr != nil {
				return fmt.Errorf("close tar output: %w", closeErr)
			}
		case tar.TypeSymlink, tar.TypeLink:
			// Native release archives commonly contain convenience links such
			// as libonnxruntime.so -> libonnxruntime.so.1.26.0. AuraGo never
			// materializes links; the real versioned file is discovered below
			// the private runtime directory instead.
			continue
		default:
			return fmt.Errorf("tar entry %q has unsupported type %d", header.Name, header.Typeflag)
		}
	}
}

func secureArchiveDestination(root, name string) (string, error) {
	if filepath.IsAbs(name) || filepath.VolumeName(name) != "" ||
		strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") {
		return "", fmt.Errorf("archive entry %q is absolute", name)
	}
	cleanName := filepath.Clean(filepath.FromSlash(name))
	if cleanName == "." || cleanName == "" {
		return filepath.Clean(root), nil
	}
	if cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("archive entry %q escapes extraction root", name)
	}
	destination := filepath.Join(root, cleanName)
	if !pathWithinRoot(destination, root) {
		return "", fmt.Errorf("archive entry %q escapes extraction root", name)
	}
	return destination, nil
}

func sanitizedArchiveMode(mode os.FileMode, name string) os.FileMode {
	result := os.FileMode(0o640)
	lower := strings.ToLower(filepath.Base(name))
	if mode&0o111 != 0 || lower == "llama-server" || lower == "llama-server.exe" {
		result = 0o750
	}
	return result
}

func pathWithinRoot(path, root string) bool {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(filepath.Clean(absoluteRoot), filepath.Clean(absolutePath))
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func safeRemoveAll(root, target string) error {
	if strings.TrimSpace(target) == "" || strings.TrimSpace(root) == "" {
		return fmt.Errorf("remove target and root are required")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve remove root: %w", err)
	}
	absoluteTarget, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve remove target: %w", err)
	}
	if filepath.Clean(absoluteRoot) == filepath.Clean(absoluteTarget) || !pathWithinRoot(absoluteTarget, absoluteRoot) {
		return fmt.Errorf("refusing to recursively remove %s outside %s", absoluteTarget, absoluteRoot)
	}
	if err := os.RemoveAll(absoluteTarget); err != nil {
		return fmt.Errorf("remove %s: %w", absoluteTarget, err)
	}
	return nil
}

func randomSuffix() (string, error) {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("generate random suffix: %w", err)
	}
	return hex.EncodeToString(data[:]), nil
}

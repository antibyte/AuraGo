package decisions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aurago/internal/fileutil"

	"github.com/gofrs/flock"
)

const (
	modelDownloadChunk = 1 << 20
	maxModelRedirects  = 8
)

var errArtifactIntegrity = errors.New("artifact size or checksum mismatch")

type artifactCache struct {
	root       string
	client     *http.Client
	progressFn func(phase string, current, total int64)
	urlFor     func(Manifest, Artifact) string
}

func newArtifactCache(root string, client *http.Client, progressFn func(string, int64, int64)) *artifactCache {
	if client == nil {
		client = newModelHTTPClient()
	}
	return &artifactCache{root: root, client: client, progressFn: progressFn, urlFor: artifactURL}
}

func newModelHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 0,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= maxModelRedirects {
				return fmt.Errorf("too many model download redirects")
			}
			if request.URL.Scheme != "https" || !trustedModelHost(request.URL.Hostname()) {
				return fmt.Errorf("model download redirected to an untrusted host")
			}
			return nil
		},
	}
}

func trustedModelHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "huggingface.co" || strings.HasSuffix(host, ".huggingface.co") || host == "hf.co" || strings.HasSuffix(host, ".hf.co")
}

func (cache *artifactCache) ensure(ctx context.Context, manifest Manifest) (modelPaths, error) {
	if err := manifest.Validate(); err != nil {
		return modelPaths{}, WrapError(CodeModelIntegrityFailed, err)
	}
	if err := os.MkdirAll(cache.root, 0o700); err != nil {
		return modelPaths{}, WrapError(CodeModelInstallFailed, err)
	}
	lock := flock.New(filepath.Join(cache.root, ".install.lock"))
	locked, err := lock.TryLockContext(ctx, 250*time.Millisecond)
	if err != nil {
		return modelPaths{}, WrapError(CodeModelInstallFailed, err)
	}
	if !locked {
		return modelPaths{}, WrapError(CodeModelInstallBusy, ctx.Err())
	}
	defer func() { _ = lock.Unlock() }()

	modelPath, err := cache.ensureFile(ctx, manifest, manifest.Model, "downloading_model")
	if err != nil {
		return modelPaths{}, err
	}
	projectorPath, err := cache.ensureFile(ctx, manifest, manifest.Projector, "downloading_projector")
	if err != nil {
		return modelPaths{}, err
	}
	return modelPaths{Model: modelPath, Projector: projectorPath}, nil
}

type modelPaths struct {
	Model     string
	Projector string
}

func (cache *artifactCache) ensureFile(ctx context.Context, manifest Manifest, artifact Artifact, phase string) (string, error) {
	destination := filepath.Join(cache.root, artifact.Filename)
	if err := validateManagedFilePath(cache.root, destination); err != nil {
		return "", WrapError(CodeModelInstallFailed, err)
	}
	if err := verifyArtifactFile(ctx, destination, artifact, cache.progressFn); err == nil {
		if cache.progressFn != nil {
			cache.progressFn("verified_"+artifact.Name, artifact.Size, artifact.Size)
		}
		return destination, nil
	} else if errors.Is(err, errArtifactIntegrity) {
		info, statErr := os.Lstat(destination)
		if statErr == nil && !info.Mode().IsRegular() {
			return "", WrapError(CodeModelInstallFailed, fmt.Errorf("managed artifact path is not a regular file"))
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", WrapError(CodeModelInstallFailed, statErr)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", WrapError(CodeModelInstallFailed, err)
	}
	if err := cache.download(ctx, manifest, artifact, destination, phase); err != nil {
		return "", err
	}
	return destination, nil
}

func validateManagedFilePath(root, destination string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	destinationAbs, err := filepath.Abs(destination)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(rootAbs, destinationAbs)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("artifact path escaped model directory")
	}
	return nil
}

func verifyArtifactFile(ctx context.Context, path string, artifact Artifact, progress func(string, int64, int64)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != artifact.Size {
		return fmt.Errorf("%w: managed file type or size mismatch", errArtifactIntegrity)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	writer := &artifactProgressWriter{ctx: ctx, writer: hash, total: artifact.Size, phase: "verifying_" + artifact.Name, callback: progress}
	if progress != nil {
		progress(writer.phase, 0, artifact.Size)
	}
	if _, err := io.CopyBuffer(writer, file, make([]byte, modelDownloadChunk)); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != strings.ToLower(artifact.SHA256) {
		return fmt.Errorf("%w: managed artifact checksum mismatch", errArtifactIntegrity)
	}
	return nil
}

func (cache *artifactCache) download(ctx context.Context, manifest Manifest, artifact Artifact, destination, phase string) error {
	partPath := destination + ".part"
	if err := validateManagedFilePath(cache.root, partPath); err != nil {
		return WrapError(CodeModelInstallFailed, err)
	}
	if info, err := os.Lstat(partPath); err == nil {
		if !info.Mode().IsRegular() {
			return WrapError(CodeModelInstallFailed, fmt.Errorf("partial artifact path is not a regular file"))
		}
		if info.Size() > artifact.Size {
			if err := os.Remove(partPath); err != nil {
				return WrapError(CodeModelInstallFailed, err)
			}
		} else if info.Size() == artifact.Size {
			if err := verifyArtifactFile(ctx, partPath, artifact, cache.progressFn); err == nil {
				if err := publishArtifact(ctx, partPath, destination); err != nil {
					return WrapError(CodeModelInstallFailed, err)
				}
				return nil
			} else if !errors.Is(err, errArtifactIntegrity) {
				return WrapError(CodeModelInstallFailed, err)
			}
			if err := os.Remove(partPath); err != nil {
				return WrapError(CodeModelInstallFailed, err)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return WrapError(CodeModelInstallFailed, err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		current, err := partialFileSize(partPath)
		if err != nil {
			return WrapError(CodeModelInstallFailed, err)
		}
		url := artifactURL(manifest, artifact)
		if cache.urlFor != nil {
			url = cache.urlFor(manifest, artifact)
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return WrapError(CodeModelDownloadFailed, err)
		}
		request.Header.Set("Accept-Encoding", "identity")
		request.Header.Set("User-Agent", "AuraGo-local-decisions")
		if current > 0 {
			request.Header.Set("Range", fmt.Sprintf("bytes=%d-", current))
		}
		response, err := cache.client.Do(request)
		if err != nil {
			if ctx.Err() != nil {
				return WrapError(CodeCancelled, ctx.Err())
			}
			return WrapError(CodeModelDownloadFailed, err)
		}
		if response.StatusCode == http.StatusRequestedRangeNotSatisfiable && current > 0 {
			_ = response.Body.Close()
			if err := os.Remove(partPath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return WrapError(CodeModelInstallFailed, err)
			}
			continue
		}
		if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
			_ = response.Body.Close()
			return NewError(CodeModelDownloadFailed, fmt.Errorf("artifact server returned HTTP %d", response.StatusCode))
		}
		if response.StatusCode == http.StatusPartialContent {
			start, rangeErr := contentRangeStart(response.Header.Get("Content-Range"))
			if rangeErr != nil || start != current {
				_ = response.Body.Close()
				return WrapError(CodeModelDownloadFailed, fmt.Errorf("artifact server returned an invalid byte range"))
			}
		} else {
			current = 0
		}
		if err := requireRuntimeDiskSpace(cache.root, artifact.Size-current); err != nil {
			_ = response.Body.Close()
			return err
		}
		flags := os.O_CREATE | os.O_WRONLY
		if current == 0 {
			flags |= os.O_TRUNC
		} else {
			flags |= os.O_APPEND
		}
		file, err := os.OpenFile(partPath, flags, 0o600)
		if err != nil {
			_ = response.Body.Close()
			return WrapError(CodeModelInstallFailed, err)
		}
		writer := &artifactProgressWriter{ctx: ctx, writer: file, current: current, total: artifact.Size, phase: phase, callback: cache.progressFn}
		copied, copyErr := io.CopyBuffer(writer, io.LimitReader(response.Body, artifact.Size-current+1), make([]byte, modelDownloadChunk))
		bodyCloseErr := response.Body.Close()
		if copyErr == nil {
			copyErr = bodyCloseErr
		}
		if syncErr := file.Sync(); copyErr == nil {
			copyErr = syncErr
		}
		if closeErr := file.Close(); copyErr == nil {
			copyErr = closeErr
		}
		if copyErr != nil {
			if ctx.Err() != nil {
				return WrapError(CodeCancelled, ctx.Err())
			}
			return WrapError(CodeModelDownloadFailed, copyErr)
		}
		if copied > artifact.Size-current {
			_ = os.Remove(partPath)
			return NewError(CodeModelIntegrityFailed, fmt.Errorf("artifact exceeded its pinned size"))
		}
		if current+copied < artifact.Size {
			return NewError(CodeModelDownloadFailed, io.ErrUnexpectedEOF)
		}
		if err := verifyArtifactFile(ctx, partPath, artifact, cache.progressFn); err != nil {
			if ctx.Err() != nil {
				return WrapError(CodeCancelled, ctx.Err())
			}
			_ = os.Remove(partPath)
			return WrapError(CodeModelIntegrityFailed, err)
		}
		if err := publishArtifact(ctx, partPath, destination); err != nil {
			return WrapError(CodeModelInstallFailed, err)
		}
		return nil
	}
	return NewError(CodeModelDownloadFailed, fmt.Errorf("artifact range request could not be resumed"))
}

type artifactProgressWriter struct {
	ctx      context.Context
	writer   io.Writer
	current  int64
	total    int64
	phase    string
	callback func(phase string, current, total int64)
}

func (writer *artifactProgressWriter) Write(data []byte) (int, error) {
	if writer.ctx != nil {
		if err := writer.ctx.Err(); err != nil {
			return 0, err
		}
	}
	written, err := writer.writer.Write(data)
	writer.current += int64(written)
	if writer.callback != nil {
		writer.callback(writer.phase, writer.current, writer.total)
	}
	return written, err
}

func partialFileSize(path string) (int64, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !info.Mode().IsRegular() {
		return 0, fmt.Errorf("partial artifact path is not a regular file")
	}
	return info.Size(), nil
}

func contentRangeStart(value string) (int64, error) {
	if !strings.HasPrefix(value, "bytes ") {
		return 0, fmt.Errorf("invalid content range")
	}
	byteRange := strings.Split(strings.TrimPrefix(value, "bytes "), "/")
	if len(byteRange) != 2 {
		return 0, fmt.Errorf("invalid content range")
	}
	startEnd := strings.Split(byteRange[0], "-")
	if len(startEnd) != 2 {
		return 0, fmt.Errorf("invalid content range")
	}
	return strconv.ParseInt(startEnd[0], 10, 64)
}

func publishArtifact(ctx context.Context, partPath, destination string) error {
	if info, err := os.Lstat(destination); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("managed artifact destination is not a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return fileutil.RenameContext(ctx, partPath, destination)
}

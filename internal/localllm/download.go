package localllm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type downloadProgress func(downloaded, total int64)
type downloadPublishGuard func(func() error) error

var availableDiskBytes = freeDiskBytes

// downloadArtifact resumes into a .part file and publishes only after size and SHA-256 verification.
func downloadArtifact(ctx context.Context, client *http.Client, rawURL, destination string, artifact Artifact, progress downloadProgress) error {
	return downloadArtifactGuarded(ctx, client, rawURL, destination, artifact, progress, nil)
}

func downloadArtifactGuarded(
	ctx context.Context,
	client *http.Client,
	rawURL, destination string,
	artifact Artifact,
	progress downloadProgress,
	publishGuard downloadPublishGuard,
) error {
	if client == nil {
		client = http.DefaultClient
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("create model directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("set model directory permissions: %w", err)
	}
	if ok, _ := verifyArtifact(destination, artifact); ok {
		return makeArtifactSidecarReadable(destination)
	}

	partPath := destination + ".part"
	var offset int64
	if info, err := os.Stat(partPath); err == nil {
		offset = info.Size()
		if offset == artifact.Size {
			ok, verifyErr := verifyArtifact(partPath, artifact)
			if verifyErr != nil {
				return verifyErr
			}
			if ok {
				return guardedArtifactPublish(ctx, publishGuard, partPath, destination)
			}
			if err := os.Remove(partPath); err != nil {
				return fmt.Errorf("remove corrupt complete partial model: %w", err)
			}
			offset = 0
		}
		if offset > artifact.Size {
			if err := os.Remove(partPath); err != nil {
				return fmt.Errorf("remove oversized partial model: %w", err)
			}
			offset = 0
		}
	}
	if free, err := availableDiskBytes(filepath.Dir(destination)); err == nil {
		required := artifact.Size - offset + (64 << 20)
		if free < required {
			return fmt.Errorf("insufficient_disk_space")
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("create model download: %w", err)
	}
	if offset > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download model: %w", err)
	}
	if offset > 0 && resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		resp.Body.Close()
		restartReq, restartErr := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if restartErr != nil {
			return fmt.Errorf("restart model download: %w", restartErr)
		}
		resp, err = client.Do(restartReq)
		if err != nil {
			return fmt.Errorf("restart model download: %w", err)
		}
		offset = 0
	}
	defer resp.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY
	switch {
	case offset > 0 && resp.StatusCode == http.StatusPartialContent:
		flags |= os.O_APPEND
	case resp.StatusCode == http.StatusOK:
		flags |= os.O_TRUNC
		offset = 0
	default:
		return fmt.Errorf("download model: unexpected HTTP status %d", resp.StatusCode)
	}
	file, err := os.OpenFile(partPath, flags, 0o600)
	if err != nil {
		return fmt.Errorf("open partial model: %w", err)
	}
	copied, copyErr := copyWithProgress(ctx, file, resp.Body, offset, artifact.Size, progress)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return fmt.Errorf("close partial model: %w", closeErr)
	}
	if offset+copied != artifact.Size {
		return fmt.Errorf("model_size_mismatch")
	}
	ok, verifyErr := verifyArtifact(partPath, artifact)
	if verifyErr != nil {
		return verifyErr
	}
	if !ok {
		_ = os.Remove(partPath)
		return fmt.Errorf("model_hash_mismatch")
	}
	return guardedArtifactPublish(ctx, publishGuard, partPath, destination)
}

func guardedArtifactPublish(ctx context.Context, guard downloadPublishGuard, partPath, destination string) error {
	publish := func() error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := publishVerifiedArtifact(partPath, destination); err != nil {
			return err
		}
		return makeArtifactSidecarReadable(destination)
	}
	if guard != nil {
		return guard(publish)
	}
	return publish()
}

func publishVerifiedArtifact(partPath, destination string) error {
	if _, err := os.Stat(destination); os.IsNotExist(err) {
		if err := os.Rename(partPath, destination); err != nil {
			return fmt.Errorf("publish verified model: %w", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect previous model: %w", err)
	}

	// Preserve the predecessor on every platform. Apart from being necessary on
	// Windows, this keeps the previous release recoverable after an explicitly
	// requested model update. v1 intentionally has no destructive cache cleanup.
	backup := destination + ".previous-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := os.Rename(destination, backup); err != nil {
		return fmt.Errorf("preserve previous model: %w", err)
	}
	if err := os.Rename(partPath, destination); err != nil {
		_ = os.Rename(backup, destination)
		return fmt.Errorf("publish verified model: %w", err)
	}
	return nil
}

func makeArtifactSidecarReadable(path string) error {
	if err := os.Chmod(path, 0o444); err != nil {
		return fmt.Errorf("set model permissions: %w", err)
	}
	return nil
}

func copyWithProgress(ctx context.Context, dst io.Writer, src io.Reader, offset, total int64, progress downloadProgress) (int64, error) {
	buf := make([]byte, 1<<20)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		n, readErr := src.Read(buf)
		if n > 0 {
			count, writeErr := dst.Write(buf[:n])
			written += int64(count)
			if progress != nil {
				progress(offset+written, total)
			}
			if writeErr != nil {
				return written, fmt.Errorf("write partial model: %w", writeErr)
			}
			if count != n {
				return written, io.ErrShortWrite
			}
		}
		if readErr == io.EOF {
			return written, nil
		}
		if readErr != nil {
			return written, fmt.Errorf("read model download: %w", readErr)
		}
	}
}

func verifyArtifact(path string, artifact Artifact) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return false, err
	}
	if info.Size() != artifact.Size {
		return false, nil
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false, err
	}
	return strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), artifact.SHA256), nil
}

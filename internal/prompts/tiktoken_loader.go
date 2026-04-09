package prompts

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tiktoken "github.com/pkoukk/tiktoken-go"
)

func init() {
	tiktoken.SetBpeLoader(newTimeoutBpeLoader(30 * time.Second))
}

// timeoutBpeLoader wraps tiktoken BPE file loading with an HTTP timeout
// to prevent indefinite hangs when the server cannot reach the download URL.
type timeoutBpeLoader struct {
	client *http.Client
}

func newTimeoutBpeLoader(timeout time.Duration) *timeoutBpeLoader {
	return &timeoutBpeLoader{
		client: &http.Client{Timeout: timeout},
	}
}

func (l *timeoutBpeLoader) LoadTiktokenBpe(tiktokenBpeFile string) (map[string]int, error) {
	contents, err := l.readFileCached(tiktokenBpeFile)
	if err != nil {
		return nil, err
	}

	bpeRanks := make(map[string]int)
	for _, line := range strings.Split(string(contents), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, " ")
		if len(parts) < 2 {
			continue
		}
		token, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			return nil, err
		}
		rank, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, err
		}
		bpeRanks[string(token)] = rank
	}
	return bpeRanks, nil
}

func (l *timeoutBpeLoader) readFileCached(blobpath string) ([]byte, error) {
	if blobpath == "" {
		return nil, fmt.Errorf("blobpath cannot be empty")
	}

	// Local file — read directly
	if !strings.HasPrefix(blobpath, "http://") && !strings.HasPrefix(blobpath, "https://") {
		return os.ReadFile(blobpath)
	}

	// Check cache first
	cacheDir := strings.TrimSpace(os.Getenv("TIKTOKEN_CACHE_DIR"))
	if cacheDir == "" {
		cacheDir = strings.TrimSpace(os.Getenv("DATA_GYM_CACHE_DIR"))
	}
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "data-gym-cache")
	}

	cacheKey := fmt.Sprintf("%x", sha1.Sum([]byte(blobpath)))
	cachePath := filepath.Join(cacheDir, cacheKey)

	if data, err := os.ReadFile(cachePath); err == nil {
		return data, nil
	}

	// Download with timeout (prevents indefinite hang on network issues)
	slog.Info("[Tiktoken] downloading BPE vocabulary", "url", blobpath, "timeout", l.client.Timeout)
	resp, err := l.client.Get(blobpath)
	if err != nil {
		return nil, fmt.Errorf("download tiktoken BPE (timeout %v): %w", l.client.Timeout, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download tiktoken BPE: HTTP %d", resp.StatusCode)
	}

	contents, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read tiktoken BPE response: %w", err)
	}

	// Cache for next time (best-effort)
	if err := os.MkdirAll(cacheDir, os.ModePerm); err == nil {
		_ = os.WriteFile(cachePath, contents, 0644)
	}

	slog.Info("[Tiktoken] BPE vocabulary cached", "path", cachePath, "bytes", len(contents))
	return contents, nil
}

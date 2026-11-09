package tools

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func homepageNonPublicArtifact(relative string) bool {
	// Credential paths retain their callers' explicit reject/filter policy.
	if hereNowSensitivePath(relative) {
		return false
	}
	for _, part := range strings.Split(strings.ToLower(filepath.ToSlash(relative)), "/") {
		if (strings.HasPrefix(part, ".") && part != ".well-known") || strings.HasSuffix(part, "~") {
			return true
		}
		for _, suffix := range []string{".bak", ".backup", ".orig", ".tmp"} {
			if strings.HasSuffix(part, suffix) || strings.Contains(part, suffix+".") || strings.Contains(part, suffix+"-") {
				return true
			}
		}
	}
	return false
}

func homepageLocalSnapshot(cfg HomepageConfig, projectDir, buildDir string) (*hereNowPublishSnapshot, string, error) {
	root, err := os.OpenRoot(cfg.WorkspacePath)
	if err != nil {
		return nil, "", err
	}
	defer root.Close()
	const relativeParent = ".aurago/published"
	if err := root.MkdirAll(relativeParent, 0700); err != nil {
		return nil, "", err
	}
	if err := validateHereNowDirectoryComponents(root, relativeParent); err != nil {
		return nil, "", err
	}
	parent := filepath.Join(cfg.WorkspacePath, filepath.FromSlash(relativeParent))
	snapshot, err := buildHomepageSnapshotIn(cfg.WorkspacePath, filepath.ToSlash(filepath.Join(projectDir, buildDir)), parent, netlifyZipMaxBytes)
	if err != nil {
		return nil, "", err
	}
	hash, err := homepageStaticIndexHash(snapshot.root)
	if err != nil {
		_ = snapshot.Close()
		return nil, "", err
	}
	return snapshot, hash, nil
}

func homepageSavedSnapshot(workspace string, state webserverState) (string, error) {
	rel := filepath.ToSlash(state.SnapshotDir)
	if !strings.HasPrefix(rel, ".aurago/published/") || strings.Contains(rel, "..") || state.IndexSHA256 == "" {
		return "", fmt.Errorf("saved publication snapshot is invalid; publish the project explicitly")
	}
	root, err := os.OpenRoot(workspace)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if err := validateHereNowDirectoryComponents(root, rel); err != nil {
		return "", err
	}
	full := filepath.Join(workspace, filepath.FromSlash(rel))
	hash, err := homepageStaticIndexHash(full)
	if err != nil {
		return "", err
	}
	if hash != state.IndexSHA256 {
		return "", fmt.Errorf("saved publication snapshot changed; publish the project explicitly")
	}
	return full, nil
}

func homepageVerifyLocalDocument(ctx context.Context, port int, expectedHash string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	client := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	var lastErr error
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/", port), nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			data, readErr := homepageReadDocument(resp.Body)
			_ = resp.Body.Close()
			if readErr == nil && resp.StatusCode == http.StatusOK && homepageDocumentHash(data) == expectedHash {
				return nil
			}
			err = fmt.Errorf("published root does not match the staged document (HTTP %d, read error %v)", resp.StatusCode, readErr)
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return fmt.Errorf("local publication verification failed: %w", lastErr)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

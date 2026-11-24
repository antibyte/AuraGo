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

const homepagePublicationsFolder = "published"

// The directory is a sibling of the resolved source workspace, which the dev
// container mounts read-write. Never reuse publication state found in that mount.
func homepagePublicationDir(workspace string) (string, error) {
	if strings.TrimSpace(workspace) == "" {
		return "", fmt.Errorf("homepage workspace is not configured")
	}
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(abs)
	if parent == abs {
		return "", fmt.Errorf("homepage workspace cannot be a filesystem root")
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	if info, err := os.Lstat(abs); err == nil {
		if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return "", fmt.Errorf("homepage workspace must be a real directory")
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	dir := filepath.Join(resolvedParent, filepath.Base(abs)+".aurago-publication")
	if info, err := os.Lstat(dir); err == nil {
		if !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return "", fmt.Errorf("homepage publication directory must be a real directory")
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return dir, nil
}

func ensureHomepagePublicationDir(workspace string) (string, error) {
	dir, err := homepagePublicationDir(workspace)
	if err != nil {
		return "", err
	}
	if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
		return "", err
	}
	return homepagePublicationDir(workspace)
}

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
	dir, err := ensureHomepagePublicationDir(cfg.WorkspacePath)
	if err != nil {
		return nil, "", err
	}
	return homepageStaticSnapshot(cfg, projectDir, buildDir, dir, homepagePublicationsFolder)
}

func homepageStaticSnapshot(cfg HomepageConfig, projectDir, buildDir, parent, relativeParent string) (*hereNowPublishSnapshot, string, error) {
	root, err := os.OpenRoot(parent)
	if err != nil {
		return nil, "", err
	}
	defer root.Close()
	if err := root.MkdirAll(relativeParent, 0700); err != nil {
		return nil, "", err
	}
	if err := validateHereNowDirectoryComponents(root, relativeParent); err != nil {
		return nil, "", err
	}
	snapshot, err := buildHomepageSnapshotIn(cfg.WorkspacePath, filepath.ToSlash(filepath.Join(projectDir, buildDir)), filepath.Join(parent, filepath.FromSlash(relativeParent)), netlifyZipMaxBytes)
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

func homepageSavedSnapshot(publicationDir string, state webserverState) (string, error) {
	rel := filepath.ToSlash(state.SnapshotDir)
	if !strings.HasPrefix(rel, homepagePublicationsFolder+"/") || strings.Contains(rel, "..") || state.IndexSHA256 == "" {
		return "", fmt.Errorf("saved publication snapshot is invalid; publish the project explicitly")
	}
	root, err := os.OpenRoot(publicationDir)
	if err != nil {
		return "", err
	}
	defer root.Close()
	if err := validateHereNowDirectoryComponents(root, rel); err != nil {
		return "", err
	}
	full := filepath.Join(publicationDir, filepath.FromSlash(rel))
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

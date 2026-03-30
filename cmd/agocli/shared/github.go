package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	// GitHubRepo is the repository path on GitHub.
	GitHubRepo = "antibyte/AuraGo"
	// GitHubAPIBase is the base URL for GitHub API.
	GitHubAPIBase = "https://api.github.com"
)

// ReleaseInfo contains information about a GitHub release.
type ReleaseInfo struct {
	TagName string      `json:"tag_name"`
	Name    string      `json:"name"`
	Assets  []AssetInfo `json:"assets"`
}

// AssetInfo contains information about a release asset.
type AssetInfo struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

var ghClient = &http.Client{Timeout: 30 * time.Second}

// GetLatestRelease fetches the latest release info from GitHub.
func GetLatestRelease() (*ReleaseInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/releases/latest", GitHubAPIBase, GitHubRepo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "agocli")

	resp, err := ghClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode GitHub response: %w", err)
	}
	return &release, nil
}

// GetLatestReleaseTag returns just the tag name of the latest release.
func GetLatestReleaseTag() (string, error) {
	rel, err := GetLatestRelease()
	if err != nil {
		return "", err
	}
	return rel.TagName, nil
}

// FindAsset finds a release asset by name.
func (r *ReleaseInfo) FindAsset(name string) *AssetInfo {
	for i := range r.Assets {
		if r.Assets[i].Name == name {
			return &r.Assets[i]
		}
	}
	return nil
}

// DownloadFile downloads a URL to a local file path.
// If onProgress is non-nil, it is called with bytes written and total size.
func DownloadFile(url, destPath string, onProgress func(written, total int64)) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0750); err != nil {
		return err
	}

	resp, err := ghClient.Get(url)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	if onProgress != nil {
		reader := &progressReader{
			reader:     resp.Body,
			total:      resp.ContentLength,
			onProgress: onProgress,
		}
		_, err = io.Copy(out, reader)
	} else {
		_, err = io.Copy(out, resp.Body)
	}
	return err
}

type progressReader struct {
	reader     io.Reader
	total      int64
	written    int64
	onProgress func(written, total int64)
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.written += int64(n)
	pr.onProgress(pr.written, pr.total)
	return n, err
}

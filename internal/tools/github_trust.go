package tools

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"aurago/internal/fileutil"
	"aurago/internal/security"
)

type githubTrustLedger struct {
	Version int                `json:"version"`
	Grants  []githubTrustGrant `json:"grants"`
}
type githubTrustGrant struct {
	APIBase    string `json:"api_base"`
	Repository string `json:"repository"`
}

var githubTrustMu sync.Mutex
var githubIdentityPart = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)

func githubTrustBase(raw string) (string, error) {
	if raw == "" {
		raw = "https://api.github.com"
	}
	raw = strings.TrimRight(raw, "/")
	if err := security.ValidateHTTPBaseURL(raw); err != nil {
		return "", err
	}
	u, _ := url.Parse(raw)
	u.Host = strings.ToLower(u.Host)
	return u.String(), nil
}

func githubTrustIdentity(fullName string) (string, error) {
	parts := strings.Split(fullName, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("GitHub trust requires owner/repository")
	}
	for _, part := range parts {
		if !githubIdentityPart.MatchString(part) || part == "." || part == ".." {
			return "", fmt.Errorf("invalid GitHub repository identity")
		}
	}
	return strings.ToLower(fullName), nil
}

func readGitHubTrust(dataDir string) (githubTrustLedger, []byte, error) {
	ledger := githubTrustLedger{Version: 1}
	if strings.TrimSpace(dataDir) == "" {
		return ledger, nil, fmt.Errorf("GitHub trust data directory is not configured")
	}
	file, err := os.Open(filepath.Join(dataDir, "github_trust.json"))
	if os.IsNotExist(err) {
		return ledger, nil, nil
	}
	if err != nil {
		return ledger, nil, err
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, 1024*1024+1))
	if err != nil {
		return ledger, nil, err
	}
	if len(raw) > 1024*1024 {
		return ledger, nil, fmt.Errorf("GitHub trust ledger exceeds size limit")
	}
	if err := json.Unmarshal(raw, &ledger); err != nil {
		return ledger, nil, fmt.Errorf("read GitHub trust ledger: %w", err)
	}
	if ledger.Version != 1 {
		return ledger, nil, fmt.Errorf("unsupported GitHub trust ledger version")
	}
	return ledger, raw, nil
}

// GitHubTrustedProjectRepos reads only protected server grants. Workspace
// inventory, including AgentCreated, is never a trust source.
func GitHubTrustedProjectRepos(dataDir string, baseURL ...string) []string {
	rawBase := ""
	if len(baseURL) > 0 {
		rawBase = baseURL[0]
	}
	base, err := githubTrustBase(rawBase)
	if err != nil {
		return nil
	}
	githubTrustMu.Lock()
	defer githubTrustMu.Unlock()
	ledger, _, err := readGitHubTrust(dataDir)
	if err != nil {
		return nil
	}
	var repos []string
	for _, grant := range ledger.Grants {
		id, err := githubTrustIdentity(grant.Repository)
		if err == nil && grant.APIBase == base {
			repos = append(repos, id)
		}
	}
	return repos
}

func setGitHubCreatedTrust(cfg GitHubConfig, fullName string, trusted bool) error {
	id, err := githubTrustIdentity(fullName)
	if err != nil {
		return err
	}
	base, err := githubTrustBase(cfg.BaseURL)
	if err != nil {
		return err
	}
	githubTrustMu.Lock()
	defer githubTrustMu.Unlock()
	ledger, previous, err := readGitHubTrust(cfg.DataDir)
	if err != nil {
		return err
	}
	grants := make([]githubTrustGrant, 0, len(ledger.Grants)+1)
	for _, grant := range ledger.Grants {
		if grant.APIBase != base || grant.Repository != id {
			grants = append(grants, grant)
		}
	}
	if trusted {
		grants = append(grants, githubTrustGrant{APIBase: base, Repository: id})
	}
	ledger.Grants = grants
	raw, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return err
	}
	target := filepath.Join(cfg.DataDir, "github_trust.json")
	if len(previous) > 0 {
		if err := writeGitHubTrustFile(target+".bak", previous); err != nil {
			return fmt.Errorf("backup GitHub trust: %w", err)
		}
	}
	return writeGitHubTrustFile(target, raw)
}

func writeGitHubTrustFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".github-trust-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	defer file.Close()
	if err := file.Chmod(0600); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return fileutil.Rename(name, path)
}

package tools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/security"
)

// GitHubConfig holds the GitHub API connection parameters.
type GitHubConfig struct {
	Token                 string // Personal Access Token (ghp_…)
	Owner                 string // GitHub username or organisation
	BaseURL               string // API base URL (default: https://api.github.com)
	DefaultPrivate        bool   // true = new repos are private by default
	ReadOnly              bool   // true = block mutating operations
	AllowDelete           bool   // explicit administrator grant; defaults off
	DataDir               string // protected server trust ledger directory
	AllowedRepos          []string
	TrustedRepos          []string // canonical owner/repo entries created by AuraGo
	ListReposUnrestricted bool     // config UI only: list all token-visible repos
	WorkspaceDir          string   // optional: track repos created by the agent
}

var githubHTTPClient = security.NewSSRFProtectedHTTPClientSameOrigin(30 * time.Second)

func githubRepoEndpoint(owner, repo string, extra ...string) string {
	parts := []string{"", "repos", url.PathEscape(strings.TrimSpace(owner)), url.PathEscape(strings.TrimSpace(repo))}
	for _, part := range extra {
		for _, segment := range strings.Split(strings.Trim(part, "/"), "/") {
			if segment == "" {
				continue
			}
			parts = append(parts, url.PathEscape(segment))
		}
	}
	return strings.Join(parts, "/")
}

func githubContentEndpoint(owner, repo, filePath string) string {
	segments := []string{"contents"}
	for _, segment := range strings.Split(strings.Trim(filePath, "/"), "/") {
		if segment == "" {
			continue
		}
		segments = append(segments, segment)
	}
	return githubRepoEndpoint(owner, repo, segments...)
}

// ── Internal helpers ────────────────────────────────────────────────────────

// githubRequest executes an authenticated HTTP request against the GitHub API.
func githubRequest(ctx context.Context, cfg GitHubConfig, method, endpoint string, body interface{}) ([]byte, http.Header, int, error) {
	if ctx == nil {
		return nil, nil, 0, fmt.Errorf("github request context is required")
	}
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.github.com"
	}
	endpointURL := strings.TrimRight(base, "/") + endpoint

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, method, endpointURL, reqBody)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, resp.Header, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return respBody, resp.Header, resp.StatusCode, nil
}

// githubOwner returns the effective owner (user/org) for API calls.
func githubOwner(cfg GitHubConfig, owner string) string {
	if owner != "" {
		return strings.TrimSpace(owner)
	}
	return strings.TrimSpace(cfg.Owner)
}

func githubReadOnlyError(cfg GitHubConfig) string {
	if !cfg.ReadOnly {
		return ""
	}
	return errJSON("GitHub is in read-only mode. Disable github.readonly to allow changes.")
}

func githubRepoAccessError(cfg GitHubConfig, owner, repo string) string {
	if repo == "" {
		return ""
	}
	effectiveOwner := githubOwner(cfg, owner)
	if GitHubRepoAllowed(cfg, effectiveOwner, repo) {
		return ""
	}
	fullName := GitHubCanonicalRepo(effectiveOwner, repo)
	if fullName == "" {
		fullName = strings.TrimSpace(repo)
	}
	return errJSON("Repo '%s' is not in the allowed repos list. Add it in Settings → GitHub to grant access.", fullName)
}

// GitHubCanonicalRepo returns the lower-case owner/repo key used for policy checks.
func GitHubCanonicalRepo(owner, repo string) string {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return ""
	}
	return strings.ToLower(owner + "/" + repo)
}

// GitHubSplitFullName splits owner/repo references and trims whitespace.
func GitHubSplitFullName(fullName string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(fullName), "/", 2)
	if len(parts) != 2 {
		return "", strings.TrimSpace(fullName)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

// GitHubRepoAllowed checks explicit allowlist entries and trusted agent-created repos.
func GitHubRepoAllowed(cfg GitHubConfig, owner, repo string) bool {
	owner = githubOwner(cfg, owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return false
	}
	return githubRepoAllowedByList(cfg.AllowedRepos, cfg.Owner, owner, repo, true) ||
		githubRepoAllowedByList(cfg.TrustedRepos, cfg.Owner, owner, repo, false)
}

func githubRepoExplicitlyAllowed(cfg GitHubConfig, owner, repo string) bool {
	owner = githubOwner(cfg, owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return false
	}
	return githubRepoAllowedByList(cfg.AllowedRepos, cfg.Owner, owner, repo, true)
}

func githubRepoAllowedByList(entries []string, cfgOwner, owner, repo string, allowLegacyBare bool) bool {
	canonical := GitHubCanonicalRepo(owner, repo)
	cfgOwner = strings.TrimSpace(cfgOwner)
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		entryOwner, entryRepo := GitHubSplitFullName(entry)
		if entryOwner != "" {
			if GitHubCanonicalRepo(entryOwner, entryRepo) == canonical {
				return true
			}
			continue
		}
		if allowLegacyBare && strings.EqualFold(owner, cfgOwner) && strings.EqualFold(entryRepo, repo) {
			return true
		}
	}
	return false
}

func githubStringValue(value interface{}) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%v", value)
}

const githubMaxListPages = 10

func githubNextLink(base string, header http.Header) (string, bool) {
	if strings.TrimSpace(base) == "" {
		base = "https://api.github.com"
	}
	baseURL, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || baseURL.Host == "" {
		return "", false
	}
	for _, part := range strings.Split(header.Get("Link"), ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.IndexByte(part, '<')
		end := strings.IndexByte(part, '>')
		if start < 0 || end <= start {
			continue
		}
		next, err := url.Parse(part[start+1 : end])
		if err != nil || !security.SameHTTPOrigin(baseURL, next) {
			continue
		}
		return next.RequestURI(), true
	}
	return "", false
}

func githubCreateRepoEndpoint(ctx context.Context, cfg GitHubConfig) (string, error) {
	owner := strings.TrimSpace(cfg.Owner)
	if owner == "" {
		return "/user/repos", nil
	}
	if !githubIdentityPart.MatchString(owner) || owner == "." || owner == ".." {
		return "", fmt.Errorf("invalid GitHub owner")
	}
	data, _, status, err := githubRequest(ctx, cfg, http.MethodGet, "/user", nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("GitHub user lookup returned HTTP %d", status)
	}
	var user struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(data, &user); err != nil {
		return "", fmt.Errorf("parse GitHub user: %w", err)
	}
	if strings.EqualFold(user.Login, owner) {
		return "/user/repos", nil
	}
	return "/orgs/" + url.PathEscape(owner) + "/repos", nil
}

// ── Repository operations ───────────────────────────────────────────────────

// GitHubListRepos lists repositories for the authenticated user or a specific owner.
func GitHubListRepos(ctx context.Context, cfg GitHubConfig, owner string, limit int) string {
	endpoint := "/user/repos?per_page=100&sort=updated"
	if owner != "" && owner != cfg.Owner {
		endpoint = fmt.Sprintf("/users/%s/repos?per_page=100&sort=updated", url.PathEscape(strings.TrimSpace(owner)))
	}

	var repos []map[string]interface{}
	for page := 0; page < githubMaxListPages; page++ {
		data, header, status, err := githubRequest(ctx, cfg, "GET", endpoint, nil)
		if err != nil {
			return errJSON("Failed to list repos: %v", err)
		}
		if status != 200 {
			return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
		}
		var pageRepos []map[string]interface{}
		if err := json.Unmarshal(data, &pageRepos); err != nil {
			return errJSON("Failed to parse repos: %v", err)
		}
		repos = append(repos, pageRepos...)
		next, ok := githubNextLink(cfg.BaseURL, header)
		if !ok {
			break
		}
		endpoint = next
	}

	// Return compact summary
	type repoSummary struct {
		Name        string `json:"name"`
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		Private     bool   `json:"private"`
		Language    string `json:"language"`
		UpdatedAt   string `json:"updated_at"`
		HTMLURL     string `json:"html_url"`
		CloneURL    string `json:"clone_url"`
	}

	var summaries []repoSummary
	for _, r := range repos {
		s := repoSummary{
			Name:     githubStringValue(r["name"]),
			FullName: githubStringValue(r["full_name"]),
			Private:  r["private"] == true,
			HTMLURL:  githubStringValue(r["html_url"]),
			CloneURL: githubStringValue(r["clone_url"]),
		}
		if r["description"] != nil {
			s.Description = githubStringValue(r["description"])
		}
		if r["language"] != nil {
			s.Language = githubStringValue(r["language"])
		}
		if r["updated_at"] != nil {
			s.UpdatedAt = githubStringValue(r["updated_at"])
		}
		if !cfg.ListReposUnrestricted {
			repoOwner, repoName := GitHubSplitFullName(s.FullName)
			if repoName == "" {
				repoName = s.Name
			}
			if repoOwner == "" {
				repoOwner = githubOwner(cfg, owner)
			}
			if !GitHubRepoAllowed(cfg, repoOwner, repoName) {
				continue
			}
		}
		summaries = append(summaries, s)
	}
	if limit > 0 && len(summaries) > limit {
		summaries = summaries[:limit]
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"count":  len(summaries),
		"repos":  summaries,
	})
	return string(out)
}

// GitHubCreateRepo creates a new repository.
func GitHubCreateRepo(ctx context.Context, cfg GitHubConfig, name, description string, private *bool) string {
	if msg := githubReadOnlyError(cfg); msg != "" {
		return msg
	}
	if name == "" {
		return errJSON("Repository name is required")
	}

	isPrivate := cfg.DefaultPrivate
	if private != nil {
		isPrivate = *private
	}

	body := map[string]interface{}{
		"name":        name,
		"description": description,
		"private":     isPrivate,
		"auto_init":   true,
	}

	endpoint, err := githubCreateRepoEndpoint(ctx, cfg)
	if err != nil {
		return errJSON("Failed to create repo: %v", err)
	}
	data, _, status, err := githubRequest(ctx, cfg, "POST", endpoint, body)
	if err != nil {
		return errJSON("Failed to create repo: %v", err)
	}
	if status != 201 {
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	var repo map[string]interface{}
	if err := json.Unmarshal(data, &repo); err != nil {
		return errJSON("Failed to parse response: %v", err)
	}

	response := map[string]interface{}{
		"status":    "ok",
		"message":   fmt.Sprintf("Repository '%s' created successfully", name),
		"name":      repo["name"],
		"full_name": repo["full_name"],
		"private":   repo["private"],
		"html_url":  repo["html_url"],
		"clone_url": repo["clone_url"],
	}
	fullName := githubStringValue(repo["full_name"])
	if err := setGitHubCreatedTrust(cfg, fullName, true); err != nil {
		response["trust_migration_required"] = true
		response["trust_warning"] = "Repository created; protected trust registration failed. Administrator approval in github.allowed_repos is required."
	} else {
		response["agent_created"] = true
	}
	if cfg.WorkspaceDir != "" {
		repoName := githubStringValue(repo["name"])
		if repoName == "" {
			repoName = name
		}
		fullName := githubStringValue(repo["full_name"])
		repoOwner, splitName := GitHubSplitFullName(fullName)
		if splitName != "" {
			repoName = splitName
		}
		if repoOwner == "" {
			repoOwner = cfg.Owner
		}
		purpose := description
		if strings.TrimSpace(purpose) == "" {
			purpose = "Created by AuraGo GitHub tool"
		}
		trackRaw := GitHubTrackCreatedProject(
			cfg.WorkspaceDir,
			repoName,
			purpose,
			githubStringValue(repo["html_url"]),
			githubStringValue(repo["clone_url"]),
			repoOwner,
			repo["private"] == true,
			fullName,
		)
		var trackResult map[string]interface{}
		if err := json.Unmarshal([]byte(trackRaw), &trackResult); err != nil || trackResult["status"] != "ok" {
			if err != nil {
				response["tracking_warning"] = fmt.Sprintf("repo created, but project tracking response could not be parsed: %v", err)
			} else if msg, _ := trackResult["message"].(string); msg != "" {
				response["tracking_warning"] = "repo created, but project tracking failed: " + msg
			} else {
				response["tracking_warning"] = "repo created, but project tracking failed"
			}
		} else {
			response["tracked"] = true
		}
	}
	out, _ := json.Marshal(response)
	return string(out)
}

// GitHubDeleteRepo deletes a repository.
func GitHubDeleteRepo(ctx context.Context, cfg GitHubConfig, owner, repo string) string {
	if msg := githubReadOnlyError(cfg); msg != "" {
		return msg
	}
	if !cfg.AllowDelete {
		return errJSON("GitHub repository deletion requires github.allow_delete=true")
	}
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" {
		return errJSON("Owner and repo name are required")
	}
	if msg := githubRepoAccessError(cfg, o, repo); msg != "" {
		return msg
	}

	canonical := GitHubCanonicalRepo(o, repo)
	hadGrant := false
	if cfg.DataDir != "" {
		had, err := githubHasTrustGrant(cfg, canonical)
		if err != nil {
			return errJSON("Cannot read repository trust before deletion: %v", err)
		}
		hadGrant = had
		if hadGrant {
			if err := setGitHubCreatedTrust(cfg, canonical, false); err != nil {
				return errJSON("Cannot revoke repository trust before deletion: %v", err)
			}
		}
	}
	data, _, status, err := githubRequest(ctx, cfg, http.MethodDelete, githubRepoEndpoint(o, repo), nil)
	if err != nil {
		if hadGrant {
			return errJSON("Failed to delete repo: %v; trust_revoked", err)
		}
		return errJSON("Failed to delete repo: %v", err)
	}
	if status != http.StatusNoContent {
		if hadGrant {
			if restoreErr := setGitHubCreatedTrust(cfg, canonical, true); restoreErr != nil {
				return errJSON("GitHub API error (HTTP %d): %s; trust grant could not be restored: %v", status, string(data), restoreErr)
			}
		}
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("Repository '%s/%s' deleted", o, repo),
	})
	return string(out)
}

// GitHubGetRepo gets detailed information about a repository.
func GitHubGetRepo(ctx context.Context, cfg GitHubConfig, owner, repo string) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" {
		return errJSON("Owner and repo name are required")
	}
	if msg := githubRepoAccessError(cfg, o, repo); msg != "" {
		return msg
	}

	data, _, status, err := githubRequest(ctx, cfg, "GET", githubRepoEndpoint(o, repo), nil)
	if err != nil {
		return errJSON("Failed to get repo: %v", err)
	}
	if status != 200 {
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return errJSON("Failed to parse response: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":         "ok",
		"name":           result["name"],
		"full_name":      result["full_name"],
		"description":    result["description"],
		"private":        result["private"],
		"language":       result["language"],
		"default_branch": result["default_branch"],
		"html_url":       result["html_url"],
		"clone_url":      result["clone_url"],
		"created_at":     result["created_at"],
		"updated_at":     result["updated_at"],
		"open_issues":    result["open_issues_count"],
		"stars":          result["stargazers_count"],
		"forks":          result["forks_count"],
	})
	return string(out)
}

// ── Issue operations ────────────────────────────────────────────────────────

// GitHubListIssues lists issues for a repository.
func GitHubListIssues(ctx context.Context, cfg GitHubConfig, owner, repo, state string) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" {
		return errJSON("Owner and repo name are required")
	}
	if msg := githubRepoAccessError(cfg, o, repo); msg != "" {
		return msg
	}
	if state == "" {
		state = "open"
	}

	issueQuery := url.Values{"state": []string{state}, "per_page": []string{"50"}}
	data, _, status, err := githubRequest(ctx, cfg, "GET", githubRepoEndpoint(o, repo, "issues")+"?"+issueQuery.Encode(), nil)
	if err != nil {
		return errJSON("Failed to list issues: %v", err)
	}
	if status != 200 {
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	var issues []map[string]interface{}
	if err := json.Unmarshal(data, &issues); err != nil {
		return errJSON("Failed to parse issues: %v", err)
	}

	type issueSummary struct {
		Number    interface{} `json:"number"`
		Title     string      `json:"title"`
		State     string      `json:"state"`
		User      string      `json:"user"`
		Labels    []string    `json:"labels"`
		CreatedAt string      `json:"created_at"`
		HTMLURL   string      `json:"html_url"`
	}

	var summaries []issueSummary
	for _, i := range issues {
		// Skip pull requests (they also appear in /issues)
		if i["pull_request"] != nil {
			continue
		}
		s := issueSummary{
			Number:    i["number"],
			Title:     githubStringValue(i["title"]),
			State:     githubStringValue(i["state"]),
			HTMLURL:   githubStringValue(i["html_url"]),
			CreatedAt: githubStringValue(i["created_at"]),
		}
		if user, ok := i["user"].(map[string]interface{}); ok {
			s.User = githubStringValue(user["login"])
		}
		if labels, ok := i["labels"].([]interface{}); ok {
			for _, l := range labels {
				if lm, ok := l.(map[string]interface{}); ok {
					s.Labels = append(s.Labels, githubStringValue(lm["name"]))
				}
			}
		}
		summaries = append(summaries, s)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"count":  len(summaries),
		"issues": summaries,
	})
	return string(out)
}

// GitHubCreateIssue creates a new issue in a repository.
func GitHubCreateIssue(ctx context.Context, cfg GitHubConfig, owner, repo, title, body string, labels []string) string {
	if msg := githubReadOnlyError(cfg); msg != "" {
		return msg
	}
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" || title == "" {
		return errJSON("Owner, repo name, and title are required")
	}
	if msg := githubRepoAccessError(cfg, o, repo); msg != "" {
		return msg
	}

	payload := map[string]interface{}{
		"title": title,
		"body":  body,
	}
	if len(labels) > 0 {
		payload["labels"] = labels
	}

	data, _, status, err := githubRequest(ctx, cfg, "POST", githubRepoEndpoint(o, repo, "issues"), payload)
	if err != nil {
		return errJSON("Failed to create issue: %v", err)
	}
	if status != 201 {
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return errJSON("Failed to parse response: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":   "ok",
		"message":  "Issue created",
		"number":   result["number"],
		"title":    result["title"],
		"html_url": result["html_url"],
	})
	return string(out)
}

// GitHubCloseIssue closes an issue.
func GitHubCloseIssue(ctx context.Context, cfg GitHubConfig, owner, repo string, number int) string {
	if msg := githubReadOnlyError(cfg); msg != "" {
		return msg
	}
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" || number <= 0 {
		return errJSON("Owner, repo name, and issue number are required")
	}
	if msg := githubRepoAccessError(cfg, o, repo); msg != "" {
		return msg
	}

	data, _, status, err := githubRequest(ctx, cfg, "PATCH",
		githubRepoEndpoint(o, repo, fmt.Sprintf("issues/%d", number)),
		map[string]interface{}{"state": "closed"})
	if err != nil {
		return errJSON("Failed to close issue: %v", err)
	}
	if status != 200 {
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("Issue #%d closed", number),
	})
	return string(out)
}

// ── Pull Request operations ─────────────────────────────────────────────────

// GitHubListPullRequests lists pull requests for a repository.
func GitHubListPullRequests(ctx context.Context, cfg GitHubConfig, owner, repo, state string) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" {
		return errJSON("Owner and repo name are required")
	}
	if msg := githubRepoAccessError(cfg, o, repo); msg != "" {
		return msg
	}
	if state == "" {
		state = "open"
	}

	prQuery := url.Values{"state": []string{state}, "per_page": []string{"50"}}
	data, _, status, err := githubRequest(ctx, cfg, "GET", githubRepoEndpoint(o, repo, "pulls")+"?"+prQuery.Encode(), nil)
	if err != nil {
		return errJSON("Failed to list PRs: %v", err)
	}
	if status != 200 {
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	var prs []map[string]interface{}
	if err := json.Unmarshal(data, &prs); err != nil {
		return errJSON("Failed to parse PRs: %v", err)
	}

	type prSummary struct {
		Number    interface{} `json:"number"`
		Title     string      `json:"title"`
		State     string      `json:"state"`
		User      string      `json:"user"`
		Head      string      `json:"head"`
		Base      string      `json:"base"`
		CreatedAt string      `json:"created_at"`
		HTMLURL   string      `json:"html_url"`
	}

	var summaries []prSummary
	for _, p := range prs {
		s := prSummary{
			Number:    p["number"],
			Title:     githubStringValue(p["title"]),
			State:     githubStringValue(p["state"]),
			HTMLURL:   githubStringValue(p["html_url"]),
			CreatedAt: githubStringValue(p["created_at"]),
		}
		if user, ok := p["user"].(map[string]interface{}); ok {
			s.User = githubStringValue(user["login"])
		}
		if head, ok := p["head"].(map[string]interface{}); ok {
			s.Head = githubStringValue(head["ref"])
		}
		if base, ok := p["base"].(map[string]interface{}); ok {
			s.Base = githubStringValue(base["ref"])
		}
		summaries = append(summaries, s)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":        "ok",
		"count":         len(summaries),
		"pull_requests": summaries,
	})
	return string(out)
}

// ── Branch operations ───────────────────────────────────────────────────────

// GitHubListBranches lists branches for a repository.
func GitHubListBranches(ctx context.Context, cfg GitHubConfig, owner, repo string) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" {
		return errJSON("Owner and repo name are required")
	}
	if msg := githubRepoAccessError(cfg, o, repo); msg != "" {
		return msg
	}

	branchQuery := url.Values{"per_page": []string{"100"}}
	data, _, status, err := githubRequest(ctx, cfg, "GET", githubRepoEndpoint(o, repo, "branches")+"?"+branchQuery.Encode(), nil)
	if err != nil {
		return errJSON("Failed to list branches: %v", err)
	}
	if status != 200 {
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	var branches []map[string]interface{}
	if err := json.Unmarshal(data, &branches); err != nil {
		return errJSON("Failed to parse branches: %v", err)
	}

	var names []string
	for _, b := range branches {
		names = append(names, githubStringValue(b["name"]))
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":   "ok",
		"count":    len(names),
		"branches": names,
	})
	return string(out)
}

// ── File operations ─────────────────────────────────────────────────────────

const githubFileTextLimit = 256 * 1024

func githubDecodeFileContent(encoding, content string) (string, string, bool) {
	if !strings.EqualFold(encoding, "base64") {
		return content, encoding, false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, content))
	if err != nil {
		return content, encoding, false
	}
	if !utf8.Valid(raw) || bytes.Contains(raw, []byte{0}) {
		return base64.StdEncoding.EncodeToString(raw), "base64", false
	}
	if len(raw) > githubFileTextLimit {
		cut := raw[:githubFileTextLimit]
		for !utf8.Valid(cut) && len(cut) > 0 {
			cut = cut[:len(cut)-1]
		}
		return string(cut), "utf-8", true
	}
	return string(raw), "utf-8", false
}

func githubFileContentBase64(content string) string {
	stripped := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, content)
	raw, err := base64.StdEncoding.DecodeString(stripped)
	if err == nil && base64.StdEncoding.EncodeToString(raw) == stripped {
		return stripped
	}
	return base64.StdEncoding.EncodeToString([]byte(content))
}

// GitHubGetFileContent reads a file from a repository.
func GitHubGetFileContent(ctx context.Context, cfg GitHubConfig, owner, repo, path, branch string) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" || path == "" {
		return errJSON("Owner, repo, and file path are required")
	}
	if msg := githubRepoAccessError(cfg, o, repo); msg != "" {
		return msg
	}

	endpoint := githubContentEndpoint(o, repo, path)
	if branch != "" {
		endpoint += "?" + url.Values{"ref": []string{branch}}.Encode()
	}

	data, _, status, err := githubRequest(ctx, cfg, "GET", endpoint, nil)
	if err != nil {
		return errJSON("Failed to get file: %v", err)
	}
	if status != 200 {
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		var entries []map[string]interface{}
		if dirErr := json.Unmarshal(data, &entries); dirErr != nil {
			return errJSON("Failed to parse response: %v", err)
		}
		listed := make([]map[string]interface{}, 0, len(entries))
		for _, entry := range entries {
			listed = append(listed, map[string]interface{}{
				"name": entry["name"],
				"path": entry["path"],
				"type": entry["type"],
				"size": entry["size"],
			})
		}
		out, _ := json.Marshal(map[string]interface{}{
			"status":  "ok",
			"type":    "dir",
			"entries": listed,
		})
		return string(out)
	}

	text, encoding, truncated := githubDecodeFileContent(githubStringValue(result["encoding"]), githubStringValue(result["content"]))
	response := map[string]interface{}{
		"status":   "ok",
		"name":     result["name"],
		"path":     result["path"],
		"size":     result["size"],
		"sha":      result["sha"],
		"content":  text,
		"encoding": encoding,
		"html_url": result["html_url"],
	}
	if truncated {
		response["truncated"] = true
	}
	out, _ := json.Marshal(response)
	return string(out)
}

// GitHubCreateOrUpdateFile creates or updates a file in a repository.
func GitHubCreateOrUpdateFile(ctx context.Context, cfg GitHubConfig, owner, repo, path, content, message, sha, branch string) string {
	if msg := githubReadOnlyError(cfg); msg != "" {
		return msg
	}
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" || path == "" {
		return errJSON("Owner, repo, and file path are required")
	}
	if msg := githubRepoAccessError(cfg, o, repo); msg != "" {
		return msg
	}
	if message == "" {
		message = "Update " + path
	}

	payload := map[string]interface{}{
		"message": message,
		"content": githubFileContentBase64(content),
	}
	if sha != "" {
		payload["sha"] = sha // required for updates
	}
	if branch != "" {
		payload["branch"] = branch
	}

	data, _, status, err := githubRequest(ctx, cfg, "PUT", githubContentEndpoint(o, repo, path), payload)
	if err != nil {
		return errJSON("Failed to create/update file: %v", err)
	}
	if status != 200 && status != 201 {
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("File '%s' saved in %s/%s", path, o, repo),
	})
	return string(out)
}

// ── Commit & Actions operations ─────────────────────────────────────────────

// GitHubListCommits lists recent commits for a repository.
func GitHubListCommits(ctx context.Context, cfg GitHubConfig, owner, repo, branch string, limit int) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" {
		return errJSON("Owner and repo name are required")
	}
	if msg := githubRepoAccessError(cfg, o, repo); msg != "" {
		return msg
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	query := url.Values{"per_page": []string{fmt.Sprintf("%d", limit)}}
	if branch != "" {
		query.Set("sha", branch)
	}
	endpoint := githubRepoEndpoint(o, repo, "commits") + "?" + query.Encode()

	data, _, status, err := githubRequest(ctx, cfg, "GET", endpoint, nil)
	if err != nil {
		return errJSON("Failed to list commits: %v", err)
	}
	if status != 200 {
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	var commits []map[string]interface{}
	if err := json.Unmarshal(data, &commits); err != nil {
		return errJSON("Failed to parse commits: %v", err)
	}

	type commitSummary struct {
		SHA     string `json:"sha"`
		Message string `json:"message"`
		Author  string `json:"author"`
		Date    string `json:"date"`
	}

	var summaries []commitSummary
	for _, c := range commits {
		s := commitSummary{
			SHA: githubStringValue(c["sha"]),
		}
		if commit, ok := c["commit"].(map[string]interface{}); ok {
			s.Message = githubStringValue(commit["message"])
			if author, ok := commit["author"].(map[string]interface{}); ok {
				s.Author = githubStringValue(author["name"])
				s.Date = githubStringValue(author["date"])
			}
		}
		// Truncate SHA for display
		if len(s.SHA) > 7 {
			s.SHA = s.SHA[:7]
		}
		summaries = append(summaries, s)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"count":   len(summaries),
		"commits": summaries,
	})
	return string(out)
}

// GitHubListWorkflowRuns lists recent GitHub Actions workflow runs.
func GitHubListWorkflowRuns(ctx context.Context, cfg GitHubConfig, owner, repo string, limit int) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" {
		return errJSON("Owner and repo name are required")
	}
	if msg := githubRepoAccessError(cfg, o, repo); msg != "" {
		return msg
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	runsQuery := url.Values{"per_page": []string{fmt.Sprintf("%d", limit)}}
	data, _, status, err := githubRequest(ctx, cfg, "GET", githubRepoEndpoint(o, repo, "actions/runs")+"?"+runsQuery.Encode(), nil)
	if err != nil {
		return errJSON("Failed to list workflow runs: %v", err)
	}
	if status != 200 {
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return errJSON("Failed to parse response: %v", err)
	}

	type runSummary struct {
		ID         interface{} `json:"id"`
		Name       string      `json:"name"`
		Status     string      `json:"status"`
		Conclusion string      `json:"conclusion"`
		Branch     string      `json:"branch"`
		CreatedAt  string      `json:"created_at"`
		HTMLURL    string      `json:"html_url"`
	}

	var summaries []runSummary
	if runs, ok := result["workflow_runs"].([]interface{}); ok {
		for _, r := range runs {
			rm, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			s := runSummary{
				ID:         rm["id"],
				Name:       githubStringValue(rm["name"]),
				Status:     githubStringValue(rm["status"]),
				Conclusion: githubStringValue(rm["conclusion"]),
				CreatedAt:  githubStringValue(rm["created_at"]),
				HTMLURL:    githubStringValue(rm["html_url"]),
			}
			if branch, ok := rm["head_branch"].(string); ok {
				s.Branch = branch
			}
			summaries = append(summaries, s)
		}
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"count":  len(summaries),
		"runs":   summaries,
	})
	return string(out)
}

// ── Search ──────────────────────────────────────────────────────────────────

// GitHubSearchRepos searches for repositories matching a query.
func GitHubSearchRepos(ctx context.Context, cfg GitHubConfig, query string, limit int) string {
	if query == "" {
		return errJSON("Search query is required")
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	searchQuery := url.Values{"q": []string{query}, "per_page": []string{fmt.Sprintf("%d", limit)}}
	data, _, status, err := githubRequest(ctx, cfg, "GET", "/search/repositories?"+searchQuery.Encode(), nil)
	if err != nil {
		return errJSON("Failed to search repos: %v", err)
	}
	if status != 200 {
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return errJSON("Failed to parse response: %v", err)
	}

	type repoResult struct {
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		Stars       int    `json:"stars"`
		Language    string `json:"language"`
		HTMLURL     string `json:"html_url"`
		Private     bool   `json:"private"`
	}

	var repos []repoResult
	if items, ok := result["items"].([]interface{}); ok {
		for _, item := range items {
			r, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			private, _ := r["private"].(bool)
			s := repoResult{
				FullName: githubStringValue(r["full_name"]),
				HTMLURL:  githubStringValue(r["html_url"]),
				Private:  private,
			}
			repoOwner, repoName := GitHubSplitFullName(s.FullName)
			if private && !GitHubRepoAllowed(cfg, repoOwner, repoName) {
				continue
			}
			if r["description"] != nil {
				s.Description = githubStringValue(r["description"])
			}
			if r["language"] != nil {
				s.Language = githubStringValue(r["language"])
			}
			if stars, ok := r["stargazers_count"].(float64); ok {
				s.Stars = int(stars)
			}
			repos = append(repos, s)
		}
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":      "ok",
		"total_count": result["total_count"],
		"repos":       repos,
	})
	return string(out)
}

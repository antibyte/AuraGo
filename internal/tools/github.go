package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aurago/internal/security"
)

// GitHubConfig holds the GitHub API connection parameters.
type GitHubConfig struct {
	Token          string // Personal Access Token (ghp_…)
	Owner          string // GitHub username or organisation
	BaseURL        string // API base URL (default: https://api.github.com)
	DefaultPrivate bool   // true = new repos are private by default
}

var githubHTTPClient = security.NewSSRFProtectedHTTPClient(30 * time.Second)

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
func githubRequest(cfg GitHubConfig, method, endpoint string, body interface{}) ([]byte, int, error) {
	base := cfg.BaseURL
	if base == "" {
		base = "https://api.github.com"
	}
	url := strings.TrimRight(base, "/") + endpoint

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := readHTTPResponseBody(resp.Body, maxHTTPResponseSize)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return respBody, resp.StatusCode, nil
}

// githubOwner returns the effective owner (user/org) for API calls.
func githubOwner(cfg GitHubConfig, owner string) string {
	if owner != "" {
		return owner
	}
	return cfg.Owner
}

// ── Repository operations ───────────────────────────────────────────────────

// GitHubListRepos lists repositories for the authenticated user or a specific owner.
func GitHubListRepos(cfg GitHubConfig, owner string) string {
	endpoint := "/user/repos?per_page=100&sort=updated"
	if owner != "" && owner != cfg.Owner {
		endpoint = fmt.Sprintf("/users/%s/repos?per_page=100&sort=updated", url.PathEscape(strings.TrimSpace(owner)))
	}

	data, status, err := githubRequest(cfg, "GET", endpoint, nil)
	if err != nil {
		return errJSON("Failed to list repos: %v", err)
	}
	if status != 200 {
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	var repos []map[string]interface{}
	if err := json.Unmarshal(data, &repos); err != nil {
		return errJSON("Failed to parse repos: %v", err)
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
			Name:     fmt.Sprintf("%v", r["name"]),
			FullName: fmt.Sprintf("%v", r["full_name"]),
			Private:  r["private"] == true,
			HTMLURL:  fmt.Sprintf("%v", r["html_url"]),
			CloneURL: fmt.Sprintf("%v", r["clone_url"]),
		}
		if r["description"] != nil {
			s.Description = fmt.Sprintf("%v", r["description"])
		}
		if r["language"] != nil {
			s.Language = fmt.Sprintf("%v", r["language"])
		}
		if r["updated_at"] != nil {
			s.UpdatedAt = fmt.Sprintf("%v", r["updated_at"])
		}
		summaries = append(summaries, s)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status": "ok",
		"count":  len(summaries),
		"repos":  summaries,
	})
	return string(out)
}

// GitHubCreateRepo creates a new repository.
func GitHubCreateRepo(cfg GitHubConfig, name, description string, private *bool) string {
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

	data, status, err := githubRequest(cfg, "POST", "/user/repos", body)
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

	out, _ := json.Marshal(map[string]interface{}{
		"status":    "ok",
		"message":   fmt.Sprintf("Repository '%s' created successfully", name),
		"name":      repo["name"],
		"full_name": repo["full_name"],
		"private":   repo["private"],
		"html_url":  repo["html_url"],
		"clone_url": repo["clone_url"],
	})
	return string(out)
}

// GitHubDeleteRepo deletes a repository.
func GitHubDeleteRepo(cfg GitHubConfig, owner, repo string) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" {
		return errJSON("Owner and repo name are required")
	}

	data, status, err := githubRequest(cfg, "DELETE", githubRepoEndpoint(o, repo), nil)
	if err != nil {
		return errJSON("Failed to delete repo: %v", err)
	}
	if status != 204 {
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": fmt.Sprintf("Repository '%s/%s' deleted", o, repo),
	})
	return string(out)
}

// GitHubGetRepo gets detailed information about a repository.
func GitHubGetRepo(cfg GitHubConfig, owner, repo string) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" {
		return errJSON("Owner and repo name are required")
	}

	data, status, err := githubRequest(cfg, "GET", githubRepoEndpoint(o, repo), nil)
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
func GitHubListIssues(cfg GitHubConfig, owner, repo, state string) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" {
		return errJSON("Owner and repo name are required")
	}
	if state == "" {
		state = "open"
	}

	issueQuery := url.Values{"state": []string{state}, "per_page": []string{"50"}}
	data, status, err := githubRequest(cfg, "GET", githubRepoEndpoint(o, repo, "issues")+"?"+issueQuery.Encode(), nil)
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
			Title:     fmt.Sprintf("%v", i["title"]),
			State:     fmt.Sprintf("%v", i["state"]),
			HTMLURL:   fmt.Sprintf("%v", i["html_url"]),
			CreatedAt: fmt.Sprintf("%v", i["created_at"]),
		}
		if user, ok := i["user"].(map[string]interface{}); ok {
			s.User = fmt.Sprintf("%v", user["login"])
		}
		if labels, ok := i["labels"].([]interface{}); ok {
			for _, l := range labels {
				if lm, ok := l.(map[string]interface{}); ok {
					s.Labels = append(s.Labels, fmt.Sprintf("%v", lm["name"]))
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
func GitHubCreateIssue(cfg GitHubConfig, owner, repo, title, body string, labels []string) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" || title == "" {
		return errJSON("Owner, repo name, and title are required")
	}

	payload := map[string]interface{}{
		"title": title,
		"body":  body,
	}
	if len(labels) > 0 {
		payload["labels"] = labels
	}

	data, status, err := githubRequest(cfg, "POST", githubRepoEndpoint(o, repo, "issues"), payload)
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
func GitHubCloseIssue(cfg GitHubConfig, owner, repo string, number int) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" || number <= 0 {
		return errJSON("Owner, repo name, and issue number are required")
	}

	data, status, err := githubRequest(cfg, "PATCH",
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
func GitHubListPullRequests(cfg GitHubConfig, owner, repo, state string) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" {
		return errJSON("Owner and repo name are required")
	}
	if state == "" {
		state = "open"
	}

	prQuery := url.Values{"state": []string{state}, "per_page": []string{"50"}}
	data, status, err := githubRequest(cfg, "GET", githubRepoEndpoint(o, repo, "pulls")+"?"+prQuery.Encode(), nil)
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
			Title:     fmt.Sprintf("%v", p["title"]),
			State:     fmt.Sprintf("%v", p["state"]),
			HTMLURL:   fmt.Sprintf("%v", p["html_url"]),
			CreatedAt: fmt.Sprintf("%v", p["created_at"]),
		}
		if user, ok := p["user"].(map[string]interface{}); ok {
			s.User = fmt.Sprintf("%v", user["login"])
		}
		if head, ok := p["head"].(map[string]interface{}); ok {
			s.Head = fmt.Sprintf("%v", head["ref"])
		}
		if base, ok := p["base"].(map[string]interface{}); ok {
			s.Base = fmt.Sprintf("%v", base["ref"])
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
func GitHubListBranches(cfg GitHubConfig, owner, repo string) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" {
		return errJSON("Owner and repo name are required")
	}

	branchQuery := url.Values{"per_page": []string{"100"}}
	data, status, err := githubRequest(cfg, "GET", githubRepoEndpoint(o, repo, "branches")+"?"+branchQuery.Encode(), nil)
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
		names = append(names, fmt.Sprintf("%v", b["name"]))
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":   "ok",
		"count":    len(names),
		"branches": names,
	})
	return string(out)
}

// ── File operations ─────────────────────────────────────────────────────────

// GitHubGetFileContent reads a file from a repository.
func GitHubGetFileContent(cfg GitHubConfig, owner, repo, path, branch string) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" || path == "" {
		return errJSON("Owner, repo, and file path are required")
	}

	endpoint := githubContentEndpoint(o, repo, path)
	if branch != "" {
		endpoint += "?" + url.Values{"ref": []string{branch}}.Encode()
	}

	data, status, err := githubRequest(cfg, "GET", endpoint, nil)
	if err != nil {
		return errJSON("Failed to get file: %v", err)
	}
	if status != 200 {
		return errJSON("GitHub API error (HTTP %d): %s", status, string(data))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return errJSON("Failed to parse response: %v", err)
	}

	out, _ := json.Marshal(map[string]interface{}{
		"status":   "ok",
		"name":     result["name"],
		"path":     result["path"],
		"size":     result["size"],
		"sha":      result["sha"],
		"content":  result["content"],
		"encoding": result["encoding"],
		"html_url": result["html_url"],
	})
	return string(out)
}

// GitHubCreateOrUpdateFile creates or updates a file in a repository.
func GitHubCreateOrUpdateFile(cfg GitHubConfig, owner, repo, path, content, message, sha, branch string) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" || path == "" {
		return errJSON("Owner, repo, and file path are required")
	}
	if message == "" {
		message = "Update " + path
	}

	payload := map[string]interface{}{
		"message": message,
		"content": content, // must be base64 encoded
	}
	if sha != "" {
		payload["sha"] = sha // required for updates
	}
	if branch != "" {
		payload["branch"] = branch
	}

	data, status, err := githubRequest(cfg, "PUT", githubContentEndpoint(o, repo, path), payload)
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
func GitHubListCommits(cfg GitHubConfig, owner, repo, branch string, limit int) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" {
		return errJSON("Owner and repo name are required")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	query := url.Values{"per_page": []string{fmt.Sprintf("%d", limit)}}
	if branch != "" {
		query.Set("sha", branch)
	}
	endpoint := githubRepoEndpoint(o, repo, "commits") + "?" + query.Encode()

	data, status, err := githubRequest(cfg, "GET", endpoint, nil)
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
			SHA: fmt.Sprintf("%v", c["sha"]),
		}
		if commit, ok := c["commit"].(map[string]interface{}); ok {
			s.Message = fmt.Sprintf("%v", commit["message"])
			if author, ok := commit["author"].(map[string]interface{}); ok {
				s.Author = fmt.Sprintf("%v", author["name"])
				s.Date = fmt.Sprintf("%v", author["date"])
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
func GitHubListWorkflowRuns(cfg GitHubConfig, owner, repo string, limit int) string {
	o := githubOwner(cfg, owner)
	if o == "" || repo == "" {
		return errJSON("Owner and repo name are required")
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	runsQuery := url.Values{"per_page": []string{fmt.Sprintf("%d", limit)}}
	data, status, err := githubRequest(cfg, "GET", githubRepoEndpoint(o, repo, "actions/runs")+"?"+runsQuery.Encode(), nil)
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
				Name:       fmt.Sprintf("%v", rm["name"]),
				Status:     fmt.Sprintf("%v", rm["status"]),
				Conclusion: fmt.Sprintf("%v", rm["conclusion"]),
				CreatedAt:  fmt.Sprintf("%v", rm["created_at"]),
				HTMLURL:    fmt.Sprintf("%v", rm["html_url"]),
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
func GitHubSearchRepos(cfg GitHubConfig, query string, limit int) string {
	if query == "" {
		return errJSON("Search query is required")
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	searchQuery := url.Values{"q": []string{query}, "per_page": []string{fmt.Sprintf("%d", limit)}}
	data, status, err := githubRequest(cfg, "GET", "/search/repositories?"+searchQuery.Encode(), nil)
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
	}

	var repos []repoResult
	if items, ok := result["items"].([]interface{}); ok {
		for _, item := range items {
			r, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			s := repoResult{
				FullName: fmt.Sprintf("%v", r["full_name"]),
				HTMLURL:  fmt.Sprintf("%v", r["html_url"]),
			}
			if r["description"] != nil {
				s.Description = fmt.Sprintf("%v", r["description"])
			}
			if r["language"] != nil {
				s.Language = fmt.Sprintf("%v", r["language"])
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

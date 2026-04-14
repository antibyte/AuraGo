// Package outputcompress – GitHub API output compressors.
//
// GitHub tool outputs are already compact JSON summaries, but for large
// lists (repos, issues, PRs, commits, workflow runs) the token count
// still grows linearly. These compressors:
//   - Strip the "Tool Output: " wrapper prefix
//   - Parse the JSON to detect the response type (repos, issues, PRs, etc.)
//   - Replace verbose per-item JSON with a compact table-like format
//   - Truncate long arrays with a "+N more" indicator
package outputcompress

import (
	"encoding/json"
	"fmt"
	"strings"
)

// compressGitHubOutput routes GitHub tool output to the appropriate sub-compressor.
func compressGitHubOutput(output string) (string, string) {
	clean := StripANSI(output)
	clean = strings.TrimPrefix(clean, "Tool Output: ")
	clean = strings.TrimSpace(clean)

	// Must be JSON
	if !strings.HasPrefix(clean, "{") {
		return compressGeneric(output), "github-nonjson"
	}

	// Parse top-level to detect response type
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(clean), &raw); err != nil {
		return compressGeneric(output), "github-parse-err"
	}

	// Error responses: return as-is (already compact)
	if statusStr := jsonString(raw["status"]); statusStr == "error" {
		return clean, "github-error"
	}

	// Detect response type by the array field present
	switch {
	case raw["repos"] != nil:
		return compressGitHubRepos(raw), "github-repos"
	case raw["issues"] != nil:
		return compressGitHubIssues(raw), "github-issues"
	case raw["pull_requests"] != nil:
		return compressGitHubPRs(raw), "github-prs"
	case raw["commits"] != nil:
		return compressGitHubCommits(raw), "github-commits"
	case raw["runs"] != nil:
		return compressGitHubRuns(raw), "github-runs"
	case raw["branches"] != nil:
		return compressGitHubBranches(raw), "github-branches"
	case raw["message"] != nil && raw["name"] != nil:
		// Single-item responses (create repo, create issue, etc.) – already compact
		return clean, "github-single"
	default:
		// Unknown structure – compact JSON
		return compactJSON(clean), "github-generic"
	}
}

// compressGitHubRepos compresses repo list output.
// From: {"status":"ok","count":30,"repos":[{"name":"repo1","full_name":"user/repo1",...},...]}
// To:    "30 repos:\n  user/repo1 (Go) – Description\n  user/repo2 (Python) – Description\n  ..."
func compressGitHubRepos(raw map[string]json.RawMessage) string {
	count := jsonInt(raw["count"])

	type repo struct {
		Name        string `json:"name"`
		FullName    string `json:"full_name"`
		Description string `json:"description"`
		Private     bool   `json:"private"`
		Language    string `json:"language"`
		UpdatedAt   string `json:"updated_at"`
		HTMLURL     string `json:"html_url"`
	}

	var repos []repo
	if err := json.Unmarshal(raw["repos"], &repos); err != nil {
		return compactJSON(rawToString(raw))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d repos", count)

	privateCount := 0
	for _, r := range repos {
		if r.Private {
			privateCount++
		}
	}
	if privateCount > 0 {
		fmt.Fprintf(&sb, " (%d private)", privateCount)
	}
	sb.WriteString(":\n")

	limit := 20
	if len(repos) < limit {
		limit = len(repos)
	}
	for i := 0; i < limit; i++ {
		r := repos[i]
		sb.WriteString("  ")
		sb.WriteString(r.FullName)
		if r.Language != "" {
			sb.WriteString(" (" + r.Language + ")")
		}
		if r.Private {
			sb.WriteString(" [private]")
		}
		if r.Description != "" {
			desc := r.Description
			if len(desc) > 80 {
				desc = desc[:77] + "..."
			}
			sb.WriteString(" – " + desc)
		}
		sb.WriteString("\n")
	}
	if len(repos) > limit {
		fmt.Fprintf(&sb, "  + %d more\n", len(repos)-limit)
	}

	return sb.String()
}

// compressGitHubIssues compresses issue list output.
// From: {"status":"ok","count":15,"issues":[{"number":42,"title":"Bug",...},...]}
// To:   "15 issues:\n  #42 Bug (open) @user [bug,urgent]\n  ..."
func compressGitHubIssues(raw map[string]json.RawMessage) string {
	count := jsonInt(raw["count"])

	type issue struct {
		Number    int      `json:"number"`
		Title     string   `json:"title"`
		State     string   `json:"state"`
		User      string   `json:"user"`
		Labels    []string `json:"labels"`
		CreatedAt string   `json:"created_at"`
	}

	var issues []issue
	if err := json.Unmarshal(raw["issues"], &issues); err != nil {
		return compactJSON(rawToString(raw))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d issues:\n", count)

	limit := 25
	if len(issues) < limit {
		limit = len(issues)
	}
	for i := 0; i < limit; i++ {
		iss := issues[i]
		fmt.Fprintf(&sb, "  #%d %s (%s)", iss.Number, iss.Title, iss.State)
		if iss.User != "" {
			sb.WriteString(" @" + iss.User)
		}
		if len(iss.Labels) > 0 {
			sb.WriteString(" [" + strings.Join(iss.Labels, ",") + "]")
		}
		sb.WriteString("\n")
	}
	if len(issues) > limit {
		fmt.Fprintf(&sb, "  + %d more\n", len(issues)-limit)
	}

	return sb.String()
}

// compressGitHubPRs compresses pull request list output.
// From: {"status":"ok","count":10,"pull_requests":[{"number":5,"title":"Fix",...},...]}
// To:   "10 PRs:\n  #5 Fix (open) feature → main @user\n  ..."
func compressGitHubPRs(raw map[string]json.RawMessage) string {
	count := jsonInt(raw["count"])

	type pr struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		State     string `json:"state"`
		User      string `json:"user"`
		Head      string `json:"head"`
		Base      string `json:"base"`
		CreatedAt string `json:"created_at"`
	}

	var prs []pr
	if err := json.Unmarshal(raw["pull_requests"], &prs); err != nil {
		return compactJSON(rawToString(raw))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d PRs:\n", count)

	limit := 25
	if len(prs) < limit {
		limit = len(prs)
	}
	for i := 0; i < limit; i++ {
		p := prs[i]
		fmt.Fprintf(&sb, "  #%d %s (%s)", p.Number, p.Title, p.State)
		if p.Head != "" && p.Base != "" {
			fmt.Fprintf(&sb, " %s → %s", p.Head, p.Base)
		}
		if p.User != "" {
			sb.WriteString(" @" + p.User)
		}
		sb.WriteString("\n")
	}
	if len(prs) > limit {
		fmt.Fprintf(&sb, "  + %d more\n", len(prs)-limit)
	}

	return sb.String()
}

// compressGitHubCommits compresses commit list output.
// From: {"status":"ok","count":20,"commits":[{"sha":"abc1234","message":"Fix bug",...},...]}
// To:   "20 commits:\n  abc1234 Fix bug (Author, 2024-01-15)\n  ..."
func compressGitHubCommits(raw map[string]json.RawMessage) string {
	count := jsonInt(raw["count"])

	type commit struct {
		SHA     string `json:"sha"`
		Message string `json:"message"`
		Author  string `json:"author"`
		Date    string `json:"date"`
	}

	var commits []commit
	if err := json.Unmarshal(raw["commits"], &commits); err != nil {
		return compactJSON(rawToString(raw))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d commits:\n", count)

	limit := 25
	if len(commits) < limit {
		limit = len(commits)
	}
	for i := 0; i < limit; i++ {
		c := commits[i]
		// Truncate commit message to first line
		msg := c.Message
		if idx := strings.Index(msg, "\n"); idx > 0 {
			msg = msg[:idx]
		}
		if len(msg) > 80 {
			msg = msg[:77] + "..."
		}
		fmt.Fprintf(&sb, "  %s %s", c.SHA, msg)
		if c.Author != "" {
			sb.WriteString(" (" + c.Author)
			if c.Date != "" {
				// Shorten ISO date to date-only
				dateShort := c.Date
				if len(dateShort) > 10 {
					dateShort = dateShort[:10]
				}
				sb.WriteString(", " + dateShort)
			}
			sb.WriteString(")")
		}
		sb.WriteString("\n")
	}
	if len(commits) > limit {
		fmt.Fprintf(&sb, "  + %d more\n", len(commits)-limit)
	}

	return sb.String()
}

// compressGitHubRuns compresses workflow run list output.
// From: {"status":"ok","count":10,"runs":[{"id":123,"name":"CI","status":"completed",...},...]}
// To:   "10 workflow runs:\n  CI (completed/success) branch:main 2024-01-15\n  ..."
func compressGitHubRuns(raw map[string]json.RawMessage) string {
	count := jsonInt(raw["count"])

	type run struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		Branch     string `json:"branch"`
		CreatedAt  string `json:"created_at"`
	}

	var runs []run
	if err := json.Unmarshal(raw["runs"], &runs); err != nil {
		return compactJSON(rawToString(raw))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d workflow runs:\n", count)

	limit := 15
	if len(runs) < limit {
		limit = len(runs)
	}
	for i := 0; i < limit; i++ {
		r := runs[i]
		fmt.Fprintf(&sb, "  %s (%s", r.Name, r.Status)
		if r.Conclusion != "" {
			sb.WriteString("/" + r.Conclusion)
		}
		sb.WriteString(")")
		if r.Branch != "" {
			sb.WriteString(" branch:" + r.Branch)
		}
		if r.CreatedAt != "" {
			dateShort := r.CreatedAt
			if len(dateShort) > 10 {
				dateShort = dateShort[:10]
			}
			sb.WriteString(" " + dateShort)
		}
		sb.WriteString("\n")
	}
	if len(runs) > limit {
		fmt.Fprintf(&sb, "  + %d more\n", len(runs)-limit)
	}

	return sb.String()
}

// compressGitHubBranches compresses branch list output.
// From: {"status":"ok","branches":[{"name":"main","protected":true},...]}
// To:   "N branches:\n  main [protected]\n  develop\n  ..."
func compressGitHubBranches(raw map[string]json.RawMessage) string {
	type branch struct {
		Name      string `json:"name"`
		Protected bool   `json:"protected"`
	}

	var branches []branch
	if err := json.Unmarshal(raw["branches"], &branches); err != nil {
		return compactJSON(rawToString(raw))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d branches:\n", len(branches))

	limit := 30
	if len(branches) < limit {
		limit = len(branches)
	}
	for i := 0; i < limit; i++ {
		b := branches[i]
		sb.WriteString("  " + b.Name)
		if b.Protected {
			sb.WriteString(" [protected]")
		}
		sb.WriteString("\n")
	}
	if len(branches) > limit {
		fmt.Fprintf(&sb, "  + %d more\n", len(branches)-limit)
	}

	return sb.String()
}

// JSON helpers werden jetzt aus utils.go importiert (zentralisiert)

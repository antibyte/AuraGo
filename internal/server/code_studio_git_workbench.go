package server

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Git workbench of Code Studio: staging, discard, repository init, commit view and blame.
// Every command runs through execGit (an argument vector, never a shell string) and every
// path is workspace-relative and passed after "--".

const (
	// The log fields are separated by the ASCII unit separator, which cannot occur in a
	// hash, a name, an ISO date or a one-line subject.
	codeStudioGitLogFormat = "--format=%H%x1f%h%x1f%an%x1f%aI%x1f%s"

	// The commit metadata ends with a record separator; the --name-status list follows it.
	codeStudioGitShowFormat = "--format=%H%x1f%h%x1f%an%x1f%ae%x1f%aI%x1f%s%x1f%b%x1e"

	// codeStudioGitShowDiffLimit caps the patch of the commit view at 512 KiB.
	codeStudioGitShowDiffLimit = 512 * 1024

	// codeStudioGitMaxPaths bounds one stage, unstage or discard request.
	codeStudioGitMaxPaths = 1000
)

var codeStudioGitHashPattern = regexp.MustCompile(`^[0-9a-fA-F]{4,40}$`)

type codeStudioGitFileChange struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

type codeStudioGitCommit struct {
	Hash    string
	Short   string
	Author  string
	Email   string
	Date    string
	Subject string
	Body    string
	Files   []codeStudioGitFileChange
}

type codeStudioGitBlameLine struct {
	Line    int    `json:"line"`
	Hash    string `json:"hash"`
	Short   string `json:"short"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Summary string `json:"summary"`
}

// codeStudioGitLogArgs is the log command of the history list and of the status summary.
func codeStudioGitLogArgs(count int) []string {
	return []string{"log", codeStudioGitLogFormat, "-n", strconv.Itoa(count)}
}

// codeStudioGitNotRepository reports git's "not a git repository" failure.
func codeStudioGitNotRepository(output string) bool {
	return strings.Contains(strings.ToLower(output), "not a git repository")
}

// codeStudioGitPaths turns request paths into unique workspace-relative pathspecs. The
// workspace root itself is refused, so no request can name every file at once.
func codeStudioGitPaths(raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("paths are required")
	}
	if len(raw) > codeStudioGitMaxPaths {
		return nil, fmt.Errorf("too many paths")
	}
	rels := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, item := range raw {
		rel, err := codeStudioGitAddPath(item)
		if err != nil {
			return nil, err
		}
		if !seen[rel] {
			seen[rel] = true
			rels = append(rels, rel)
		}
	}
	return rels, nil
}

func codeStudioGitIsHex(value string) bool {
	return value != "" && strings.Trim(value, "0123456789abcdefABCDEF") == ""
}

// parseGitLog reads the output of codeStudioGitLogArgs. Lines without the five fields
// (warnings, stderr noise) are skipped.
func parseGitLog(output string) []map[string]string {
	entries := []map[string]string{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.SplitN(strings.TrimRight(line, "\r"), "\x1f", 5)
		if len(fields) != 5 {
			continue
		}
		hash := strings.TrimSpace(fields[0])
		if len(hash) < 4 || !codeStudioGitIsHex(hash) {
			continue
		}
		entries = append(entries, map[string]string{
			"hash":    hash,
			"short":   fields[1],
			"author":  fields[2],
			"date":    fields[3],
			"message": fields[4],
		})
	}
	return entries
}

// parseGitShowMeta reads `git show --name-status` with codeStudioGitShowFormat.
func parseGitShowMeta(output string) (codeStudioGitCommit, error) {
	head, rest, found := strings.Cut(output, "\x1e")
	fields := strings.SplitN(head, "\x1f", 7)
	if !found || len(fields) != 7 {
		return codeStudioGitCommit{}, fmt.Errorf("unexpected git show output")
	}
	commit := codeStudioGitCommit{
		Hash:    strings.TrimSpace(fields[0]),
		Short:   strings.TrimSpace(fields[1]),
		Author:  fields[2],
		Email:   fields[3],
		Date:    strings.TrimSpace(fields[4]),
		Subject: fields[5],
		Body:    strings.TrimSpace(fields[6]),
		Files:   []codeStudioGitFileChange{},
	}
	for _, line := range strings.Split(rest, "\n") {
		parts := strings.Split(strings.TrimRight(line, "\r"), "\t")
		if len(parts) < 2 || parts[0] == "" {
			continue
		}
		// Renames and copies list the old and the new path; the new one is the last field.
		commit.Files = append(commit.Files, codeStudioGitFileChange{
			Path:   codeStudioGitUnquotePath(parts[len(parts)-1]),
			Status: parts[0][:1],
		})
	}
	return commit, nil
}

// codeStudioGitUnquotePath undoes git's C-style quoting of paths with control characters.
func codeStudioGitUnquotePath(path string) string {
	if len(path) >= 2 && strings.HasPrefix(path, `"`) && strings.HasSuffix(path, `"`) {
		if unquoted, err := strconv.Unquote(path); err == nil {
			return unquoted
		}
	}
	return path
}

// truncateCodeStudioGitDiff cuts a diff to limit bytes at a line end.
func truncateCodeStudioGitDiff(diff string, limit int) (string, bool) {
	if len(diff) <= limit {
		return diff, false
	}
	cut := diff[:limit]
	if index := strings.LastIndexByte(cut, '\n'); index > 0 {
		cut = cut[:index+1]
	}
	return strings.ToValidUTF8(cut, ""), true
}

// codeStudioGitZone turns a git zone such as "+0100" or "-0230" into a fixed location.
func codeStudioGitZone(zone string) *time.Location {
	if len(zone) != 5 || (zone[0] != '+' && zone[0] != '-') {
		return time.UTC
	}
	hours, hoursErr := strconv.Atoi(zone[1:3])
	minutes, minutesErr := strconv.Atoi(zone[3:5])
	if hoursErr != nil || minutesErr != nil {
		return time.UTC
	}
	offset := hours*3600 + minutes*60
	if zone[0] == '-' {
		offset = -offset
	}
	return time.FixedZone(zone, offset)
}

// parseGitBlamePorcelain reads `git blame --porcelain`. The details of a commit are printed
// only the first time it appears, so they are collected first and joined to every line at
// the end. Source lines start with a TAB and are never parsed.
func parseGitBlamePorcelain(output string) []codeStudioGitBlameLine {
	type blameCommit struct {
		author     string
		summary    string
		zone       string
		authorTime int64
		hasTime    bool
	}
	type blameRef struct {
		hash string
		line int
	}
	commits := map[string]*blameCommit{}
	refs := []blameRef{}
	current := ""
	finalLine := 0
	for _, raw := range strings.Split(output, "\n") {
		if strings.HasPrefix(raw, "\t") {
			if current != "" && finalLine > 0 {
				refs = append(refs, blameRef{hash: current, line: finalLine})
			}
			continue
		}
		line := strings.TrimRight(raw, "\r")
		fields := strings.Fields(line)
		if len(fields) >= 3 && (len(fields[0]) == 40 || len(fields[0]) == 64) && codeStudioGitIsHex(fields[0]) {
			if number, err := strconv.Atoi(fields[2]); err == nil {
				current = fields[0]
				finalLine = number
				if commits[current] == nil {
					commits[current] = &blameCommit{}
				}
				continue
			}
		}
		if current == "" {
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		info := commits[current]
		switch key {
		case "author":
			info.author = value
		case "author-time":
			if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
				info.authorTime = seconds
				info.hasTime = true
			}
		case "author-tz":
			info.zone = value
		case "summary":
			info.summary = value
		}
	}
	lines := make([]codeStudioGitBlameLine, 0, len(refs))
	for _, ref := range refs {
		info := commits[ref.hash]
		short := ref.hash
		if len(short) > 7 {
			short = short[:7]
		}
		date := ""
		if info.hasTime {
			date = time.Unix(info.authorTime, 0).In(codeStudioGitZone(info.zone)).Format(time.RFC3339)
		}
		lines = append(lines, codeStudioGitBlameLine{Line: ref.line, Hash: ref.hash, Short: short, Author: info.author, Date: date, Summary: info.summary})
	}
	return lines
}

// gitPathsRequest checks a write request with the body {"paths": [...]} and resolves the
// workspace backend. The pointer receiver matters: backendFor installs the Docker adapter on h,
// and the calling handler runs git through the returned backend.
func (h *codeStudioHandlers) gitPathsRequest(w http.ResponseWriter, r *http.Request) ([]string, codeStudioBackend, bool) {
	if !h.requirePermission(w, r, desktopScopeWrite) {
		return nil, nil, false
	}
	if h.rejectReadOnly(w) {
		return nil, nil, false
	}
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return nil, nil, false
	}
	var body struct {
		Paths []string `json:"paths"`
	}
	if err := decodeJSONRequest(w, r, &body, h.server); err != nil {
		jsonError(w, "Invalid JSON", jsonDecodeStatus(err))
		return nil, nil, false
	}
	rels, err := codeStudioGitPaths(body.Paths)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return nil, nil, false
	}
	b, err := h.backendFor(r)
	if err != nil {
		writeCodeStudioBackendError(w, err)
		return nil, nil, false
	}
	return rels, b, true
}

// runGitSteps runs the commands in order and stops at the first failure, whose git output
// is returned verbatim.
// It reports whether every step succeeded (the response is written either way).
func (h codeStudioHandlers) runGitSteps(w http.ResponseWriter, r *http.Request, b codeStudioBackend, steps [][]string, fallback string) bool {
	for _, args := range steps {
		result, err := b.Git(r.Context(), 30*time.Second, args...)
		if err != nil {
			writeCodeStudioOpError(w, err)
			return false
		}
		if result.ExitCode != 0 {
			jsonError(w, codeStudioGitExecError(result.Output, fallback), http.StatusBadRequest)
			return false
		}
	}
	writeJSON(w, map[string]interface{}{"status": "ok"})
	return true
}

func (h codeStudioHandlers) handleGitStage(w http.ResponseWriter, r *http.Request) {
	rels, b, ok := h.gitPathsRequest(w, r)
	if !ok {
		return
	}
	h.runGitSteps(w, r, b, [][]string{append([]string{"add", "--"}, rels...)}, "git add failed")
}

func (h codeStudioHandlers) handleGitUnstage(w http.ResponseWriter, r *http.Request) {
	rels, b, ok := h.gitPathsRequest(w, r)
	if !ok {
		return
	}
	head, err := b.Git(r.Context(), 10*time.Second, "rev-parse", "--verify", "-q", "HEAD")
	if err != nil {
		writeCodeStudioOpError(w, err)
		return
	}
	args := append([]string{"restore", "--staged", "--"}, rels...)
	if head.ExitCode != 0 {
		// Before the first commit there is no HEAD to restore the index from.
		args = append([]string{"rm", "--cached", "-r", "-q", "--"}, rels...)
	}
	h.runGitSteps(w, r, b, [][]string{args}, "git unstage failed")
}

func (h codeStudioHandlers) handleGitDiscard(w http.ResponseWriter, r *http.Request) {
	rels, b, ok := h.gitPathsRequest(w, r)
	if !ok {
		return
	}
	// The server decides itself which paths are untracked: only those are deleted with
	// git clean; every other path is restored from the index.
	statusArgs := append([]string{"status", "--porcelain=v1", "-z", "--"}, rels...)
	statusResult, err := b.Git(r.Context(), 15*time.Second, statusArgs...)
	if err != nil {
		writeCodeStudioOpError(w, err)
		return
	}
	if statusResult.ExitCode != 0 {
		jsonError(w, codeStudioGitExecError(statusResult.Output, "git status failed"), http.StatusBadRequest)
		return
	}
	untracked := map[string]bool{}
	for _, change := range parseGitPorcelain(statusResult.Output) {
		if change["status"] == "??" {
			untracked[strings.TrimSuffix(change["path"], "/")] = strings.HasSuffix(change["path"], "/")
		}
	}
	var files, dirs, tracked []string
	for _, rel := range rels {
		isDir, isUntracked := untracked[rel]
		switch {
		case isUntracked && isDir:
			dirs = append(dirs, rel)
		case isUntracked:
			files = append(files, rel)
		default:
			tracked = append(tracked, rel)
		}
	}
	steps := [][]string{}
	if len(files) > 0 {
		steps = append(steps, append([]string{"clean", "-f", "-q", "--"}, files...))
	}
	if len(dirs) > 0 {
		steps = append(steps, append([]string{"clean", "-f", "-q", "-d", "--"}, dirs...))
	}
	if len(tracked) > 0 {
		steps = append(steps, append([]string{"restore", "--worktree", "--"}, tracked...))
	}
	if h.runGitSteps(w, r, b, steps, "git discard failed") {
		virtual := make([]string, len(rels))
		for i, rel := range rels {
			virtual[i] = codeStudioWorkspaceRoot + "/" + rel
		}
		h.notifyWorkspaceChange(b.Info().ID, virtual...)
	}
}

func (h codeStudioHandlers) handleGitInit(w http.ResponseWriter, r *http.Request) {
	if !h.requirePermission(w, r, desktopScopeWrite) {
		return
	}
	if h.rejectReadOnly(w) {
		return
	}
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	b, err := h.backendFor(r)
	if err != nil {
		writeCodeStudioBackendError(w, err)
		return
	}
	probe, err := b.Git(r.Context(), 10*time.Second, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		writeCodeStudioOpError(w, err)
		return
	}
	if probe.ExitCode == 0 {
		jsonError(w, "workspace is already a git repository", http.StatusConflict)
		return
	}
	if !codeStudioGitNotRepository(probe.Output) {
		jsonError(w, codeStudioGitExecError(probe.Output, "git rev-parse failed"), http.StatusBadRequest)
		return
	}
	if h.runGitSteps(w, r, b, [][]string{{"init"}}, "git init failed") {
		h.notifyWorkspaceChange(b.Info().ID)
	}
}

func (h codeStudioHandlers) handleGitShow(w http.ResponseWriter, r *http.Request) {
	if !h.requirePermission(w, r, desktopScopeRead) {
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hash := strings.TrimSpace(r.URL.Query().Get("hash"))
	if !codeStudioGitHashPattern.MatchString(hash) {
		jsonError(w, "invalid commit hash", http.StatusBadRequest)
		return
	}
	b, err := h.backendFor(r)
	if err != nil {
		writeCodeStudioBackendError(w, err)
		return
	}
	// The hash holds hex digits only, so it can never be read as an option; "--" ends the revisions.
	metaResult, err := b.Git(r.Context(), 15*time.Second, "-c", "core.quotePath=false", "show", "--no-color", "--name-status", codeStudioGitShowFormat, hash, "--")
	if err != nil {
		writeCodeStudioOpError(w, err)
		return
	}
	if metaResult.ExitCode != 0 {
		jsonError(w, codeStudioGitExecError(metaResult.Output, "git show failed"), http.StatusBadRequest)
		return
	}
	commit, err := parseGitShowMeta(metaResult.Output)
	if err != nil {
		writeCodeStudioOpError(w, err)
		return
	}
	patchResult, err := b.Git(r.Context(), 30*time.Second, "show", "--no-color", "--format=", "--patch", hash, "--")
	if err != nil {
		writeCodeStudioOpError(w, err)
		return
	}
	if patchResult.ExitCode != 0 {
		jsonError(w, codeStudioGitExecError(patchResult.Output, "git show failed"), http.StatusBadRequest)
		return
	}
	diff, truncated := truncateCodeStudioGitDiff(strings.TrimLeft(patchResult.Output, "\n"), codeStudioGitShowDiffLimit)
	writeJSON(w, map[string]interface{}{
		"status":    "ok",
		"hash":      commit.Hash,
		"short":     commit.Short,
		"author":    commit.Author,
		"email":     commit.Email,
		"date":      commit.Date,
		"subject":   commit.Subject,
		"body":      commit.Body,
		"files":     commit.Files,
		"diff":      diff,
		"truncated": truncated,
	})
}

func (h codeStudioHandlers) handleGitBlame(w http.ResponseWriter, r *http.Request) {
	if !h.requirePermission(w, r, desktopScopeRead) {
		return
	}
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rel, err := codeStudioGitAddPath(r.URL.Query().Get("path"))
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	b, err := h.backendFor(r)
	if err != nil {
		writeCodeStudioBackendError(w, err)
		return
	}
	result, err := b.Git(r.Context(), 30*time.Second, "blame", "--porcelain", "--", rel)
	if err != nil {
		writeCodeStudioOpError(w, err)
		return
	}
	if result.ExitCode != 0 {
		jsonError(w, codeStudioGitExecError(result.Output, "git blame failed"), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "path": rel, "lines": parseGitBlamePorcelain(result.Output)})
}

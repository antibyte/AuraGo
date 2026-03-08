package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// UpdateCheckResult is the payload returned by /api/updates/check.
type UpdateCheckResult struct {
	Mode            string `json:"mode"`             // "git" or "binary"
	UpdateAvailable bool   `json:"update_available"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	CommitCount     int    `json:"commit_count,omitempty"`
	Changelog       string `json:"changelog,omitempty"`
	Message         string `json:"message"`
	Error           string `json:"error,omitempty"`
}

const githubRepo = "antibyte/AuraGo"

// appInstallDir returns the directory the application was installed in.
func appInstallDir(s *Server) string {
	if s.Cfg.ConfigPath != "" {
		return filepath.Dir(s.Cfg.ConfigPath)
	}
	d, _ := os.Getwd()
	return d
}

// isBinaryInstall returns true when the install dir has no .git directory
// (i.e. was installed from a GitHub Release, not cloned from source).
func isBinaryInstall(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return os.IsNotExist(err)
}

// readInstalledVersion reads the .version file written by install.sh / update.sh.
func readInstalledVersion(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, ".version"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(data))
}

// fetchLatestRelease queries the GitHub API for the latest release tag.
func fetchLatestRelease() (tag string, err error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "AuraGo-Updater/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var release struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return "", err
	}
	return release.TagName, nil
}

// checkUpdates detects mode and returns update information.
func checkUpdates(dir string) UpdateCheckResult {
	if isBinaryInstall(dir) {
		return checkUpdatesBinary(dir)
	}
	return checkUpdatesGit(dir)
}

// checkUpdatesBinary checks for a new GitHub Release.
func checkUpdatesBinary(dir string) UpdateCheckResult {
	current := readInstalledVersion(dir)

	latest, err := fetchLatestRelease()
	if err != nil {
		return UpdateCheckResult{
			Mode:    "binary",
			Error:   fmt.Sprintf("Could not reach GitHub: %v", err),
			Message: "Update check failed.",
		}
	}

	available := current != "unknown" && latest != "" && current != latest
	msg := "AuraGo is up to date."
	if current == "unknown" {
		msg = "Installed version could not be determined. Latest available: " + latest
	}
	if available {
		msg = fmt.Sprintf("Update available: %s → %s", current, latest)
	}

	return UpdateCheckResult{
		Mode:            "binary",
		UpdateAvailable: available,
		CurrentVersion:  current,
		LatestVersion:   latest,
		Message:         msg,
	}
}

// checkUpdatesGit checks for new commits on origin/main.
func checkUpdatesGit(dir string) UpdateCheckResult {
	// Determine current version from git describe
	current := "unknown"
	if out, err := runCmd(dir, "git", "-c", "safe.directory="+dir, "describe", "--tags", "--always"); err == nil {
		current = strings.TrimSpace(string(out))
	}

	// Fetch
	if _, err := runCmd(dir, "git", "-c", "safe.directory="+dir, "fetch", "origin", "main", "--quiet"); err != nil {
		return UpdateCheckResult{
			Mode:    "git",
			Error:   fmt.Sprintf("Could not reach GitHub: %v", err),
			Message: "Update check failed.",
		}
	}

	// Count new commits
	countOut, err := runCmd(dir, "git", "-c", "safe.directory="+dir, "rev-list", "HEAD..origin/main", "--count")
	if err != nil {
		return UpdateCheckResult{Mode: "git", Error: err.Error()}
	}
	count, _ := strconv.Atoi(strings.TrimSpace(string(countOut)))

	// Get latest release tag
	latest, _ := fetchLatestRelease()

	if count == 0 {
		return UpdateCheckResult{
			Mode:            "git",
			UpdateAvailable: false,
			CurrentVersion:  current,
			LatestVersion:   latest,
			Message:         "AuraGo is up to date.",
		}
	}

	logOut, _ := runCmd(dir, "git", "-c", "safe.directory="+dir, "log", "HEAD..origin/main", "--oneline", "-n", "10")

	return UpdateCheckResult{
		Mode:            "git",
		UpdateAvailable: true,
		CurrentVersion:  current,
		LatestVersion:   latest,
		CommitCount:     count,
		Changelog:       strings.TrimSpace(string(logOut)),
		Message:         fmt.Sprintf("%d update(s) available.", count),
	}
}

// runCmd runs a command and returns combined output.
func runCmd(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if home, _ := os.UserHomeDir(); home != "" {
		cmd.Env = append(os.Environ(), "HOME="+home)
	}
	return cmd.CombinedOutput()
}

// handleUpdateCheck returns the current update status.
// GET /api/updates/check
func handleUpdateCheck(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		result := checkUpdates(appInstallDir(s))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// handleUpdateInstall starts update.sh --yes in the background.
// POST /api/updates/install
func handleUpdateInstall(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		dir := appInstallDir(s)
		scriptPath := filepath.Join(dir, "update.sh")
		if _, err := os.Stat(scriptPath); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "update.sh not found in installation directory. Download it from: https://raw.githubusercontent.com/" + githubRepo + "/main/update.sh",
			})
			return
		}

		cmd := exec.Command("/bin/bash", "update.sh", "--yes")
		cmd.Dir = dir
		if home, _ := os.UserHomeDir(); home != "" {
			cmd.Env = append(os.Environ(), "HOME="+home)
		}

		if err := cmd.Start(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("Failed to start update: %v", err),
			})
			return
		}

		s.Logger.Info("[Update] Update script started", "pid", cmd.Process.Pid)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "started",
			"message": "Update started. AuraGo will restart automatically once the update is complete.",
		})
	}
}

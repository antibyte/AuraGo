package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os/exec"
	"runtime"

	"aurago/internal/updater"
)

// UpdateCheckResult is the payload returned by /api/updates/check.
type UpdateCheckResult = updater.CheckResult

const githubRepo = updater.DefaultGitHubRepo

var (
	updateGOOS         = runtime.GOOS
	updateLookPath     = exec.LookPath
	updateStartInstall = updater.StartInstall
)

// appInstallDir returns the directory the application was installed in.
func appInstallDir(s *Server) string {
	if s == nil || s.Cfg == nil {
		return updater.InstallDir("")
	}
	return updater.InstallDir(s.Cfg.ConfigPath)
}

func isBinaryInstall(dir string) bool {
	return updater.IsBinaryInstall(dir)
}

func readInstalledVersion(dir string) string {
	return updater.ReadInstalledVersion(dir)
}

// checkUpdates detects mode and returns update information.
func checkUpdates(dir string) UpdateCheckResult {
	return updater.CheckUpdates(context.Background(), updater.CheckOptions{InstallDir: dir})
}

// checkUpdatesBinary checks for a new GitHub Release.
func checkUpdatesBinary(dir string) UpdateCheckResult {
	return updater.CheckUpdates(context.Background(), updater.CheckOptions{InstallDir: dir})
}

// checkUpdatesGit checks for new commits on origin/main.
func checkUpdatesGit(dir string) UpdateCheckResult {
	return updater.CheckUpdates(context.Background(), updater.CheckOptions{InstallDir: dir})
}

// handleUpdateCheck returns the current update status.
// GET /api/updates/check
func handleUpdateCheck(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
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
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		s.CfgMu.RLock()
		cfg := s.Cfg
		s.CfgMu.RUnlock()

		result, err := updateStartInstall(updater.StartInstallOptions{
			Cfg:        cfg,
			InstallDir: appInstallDir(s),
			GOOS:       updateGOOS,
			LookPath:   updateLookPath,
		})
		if err != nil {
			status := http.StatusServiceUnavailable
			if errors.Is(err, updater.ErrSelfUpdateDisabled) {
				status = http.StatusForbidden
			}
			s.Logger.Warn("[Update] Update install blocked", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]string{"error": cleanUpdaterError(err)})
			return
		}

		s.Logger.Info("[Update] Update script started", "log", result.LogPath)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  result.Status,
			"message": result.Message,
		})
	}
}

func cleanUpdaterError(err error) string {
	msg := err.Error()
	for _, prefix := range []string{
		updater.ErrSelfUpdateDisabled.Error() + ": ",
		updater.ErrDockerRuntime.Error() + ": ",
		updater.ErrUnsupportedOS.Error() + ": ",
		updater.ErrMissingScript.Error() + ": ",
		updater.ErrMissingBash.Error() + ": ",
	} {
		if len(msg) >= len(prefix) && msg[:len(prefix)] == prefix {
			return msg[len(prefix):]
		}
	}
	return msg
}

package desktopstore

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// DefaultCompanionSettle is the minimum time between the last companion start
// and the companion state check in production.
const DefaultCompanionSettle = 2 * time.Second

// checkStartedCompanions inspects the companions an install or update just
// started, after the app is ready. Only a companion that Docker reports as
// exited or dead with a non-zero exit code fails the operation, which then
// cleans up or rolls back as for any other failure. Store companions run with
// the unless-stopped restart policy, so a crashing process shows up as
// "restarting"; slow or dependency-waiting companions can look the same, so
// restarts, unhealthy or starting health checks, exit code 0 and inspect
// errors are only logged.
func (s *Service) checkStartedCompanions(ctx context.Context, app InstalledApp, names []string, startedAt time.Time) error {
	if len(names) == 0 {
		return nil
	}
	if wait := s.cfg.CompanionSettle - time.Since(startedAt); wait > 0 {
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	for _, name := range names {
		state, err := s.requireDocker().InspectContainer(ctx, name)
		if err != nil {
			s.logger().Warn("Store companion state check failed", "app_id", app.AppID, "container", name, "error", err)
			continue
		}
		status := strings.ToLower(strings.TrimSpace(state.Status))
		if (status == "exited" || status == "dead") && state.ExitCode != 0 {
			return fmt.Errorf("companion container %s %s with exit code %d", name, status, state.ExitCode)
		}
		health := strings.ToLower(strings.TrimSpace(state.Health))
		if state.Restarting || status == "restarting" || state.RestartCount > 0 || health == "unhealthy" {
			s.logger().Warn("Store companion is not running steadily", "app_id", app.AppID, "container", name,
				"status", state.Status, "health", state.Health, "exit_code", state.ExitCode, "restart_count", state.RestartCount)
		}
	}
	return nil
}

func companionContainerNames(companions []CompanionApp) []string {
	names := make([]string, 0, len(companions))
	for _, companion := range companions {
		if name := strings.TrimSpace(companion.ContainerName); name != "" {
			names = append(names, name)
		}
	}
	return names
}

// replacedCompanionNames lists the companions this update created; companions
// it did not replace are never checked.
func replacedCompanionNames(replaced []replacedContainer) []string {
	var names []string
	for _, item := range replaced {
		if item.companionID != "" && item.created {
			names = append(names, item.name)
		}
	}
	return names
}

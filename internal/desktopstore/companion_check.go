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

// DefaultCompanionRecheck is the wait before a companion that looked exited
// with an error is inspected again in production.
const DefaultCompanionRecheck = 1500 * time.Millisecond

// checkStartedCompanions inspects the companions an install or update just
// started, after the app is ready. Only a companion that Docker reports as
// exited or dead with a non-zero exit code fails the operation, which then
// cleans up or rolls back as for any other failure. Store companions run with
// the unless-stopped restart policy, so a crashing process shows up as
// "restarting"; slow or dependency-waiting companions can look the same, so
// restarts, unhealthy or starting health checks, exit code 0 and inspect
// errors are only logged. Some engines (Podman) report a companion as exited
// while their restart policy brings it back, so an exited companion is
// inspected again after CompanionRecheck and fails the operation only when it
// is still exited with an error and its restart count has not moved.
func (s *Service) checkStartedCompanions(ctx context.Context, app InstalledApp, names []string, startedAt time.Time) error {
	if len(names) == 0 {
		return nil
	}
	if err := sleepContext(ctx, s.cfg.CompanionSettle-time.Since(startedAt)); err != nil {
		return err
	}
	for _, name := range names {
		state, err := s.requireDocker().InspectContainer(ctx, name)
		if err != nil {
			s.logger().Warn("Store companion state check failed", "app_id", app.AppID, "container", name, "error", err)
			continue
		}
		if exitedWithError(state) {
			if err := sleepContext(ctx, s.cfg.CompanionRecheck); err != nil {
				return err
			}
			again, err := s.requireDocker().InspectContainer(ctx, name)
			if err != nil {
				s.logger().Warn("Store companion state check failed", "app_id", app.AppID, "container", name, "error", err)
				continue
			}
			if exitedWithError(again) && again.RestartCount == state.RestartCount {
				status := strings.ToLower(strings.TrimSpace(again.Status))
				return fmt.Errorf("companion container %s %s with exit code %d", name, status, again.ExitCode)
			}
			s.logger().Warn("Store companion exited and was restarted", "app_id", app.AppID, "container", name,
				"status", again.Status, "exit_code", state.ExitCode, "restart_count", again.RestartCount)
			state = again
		}
		status := strings.ToLower(strings.TrimSpace(state.Status))
		health := strings.ToLower(strings.TrimSpace(state.Health))
		if state.Restarting || status == "restarting" || state.RestartCount > 0 || health == "unhealthy" {
			s.logger().Warn("Store companion is not running steadily", "app_id", app.AppID, "container", name,
				"status", state.Status, "health", state.Health, "exit_code", state.ExitCode, "restart_count", state.RestartCount)
		}
	}
	return nil
}

// exitedWithError reports a container Docker shows as exited or dead with a
// non-zero exit code.
func exitedWithError(state ContainerState) bool {
	status := strings.ToLower(strings.TrimSpace(state.Status))
	return (status == "exited" || status == "dead") && state.ExitCode != 0
}

// sleepContext waits for d, or returns ctx.Err() when ctx ends first.
func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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

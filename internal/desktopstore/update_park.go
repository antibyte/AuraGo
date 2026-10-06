package desktopstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// updateRollbackTimeout bounds the rollback of a failed update. The rollback
// and the save of the previous record run on a context detached from the
// operation, so a shutdown or the operation deadline cannot cut them short.
const updateRollbackTimeout = 2 * time.Minute

// replacedContainer tracks one container that an update replaces. The name is
// the same before and after the update.
type replacedContainer struct {
	name        string
	companionID string // empty for the app container
	parked      bool   // the previous container waits, stopped, under parkedContainerName(name)
	removed     bool   // the engine could not park it, so it was removed as before parking existed
	missing     bool   // no previous container existed under name or its parked name
	created     bool   // a replacement may exist under name (set before the create)
	wasRunning  bool   // the previous container was running before the update stopped it
}

// parkOutcome is the result of parkContainer.
type parkOutcome struct {
	parked     bool // a previous container now waits under the parked name, also one an interrupted update left there
	removed    bool // the container could not be parked and was removed instead
	wasRunning bool // the previous container was running
}

// parkContainer stops the container called name and renames it to its parked
// name, so a replacement can take the name while the previous container stays
// restorable. A stopped container holds no host ports and no network DNS
// entry, and keeps its ID, volumes, binds and networks.
//
// A container that does not stop, or an engine that cannot rename, falls back
// to removing the container as updates did before parking existed. Neither
// parked nor removed means there was nothing to park. Only parked containers
// that carry this app's Store labels are removed or adopted. On error the
// stopped container keeps its name.
func (s *Service) parkContainer(ctx context.Context, appID, name string) (parkOutcome, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return parkOutcome{}, nil
	}
	docker := s.requireDocker()
	parkedName := parkedContainerName(name)
	var out parkOutcome
	if state, err := docker.InspectContainer(ctx, name); err == nil {
		out.wasRunning = state.Running
	}
	var parkErr error
	if stopErr := docker.StopContainer(ctx, name); stopErr != nil && !errors.Is(stopErr, errContainerNotFound) {
		// A container that did not stop keeps its host ports, so its
		// replacement could not start.
		parkErr = fmt.Errorf("stop %s: %w", name, stopErr)
	} else {
		parkErr = docker.RenameContainer(ctx, name, parkedName)
		if errors.Is(parkErr, errContainerNameConflict) {
			// A parked container that an earlier update could not remove. The
			// container under name is the current one, so this app's leftover
			// is stale; a foreign container keeps the name.
			if found, ours := s.appContainer(ctx, appID, parkedName); found && ours {
				if err := docker.RemoveContainer(ctx, parkedName, true); err != nil {
					return out, fmt.Errorf("remove stale %s: %w", parkedName, err)
				}
				parkErr = docker.RenameContainer(ctx, name, parkedName)
			}
		}
		if parkErr == nil {
			out.parked = true
			return out, nil
		}
		if errors.Is(parkErr, errContainerNotFound) {
			if _, inspectErr := docker.InspectContainer(ctx, name); errors.Is(inspectErr, errContainerNotFound) {
				// Nothing under name. An interrupted update can leave this app's
				// previous container parked; restore that one if this update fails.
				if state, err := docker.InspectContainer(ctx, parkedName); err == nil && isAppStoreContainer(state.Labels, appID) {
					_ = docker.StopContainer(ctx, parkedName)
					return parkOutcome{parked: true, wasRunning: state.Running}, nil
				}
				return parkOutcome{}, nil
			}
		}
	}
	// The container does not stop or the engine cannot rename (an older
	// Podman, a proxy that refuses the endpoint, a 404 for a container that
	// exists). Remove the container as updates did before parking, so the
	// update itself keeps working.
	s.logger().Warn("Store update cannot park the previous container; removing it instead", "container", name, "error", parkErr)
	if err := docker.RemoveContainer(ctx, name, true); err != nil {
		return out, fmt.Errorf("remove old container %s: %w", name, err)
	}
	out.removed = true
	return out, nil
}

// appContainer inspects name and reports whether it exists and carries the
// Store labels of appID.
func (s *Service) appContainer(ctx context.Context, appID, name string) (found, ours bool) {
	state, err := s.requireDocker().InspectContainer(ctx, name)
	if err != nil {
		return !errors.Is(err, errContainerNotFound), false
	}
	return true, isAppStoreContainer(state.Labels, appID)
}

// isAppStoreContainer reports whether labels mark a container the Software
// Store created for appID (the app or one of its companions).
func isAppStoreContainer(labels map[string]string, appID string) bool {
	id := normalizeAppID(labels["aurago.desktop_store.app_id"])
	return id != "" && id == normalizeAppID(appID)
}

// parkForReplacement parks the container called name and records it in
// replaced, also when parking fails, so the rollback restarts it.
func (s *Service) parkForReplacement(ctx context.Context, appID string, replaced *[]replacedContainer, name, companionID string) error {
	out, err := s.parkContainer(ctx, appID, name)
	*replaced = append(*replaced, replacedContainer{
		name:        name,
		companionID: companionID,
		parked:      out.parked,
		removed:     out.removed,
		missing:     err == nil && !out.parked && !out.removed,
		wasRunning:  out.wasRunning,
	})
	return err
}

// replaceAutoCompanions parks each companion the update recreates, then creates
// and starts its replacement.
func (s *Service) replaceAutoCompanions(ctx context.Context, app *InstalledApp, companions []CompanionApp, replaced *[]replacedContainer) error {
	for _, companion := range companions {
		index := companionIndex(app.Companions, companion.ID)
		if index < 0 {
			app.Companions = append(app.Companions, companion)
			index = len(app.Companions) - 1
		} else {
			app.Companions[index] = companion
		}
		if err := s.parkForReplacement(ctx, app.AppID, replaced, companion.ContainerName, companion.ID); err != nil {
			return err
		}
		// Set before the create: a replacement whose start and own removal
		// failed still holds the name, and the rollback must remove it before
		// the parked container gets the name back. A missing one is fine.
		(*replaced)[len(*replaced)-1].created = true
		if err := s.createCompanionAt(ctx, app, index); err != nil {
			return err
		}
	}
	return nil
}

// restoreReplaced puts the previous containers back after a failed update. It
// removes the replacements first, so the previous containers get their names
// and host ports back, then renames parked containers back and starts each one
// that was running or belongs to a running app. Containers that could not be
// parked are recreated from the previous record, as before parking existed.
func (s *Service) restoreReplaced(ctx context.Context, previous *InstalledApp, previousSpec ContainerSpec, previousWasRunning bool, replaced []replacedContainer) error {
	docker := s.requireDocker()
	for i := len(replaced) - 1; i >= 0; i-- {
		if !replaced[i].created {
			continue
		}
		_ = docker.StopContainer(ctx, replaced[i].name)
		if err := docker.RemoveContainer(ctx, replaced[i].name, true); err != nil {
			return fmt.Errorf("remove replacement %s: %w; the previous containers stay parked as <name>.prev", replaced[i].name, err)
		}
	}
	var errs joinedErrors
	for _, item := range replaced {
		start := previousWasRunning || item.wasRunning
		switch {
		case item.parked:
			parkedName := parkedContainerName(item.name)
			if err := docker.RenameContainer(ctx, parkedName, item.name); err != nil {
				errs = append(errs, fmt.Errorf("rename %s back to %s (the previous container is kept as %s): %w", parkedName, item.name, parkedName, err))
				continue
			}
			if start {
				if err := docker.StartContainer(ctx, item.name); err != nil {
					errs = append(errs, fmt.Errorf("start restored %s: %w", item.name, err))
				}
			}
		case item.removed || item.missing:
			if err := s.recreatePrevious(ctx, previous, previousSpec, previousWasRunning, item); err != nil {
				errs = append(errs, err)
			}
		default:
			// Parking failed before anything changed: the stopped previous
			// container still has its name.
			if start {
				if err := docker.StartContainer(ctx, item.name); err != nil {
					errs = append(errs, fmt.Errorf("restart previous %s: %w", item.name, err))
				}
			}
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// joinedErrors joins errors on one line ("; "): the rollback error ends up in
// InstalledApp.Error, which the Store window shows. errors.Is and errors.As
// still see every error.
type joinedErrors []error

func (e joinedErrors) Error() string {
	parts := make([]string, 0, len(e))
	for _, err := range e {
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, "; ")
}

func (e joinedErrors) Unwrap() []error { return e }

// recreatePrevious recreates one previous container from the previous record:
// the rollback for engines that cannot rename and for containers that were
// already missing.
func (s *Service) recreatePrevious(ctx context.Context, previous *InstalledApp, previousSpec ContainerSpec, previousWasRunning bool, item replacedContainer) error {
	docker := s.requireDocker()
	if item.companionID == "" {
		id, err := docker.CreateContainer(ctx, previousSpec)
		if err != nil {
			return fmt.Errorf("recreate %s: %w", item.name, err)
		}
		previous.ContainerID = id
		if previousWasRunning {
			if err := docker.StartContainer(ctx, previous.ContainerName); err != nil {
				return fmt.Errorf("start recreated %s: %w", item.name, err)
			}
		}
		return nil
	}
	index := companionIndex(previous.Companions, item.companionID)
	if index < 0 {
		// The update added this companion; removing the replacement is enough.
		return nil
	}
	if err := s.createCompanionAt(ctx, previous, index); err != nil {
		return err
	}
	if !previousWasRunning {
		_ = docker.StopContainer(ctx, previous.Companions[index].ContainerName)
		previous.Companions[index].Status = AppStatusStopped
	}
	return nil
}

// removeParked removes the parked previous containers after a successful
// update. A failure only leaves a stopped leftover, which the next update or
// the uninstall removes.
func (s *Service) removeParked(ctx context.Context, appID string, replaced []replacedContainer) {
	for _, item := range replaced {
		if !item.parked {
			continue
		}
		if err := s.removeParkedContainer(ctx, appID, item.name); err != nil {
			s.logger().Warn("Store update could not remove the previous container; the next update or uninstall removes it", "container", parkedContainerName(item.name), "error", err)
		}
	}
}

// removeParkedContainer removes the parked container of name when it carries
// appID's Store labels. A missing one is fine; another container with that
// name is left in place.
func (s *Service) removeParkedContainer(ctx context.Context, appID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	parkedName := parkedContainerName(name)
	state, err := s.requireDocker().InspectContainer(ctx, parkedName)
	if errors.Is(err, errContainerNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !isAppStoreContainer(state.Labels, appID) {
		return fmt.Errorf("%s is not a Software Store container of %s; it is left in place", parkedName, appID)
	}
	return s.requireDocker().RemoveContainer(ctx, parkedName, true)
}

// appWasReplaced reports whether the update reached the app container.
func appWasReplaced(replaced []replacedContainer) bool {
	for _, item := range replaced {
		if item.companionID == "" {
			return true
		}
	}
	return false
}

package desktopstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// replacedContainer tracks one container that an update replaces. The name is
// the same before and after the update.
type replacedContainer struct {
	name        string
	companionID string // empty for the app container
	parked      bool   // the previous container waits, stopped, under parkedContainerName(name)
	removed     bool   // the engine could not rename it, so it was removed as before parking existed
	missing     bool   // no previous container existed under name or its parked name
	created     bool   // the replacement exists under name
}

// parkContainer stops the container called name and renames it to its parked
// name, so a replacement can take the name while the previous container stays
// restorable. A stopped container holds no host ports and no network DNS
// entry, and keeps its ID, volumes, binds and networks.
//
// parked reports that a previous container now waits under the parked name,
// including one an interrupted update left there. removed reports that the
// engine cannot rename and the container was removed instead. Neither means
// there was nothing to park. On error the stopped container keeps its name.
func (s *Service) parkContainer(ctx context.Context, name string) (parked, removed bool, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, false, nil
	}
	docker := s.requireDocker()
	parkedName := parkedContainerName(name)
	_ = docker.StopContainer(ctx, name)
	renameErr := docker.RenameContainer(ctx, name, parkedName)
	if errors.Is(renameErr, errContainerNameConflict) {
		// A parked container that an earlier update could not remove. The
		// container under name is the current one, so the leftover is stale.
		if err := docker.RemoveContainer(ctx, parkedName, true); err != nil {
			return false, false, fmt.Errorf("remove stale %s: %w", parkedName, err)
		}
		renameErr = docker.RenameContainer(ctx, name, parkedName)
	}
	if renameErr == nil {
		return true, false, nil
	}
	if errors.Is(renameErr, errContainerNotFound) {
		if _, inspectErr := docker.InspectContainer(ctx, name); errors.Is(inspectErr, errContainerNotFound) {
			// Nothing under name. An interrupted update can leave the previous
			// container parked; restore that one if this update fails.
			if _, parkedErr := docker.InspectContainer(ctx, parkedName); parkedErr == nil {
				return true, false, nil
			}
			return false, false, nil
		}
	}
	// The engine cannot rename (an older Podman, a proxy that refuses the
	// endpoint, a 404 for a container that exists). Remove the container as
	// updates did before parking, so the update itself keeps working.
	s.logger().Warn("Store update cannot park the previous container; removing it instead", "container", name, "error", renameErr)
	if err := docker.RemoveContainer(ctx, name, true); err != nil {
		return false, false, fmt.Errorf("remove old container %s: %w", name, err)
	}
	return false, true, nil
}

// parkForReplacement parks the container called name and records it in
// replaced, also when parking fails, so the rollback restarts it.
func (s *Service) parkForReplacement(ctx context.Context, replaced *[]replacedContainer, name, companionID string) error {
	parked, removed, err := s.parkContainer(ctx, name)
	*replaced = append(*replaced, replacedContainer{
		name:        name,
		companionID: companionID,
		parked:      parked,
		removed:     removed,
		missing:     err == nil && !parked && !removed,
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
		if err := s.parkForReplacement(ctx, replaced, companion.ContainerName, companion.ID); err != nil {
			return err
		}
		if err := s.createCompanionAt(ctx, app, index); err != nil {
			return err
		}
		(*replaced)[len(*replaced)-1].created = true
	}
	return nil
}

// restoreReplaced puts the previous containers back after a failed update. It
// removes the replacements first, so the previous containers get their names
// and host ports back, then renames parked containers back and starts them when
// the app was running. Containers that could not be parked are recreated from
// the previous record, as before parking existed.
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
	var errs []error
	for _, item := range replaced {
		switch {
		case item.parked:
			parkedName := parkedContainerName(item.name)
			if err := docker.RenameContainer(ctx, parkedName, item.name); err != nil {
				errs = append(errs, fmt.Errorf("rename %s back to %s (the previous container is kept as %s): %w", parkedName, item.name, parkedName, err))
				continue
			}
			if previousWasRunning {
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
			if previousWasRunning {
				if err := docker.StartContainer(ctx, item.name); err != nil {
					errs = append(errs, fmt.Errorf("restart previous %s: %w", item.name, err))
				}
			}
		}
	}
	return errors.Join(errs...)
}

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
func (s *Service) removeParked(ctx context.Context, replaced []replacedContainer) {
	for _, item := range replaced {
		if !item.parked {
			continue
		}
		parkedName := parkedContainerName(item.name)
		if err := s.requireDocker().RemoveContainer(ctx, parkedName, true); err != nil {
			s.logger().Warn("Store update could not remove the previous container; the next update or uninstall removes it", "container", parkedName, "error", err)
		}
	}
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

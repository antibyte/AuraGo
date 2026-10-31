package desktop

import (
	"context"
	"fmt"
	"sync"
)

// EnsureContainer returns the ID verified by this acquisition; callers must not
// rediscover a possibly different container before executing their operation.
func (s *CodeContainerService) EnsureContainer(ctx context.Context) (string, error) {
	if !s.cfg.CodeStudio.Enabled {
		return "", fmt.Errorf("code studio is disabled")
	}
	s.lifecycleMu.RLock()
	id, err := s.runningContainerLocked(ctx)
	s.lifecycleMu.RUnlock()
	if err != nil || id != "" {
		return id, err
	}
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	// Another caller may have completed startup while we waited.
	if id, err = s.runningContainerLocked(ctx); err != nil || id != "" {
		return id, err
	}
	if err = s.startContainerLocked(ctx); err != nil {
		return "", err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.containerID, nil
}

func (s *CodeContainerService) runningContainerLocked(ctx context.Context) (string, error) {
	s.mu.RLock()
	id, running := s.containerID, s.state == StateRunning
	s.mu.RUnlock()
	if !running || id == "" {
		return "", nil
	}
	inspect, err := s.docker.InspectContainer(ctx, id)
	if err != nil {
		// Only a successful discovery can distinguish removal from an outage.
		containers, listErr := s.docker.ListContainers(ctx, true)
		if listErr == nil {
			found := false
			for _, container := range containers {
				if container.ID == id {
					found = true
					break
				}
			}
			if !found {
				s.setState(StateStopped)
				return "", nil
			}
		}
		s.setState(StateError)
		return "", fmt.Errorf("inspect code studio container: %w", err)
	}
	if !inspect.State.Running {
		return "", nil
	}
	s.mu.Lock()
	s.touchLocked()
	s.mu.Unlock()
	return id, nil
}

func (s *CodeContainerService) setState(state ContainerState) {
	s.mu.Lock()
	s.state = state
	if state != StateRunning {
		s.containerID = ""
	}
	s.mu.Unlock()
}

func (s *CodeContainerService) markRunning(id string) {
	s.mu.Lock()
	s.containerID, s.state = id, StateRunning
	s.touchLocked()
	s.mu.Unlock()
}

// HoldTerminal prevents idle shutdown until the returned idempotent release.
func (s *CodeContainerService) HoldTerminal(id string) (func(), error) {
	s.lifecycleMu.RLock()
	defer s.lifecycleMu.RUnlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != StateRunning || s.containerID != id {
		return nil, fmt.Errorf("code studio container changed")
	}
	s.activeSessions++
	s.touchLocked()
	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			s.activeSessions--
			s.touchLocked()
		})
	}, nil
}

// generation == 0 is an explicit stop. Idle callbacks must still own the timer.
func (s *CodeContainerService) stopContainerLocked(ctx context.Context, generation uint64) error {
	s.mu.Lock()
	if generation != 0 && (generation != s.timerGeneration || s.activeSessions > 0) {
		s.mu.Unlock()
		return nil
	}
	s.timerGeneration++
	if s.stopTimer != nil {
		s.stopTimer.Stop()
		s.stopTimer = nil
	}
	id, running := s.containerID, s.state == StateRunning
	s.mu.Unlock()
	if !running {
		return nil
	}
	if id == "" {
		id = codeContainerName
	}
	if err := s.docker.ContainerAction(ctx, id, "stop"); err != nil {
		return fmt.Errorf("stop code studio container: %w", err)
	}
	s.setState(StateStopped)
	return nil
}

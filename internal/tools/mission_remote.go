package tools

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// MissionRunner defines where a mission is executed.
type MissionRunner string

const (
	MissionRunnerLocal  MissionRunner = "local"
	MissionRunnerRemote MissionRunner = "remote"
)

const (
	RemoteSyncPending = "pending"
	RemoteSyncSynced  = "synced"
	RemoteSyncError   = "error"
)

// RemoteMissionClient delivers mission definitions and commands to remote eggs.
type RemoteMissionClient interface {
	SyncMission(ctx context.Context, mission MissionV2, promptSnapshot string) error
	DeleteMission(ctx context.Context, mission MissionV2) error
	RunMission(ctx context.Context, mission MissionV2, triggerType, triggerData string) error
}

func normalizeMissionRunner(r MissionRunner) MissionRunner {
	if strings.TrimSpace(string(r)) == "" {
		return MissionRunnerLocal
	}
	return r
}

func isRemoteMission(m *MissionV2) bool {
	if m == nil {
		return false
	}
	return normalizeMissionRunner(m.RunnerType) == MissionRunnerRemote
}

func validateRemoteMission(m MissionV2) error {
	if normalizeMissionRunner(m.RunnerType) != MissionRunnerRemote {
		return nil
	}
	if strings.TrimSpace(m.RemoteNestID) == "" {
		return fmt.Errorf("remote_nest_id is required for remote missions")
	}
	if strings.TrimSpace(m.RemoteEggID) == "" {
		return fmt.Errorf("remote_egg_id is required for remote missions")
	}
	if !RemoteTriggerAllowed(m.ExecutionType, m.TriggerType) {
		return fmt.Errorf("trigger %q is not supported for remote missions", m.TriggerType)
	}
	return nil
}

// RemoteTriggerAllowed reports whether a mission execution/trigger type can be
// evaluated by an egg without relying on master-side event sources.
func RemoteTriggerAllowed(exec ExecutionType, trig TriggerType) bool {
	switch exec {
	case ExecutionManual, ExecutionScheduled:
		return true
	case ExecutionTriggered:
		switch trig {
		case TriggerSystemStartup, TriggerMQTTMessage, TriggerHomeAssistantState:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func newRemoteRevision() string {
	return fmt.Sprintf("rev_%d", time.Now().UTC().UnixNano())
}

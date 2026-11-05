package server

import (
	"context"

	"aurago/internal/config"
	"aurago/internal/flows"
	"aurago/internal/tools"
)

var _ flows.DurableAuthority = flowMissionBridge{}

// DurableEnabled reads the current saved switch. The core separately requires
// its immutable boot-time enablement before admitting any durable work.
func (b flowMissionBridge) DurableEnabled(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if b.s == nil {
		return false, flows.ErrDurableUnavailable
	}
	cfg := b.s.ConfigSnapshot()
	if cfg == nil {
		return false, flows.ErrDurableUnavailable
	}
	return cfg.Flows.DurableEnabled, nil
}

// DurableAllowed checks the current global policy without opening a Vault key.
func (b flowMissionBridge) DurableAllowed(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.s == nil {
		return flows.ErrDurableUnavailable
	}
	cfg := b.s.ConfigSnapshot()
	if !durableAllowedConfig(cfg) {
		return flows.ErrDurableUnavailable
	}
	return nil
}

func durableAllowedConfig(cfg *config.Config) bool {
	return cfg != nil && cfg.Flows.Enabled && cfg.Tools.Missions.Enabled && !cfg.Tools.Missions.ReadOnly && !cfg.VirtualDesktop.ReadOnly
}

// AuthorizeDurable checks current server and mission policy without taking a
// flow or store lock. The core separately verifies the stored revision/hash.
func (b flowMissionBridge) AuthorizeDurable(ctx context.Context, flowID, missionID string, revision int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.s == nil || flowID == "" || missionID == "" || revision <= 0 {
		return flows.ErrDurableUnavailable
	}
	cfg := b.s.ConfigSnapshot()
	if !durableAllowedConfig(cfg) || !cfg.Flows.DurableEnabled {
		return flows.ErrDurableUnavailable
	}
	manager := b.s.MissionManagerV2
	if manager == nil {
		return flows.ErrDurableUnavailable
	}
	mission, ok := manager.Get(missionID)
	if !ok || mission.ExecutionType != tools.ExecutionFlow || mission.FlowID != flowID || !mission.Enabled || !mission.FlowPublished {
		return flows.ErrDurableUnavailable
	}
	return ctx.Err()
}

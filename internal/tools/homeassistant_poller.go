package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"
)

// StartHomeAssistantPoller loops and checks HA states to fire Mission triggers.
func StartHomeAssistantPoller(ctx context.Context, cfg HAConfig, m *MissionManagerV2, logger *slog.Logger) {
	if cfg.URL == "" || cfg.AccessToken == "" {
		logger.Debug("Home Assistant poller not started: missing config URL or Token")
		return
	}

	logger.Info("Starting Home Assistant state poller for Mission Control triggers")

	// Track previous state for trigger edges
	lastStateMap := make(map[string]string)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("Home Assistant poller shutting down")
			return
		case <-ticker.C:
			// Fetch states only for monitored entities
			monitoredEntities := make(map[string]bool)
			for _, mission := range m.List() {
				if mission.ExecutionType == ExecutionTriggered && mission.TriggerType == TriggerHomeAssistantState && mission.Enabled {
					if mission.TriggerConfig != nil && mission.TriggerConfig.HAEntityID != "" {
						monitoredEntities[mission.TriggerConfig.HAEntityID] = true
					}
				}
			}

			if len(monitoredEntities) == 0 {
				// No active entities, skip polling
				continue
			}

			for entityID := range monitoredEntities {
				data, code, err := haRequest(cfg, "GET", "/api/states/"+entityID, "")
				if err != nil {
					logger.Debug("HA poller request failed for entity", "entity", entityID, "error", err)
					continue
				}
				if code != 200 {
					logger.Debug("HA poller bad status code for entity", "entity", entityID, "code", code)
					continue
				}

				var stateObj map[string]interface{}
				if err := json.Unmarshal(data, &stateObj); err != nil {
					continue
				}

				state, ok := stateObj["state"].(string)
				if !ok {
					continue
				}

				oldState, known := lastStateMap[entityID]
				if !known {
					// First run: just initialize the state map
					lastStateMap[entityID] = state
					continue
				}

				if oldState != state {
					// State changed! Notify mission manager
					lastStateMap[entityID] = state
					m.NotifyHomeAssistantEvent(entityID, state, oldState)
				}
			}
		}
	}
}

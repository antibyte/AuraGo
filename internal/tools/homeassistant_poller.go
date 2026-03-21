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
			// Fetch all states
			data, code, err := haRequest(cfg, "GET", "/api/states", "")
			if err != nil {
				logger.Debug("HA poller request failed", "error", err)
				continue
			}
			if code != 200 {
				logger.Debug("HA poller bad status code", "code", code)
				continue
			}

			var states []map[string]interface{}
			if err := json.Unmarshal(data, &states); err != nil {
				continue
			}

			// Evaluate changes
			for _, s := range states {
				entityID, ok1 := s["entity_id"].(string)
				state, ok2 := s["state"].(string)
				if !ok1 || !ok2 {
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

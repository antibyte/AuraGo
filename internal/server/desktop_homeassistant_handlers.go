package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/tools"
)

type desktopHAEntity struct {
	EntityID string `json:"entity_id"`
	Name     string `json:"friendly_name"`
	State    string `json:"state"`
}

func desktopHAStates(ctx context.Context, cfg tools.HAConfig) ([]desktopHAEntity, bool) {
	var result struct {
		Status string            `json:"status"`
		States []desktopHAEntity `json:"states"`
	}
	if json.Unmarshal([]byte(tools.HAGetStatesContext(ctx, cfg, "switch")), &result) != nil || result.Status != "success" {
		return nil, false
	}
	entities := make([]desktopHAEntity, 0, len(result.States))
	for _, entity := range result.States {
		if !desktop.ValidHASwitchEntityID(entity.EntityID) {
			continue
		}
		switch entity.State {
		case "on", "off", "unavailable":
		default:
			entity.State = "unknown"
		}
		if entity.Name == "" {
			entity.Name = entity.EntityID
		}
		entities = append(entities, entity)
	}
	sort.Slice(entities, func(i, j int) bool { return entities[i].EntityID < entities[j].EntityID })
	return entities, true
}

func handleDesktopHomeAssistant(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		catalog := r.URL.Path == "/api/desktop/home-assistant/entities" && r.Method == http.MethodGet
		states := r.URL.Path == "/api/desktop/home-assistant/states" && r.Method == http.MethodGet
		write := r.URL.Path == "/api/desktop/home-assistant/switch" && r.Method == http.MethodPost
		if !catalog && !states && !write {
			jsonError(w, "method_not_allowed", http.StatusMethodNotAllowed)
			return
		}
		if !requireDesktopPermission(s, w, r, desktopScopeAdmin) {
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		svc, _, err := s.getDesktopService(r.Context())
		if err != nil {
			jsonError(w, "desktop_unavailable", http.StatusServiceUnavailable)
			return
		}
		board, err := svc.HASwitchboard(r.Context())
		if err != nil {
			jsonError(w, "board_unavailable", http.StatusInternalServerError)
			return
		}
		s.CfgMu.RLock()
		ha := s.Cfg.HomeAssistant
		cfg := tools.HAConfig{URL: ha.URL, AccessToken: ha.AccessToken, ReadOnly: ha.ReadOnly,
			AllowedServices: append([]string(nil), ha.AllowedServices...), BlockedServices: append([]string(nil), ha.BlockedServices...)}
		s.CfgMu.RUnlock()
		ready := ha.Enabled && strings.TrimSpace(ha.URL) != "" && ha.AccessToken != ""
		readOnly := svc.Config().ReadOnly || cfg.ReadOnly
		canOn := ready && !readOnly && tools.HAServiceAllowed(cfg, "switch", "turn_on")
		canOff := ready && !readOnly && tools.HAServiceAllowed(cfg, "switch", "turn_off")
		payload := map[string]interface{}{"ready": ready, "readonly": readOnly, "board_readonly": svc.Config().ReadOnly, "can_on": canOn, "can_off": canOff,
			"board": board, "entities": []desktopHAEntity{}}
		w.Header().Set("Content-Type", "application/json")
		if !ready {
			if write {
				jsonError(w, "setup_required", http.StatusServiceUnavailable)
			} else {
				payload["issue"] = "setup_required"
				_ = json.NewEncoder(w).Encode(payload)
			}
			return
		}
		selected := make(map[string]bool, len(board.Switches))
		for _, entry := range board.Switches {
			selected[entry.EntityID] = true
		}
		var command struct {
			EntityID string `json:"entity_id"`
			State    string `json:"state"`
		}
		if write {
			dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
			dec.DisallowUnknownFields()
			if dec.Decode(&command) != nil || dec.Decode(new(interface{})) != io.EOF || !desktop.ValidHASwitchEntityID(command.EntityID) || !selected[command.EntityID] || (command.State != "on" && command.State != "off") {
				jsonError(w, "invalid_switch", http.StatusBadRequest)
				return
			}
			if (command.State == "on" && !canOn) || (command.State == "off" && !canOff) {
				jsonError(w, "switch_not_allowed", http.StatusForbidden)
				return
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		entities, ok := desktopHAStates(ctx, cfg)
		if !ok {
			jsonError(w, "ha_unavailable", http.StatusBadGateway)
			return
		}
		if write {
			available := false
			for _, entity := range entities {
				if entity.EntityID == command.EntityID {
					available = entity.State == "on" || entity.State == "off"
					break
				}
			}
			if !available {
				jsonError(w, "switch_unavailable", http.StatusConflict)
				return
			}
			var result struct {
				Status string `json:"status"`
			}
			response := tools.HACallServiceContext(ctx, cfg, "switch", "turn_"+command.State, command.EntityID, nil)
			if json.Unmarshal([]byte(response), &result) != nil || result.Status != "success" {
				jsonError(w, "switch_failed", http.StatusBadGateway)
				return
			}
			_ = svc.Audit(r.Context(), "ha_switch", command.EntityID, map[string]interface{}{"state": command.State}, desktop.SourceUser)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"accepted": true})
			return
		}
		if !catalog {
			filtered := make([]desktopHAEntity, 0, len(selected))
			for _, entity := range entities {
				if selected[entity.EntityID] {
					filtered = append(filtered, entity)
				}
			}
			entities = filtered
		}
		payload["entities"] = entities
		payload["checked_at"] = time.Now().UTC().Format(time.RFC3339)
		_ = json.NewEncoder(w).Encode(payload)
	}
}

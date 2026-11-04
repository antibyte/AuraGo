package agent

import (
	"reflect"
	"slices"
	"sort"
	"strings"

	"aurago/internal/config"
)

// MQTTRelayAllowedTools narrows discovery; dispatch separately checks every grant.
func MQTTRelayAllowedTools(grants []config.ActionPolicyRule) []string {
	set := make(map[string]bool)
	for _, grant := range grants {
		set[grant.Tool] = true
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func checkMQTTRelayAction(tc ToolCall, dc *DispatchContext, action ActionDescriptor) *ToolDispatchResult {
	if dc == nil || dc.MessageSource != "mqtt" && dc.MessageSource != "frigate" {
		return nil
	}
	deny := func() *ToolDispatchResult {
		return actionPolicyRefusal("mqtt_relay_grant_required", "MQTT relay action requires a current explicit tool, operation and target grant.", ToolResultDenied)
	}
	if dc.Cfg == nil || dc.MQTTRelayTopic == "" {
		return deny()
	}
	current := dc.Cfg
	if resolver := dc.Cfg.AuthorizationSnapshots; resolver != nil {
		_, current = resolver()
		if current == nil {
			return deny()
		}
	}
	for _, cfg := range []*config.Config{dc.Cfg, current} {
		if config.MQTTRelayPauseReason(cfg) != "" || !config.MQTTRelayTopicAllowed(cfg, dc.MQTTRelayTopic) {
			return deny()
		}
	}
	action, ok := mqttRelayAction(tc, action)
	if !ok || action.targetUnknown {
		return deny()
	}
	for _, cfg := range []*config.Config{dc.Cfg, current} {
		if !mqttRelayHardAllowed(cfg, dc.Cfg, tc, action) {
			return deny()
		}
		if config.ValidateMQTTRelayGrants(cfg.MQTT.RelayGrants) != nil || !actionAllowRulesCover(cfg.MQTT.RelayGrants, action) {
			return deny()
		}
	}
	return nil
}

func mqttRelayHardAllowed(cfg, captured *config.Config, tc ToolCall, action ActionDescriptor) bool {
	if action.Mutating && catalogReadOnly(cfg, &ToolCatalogEntry{Name: tc.Action}) {
		return false
	}
	if tc.Action == "home_assistant" {
		if !cfg.HomeAssistant.Enabled || cfg.HomeAssistant.URL != captured.HomeAssistant.URL || cfg.HomeAssistant.AccessToken != captured.HomeAssistant.AccessToken {
			return false
		}
		req := decodeHomeAssistantArgs(tc)
		return !action.Mutating || homeAssistantServiceGate(req.Domain, req.Service, cfg.HomeAssistant.AllowedServices, cfg.HomeAssistant.BlockedServices) == ""
	}
	if tc.Action != "send_notification" {
		return true
	}
	channel := strings.ToLower(strings.TrimSpace(decodeNotificationArgs(tc).Channel))
	switch channel {
	case "ntfy":
		return cfg.Notifications.Ntfy.Enabled && reflect.DeepEqual(cfg.Notifications.Ntfy, captured.Notifications.Ntfy)
	case "pushover":
		return cfg.Notifications.Pushover.Enabled && reflect.DeepEqual(cfg.Notifications.Pushover, captured.Notifications.Pushover)
	case "telegram":
		return !cfg.Telegram.ReadOnly && cfg.Telegram.BotToken != "" && cfg.Telegram.UserID != 0 && reflect.DeepEqual(cfg.Telegram, captured.Telegram)
	case "whatsapp":
		return cfg.WhatsApp.Enabled && !cfg.WhatsApp.ReadOnly && reflect.DeepEqual(cfg.WhatsApp, captured.WhatsApp)
	case "discord":
		return cfg.Discord.Enabled && !cfg.Discord.ReadOnly && reflect.DeepEqual(cfg.Discord, captured.Discord)
	case "telnyx":
		return cfg.Telnyx.Enabled && !cfg.Telnyx.ReadOnly && reflect.DeepEqual(cfg.Telnyx, captured.Telnyx)
	case "cyd":
		return cfg.Cyd.Enabled && reflect.DeepEqual(cfg.Cyd, captured.Cyd)
	case "push":
		return true
	}
	return false
}

// mqttRelayAction resolves execution arguments, never model-written target labels.
func mqttRelayAction(tc ToolCall, action ActionDescriptor) (ActionDescriptor, bool) {
	switch tc.Action {
	case "mqtt_publish", "mqtt_subscribe", "mqtt_unsubscribe", "mqtt_get_messages":
		return action, !action.targetUnknown
	case "send_notification":
		channel := strings.ToLower(strings.TrimSpace(decodeNotificationArgs(tc).Channel))
		if channel == "" || channel == "all" || strings.ContainsAny(channel, ", /\\") {
			return action, false
		}
		action.targets = []string{"notification:" + channel}
	case "home_assistant":
		req := decodeHomeAssistantArgs(tc)
		operation := action.Operation
		if operation == "state" {
			operation = "get_state"
		}
		if operation == "service" {
			operation = "call_service"
		}
		if operation != "get_state" && operation != "call_service" {
			return action, false
		}
		// These selectors can broaden a service call beyond its explicit entity.
		for _, key := range []string{"entity_id", "area_id", "device_id", "floor_id", "label_id", "target"} {
			if _, exists := req.ServiceData[key]; exists {
				return action, false
			}
		}
		service := req.Domain + "." + req.Service
		if strings.Contains(req.Service, ".") {
			// The execution API passes domain and service separately.
			return action, false
		}
		action.Operation = operation
		action.targets = nil
		for _, entity := range strings.Split(req.EntityID, ",") {
			entity = strings.TrimSpace(entity)
			target := "ha:" + entity
			if operation == "call_service" {
				target += "/" + service
			}
			probe := config.ActionPolicyRule{Tool: "home_assistant", Operation: operation, Target: target}
			if config.ValidateMQTTRelayGrants([]config.ActionPolicyRule{probe}) != nil {
				return action, false
			}
			action.targets = append(action.targets, target)
		}
	default:
		return action, false
	}
	sort.Strings(action.targets)
	action.targets = slices.Compact(action.targets)
	action.Target = actionTargetSummary(action.targets)
	action.targetUnknown = false
	return action, true
}

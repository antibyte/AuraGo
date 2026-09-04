package agent

import (
	"encoding/json"
	"strings"

	"aurago/internal/cyd"
)

func handleCYDDisplay(dc *DispatchContext, req cydDisplayArgs) string {
	encode := func(v map[string]interface{}) string {
		b, _ := json.Marshal(v)
		return string(b)
	}
	if dc == nil || dc.Cfg == nil || !dc.Cfg.Cyd.Enabled {
		return encode(map[string]interface{}{"status": "error", "message": "cyd is not enabled in config"})
	}
	hub := cyd.Global()
	if hub == nil {
		return encode(map[string]interface{}{"status": "error", "message": "cyd hub is not started"})
	}
	op := strings.ToLower(strings.TrimSpace(req.Operation))
	switch op {
	case "notify":
		if strings.TrimSpace(req.Message) == "" {
			return encode(map[string]interface{}{"status": "error", "message": "message is required"})
		}
		n := hub.Notify(req.Title, req.Message, req.Priority, req.TTL)
		return encode(map[string]interface{}{"status": "ok", "notify": n})
	case "show":
		if strings.TrimSpace(req.Message) == "" {
			return encode(map[string]interface{}{"status": "error", "message": "message is required"})
		}
		hub.PinTask(req.Message, req.TTL)
		return encode(map[string]interface{}{"status": "ok", "task": req.Message})
	case "clear":
		hub.Clear()
		return encode(map[string]interface{}{"status": "ok"})
	case "page", "brightness", "led":
		if !dc.Cfg.Cyd.AllowAgentControl {
			return encode(map[string]interface{}{"status": "error", "message": "cyd.allow_agent_control is disabled"})
		}
		switch op {
		case "page":
			hub.SetPage(req.Page)
		case "brightness":
			hub.SetBrightness(req.Brightness)
		case "led":
			hub.SetLED(req.LED)
		}
		return encode(map[string]interface{}{"status": "ok"})
	case "status":
		return encode(map[string]interface{}{
			"status":     "ok",
			"devices":    hub.Devices(),
			"ws_clients": hub.ClientCount(),
			"snapshot":   hub.Snapshot(),
		})
	default:
		return encode(map[string]interface{}{"status": "error", "message": "unknown operation"})
	}
}

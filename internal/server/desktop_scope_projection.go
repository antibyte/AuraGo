package server

import (
	"net/http"

	"aurago/internal/desktop"
)

func desktopRequestIsAdmin(s *Server, r *http.Request) bool {
	if token, bearer := bearerCredential(r.Header.Get("Authorization")); bearer {
		return desktopTokenHasScope(s, token, desktopScopeAdmin)
	}
	// The caller has already authenticated the administrator browser session.
	return true
}

// HTTP and WebSocket bootstrap share exactly the same scope projection.
func filterDesktopBootstrap(s *Server, r *http.Request, payload desktop.BootstrapPayload) desktop.BootstrapPayload {
	if desktopRequestIsAdmin(s, r) {
		return payload
	}
	payload.Settings = nil
	payload.AllWidgets = nil
	payload.Providers = nil
	payload.AllowAgentControl = false
	payload.AllowGeneratedApps = false
	payload.AllowPythonJobs = false
	payload.SerialBrowserEnabled = false
	payload.SerialHostEnabled = false
	if token, _ := bearerCredential(r.Header.Get("Authorization")); !desktopTokenHasScope(s, token, desktopScopeWrite) {
		payload.ReadOnly = true
		payload.RetroNetEnabled = false
	}
	return payload
}

func filterDesktopEvent(s *Server, r *http.Request, event desktop.Event) (desktop.Event, bool) {
	if desktopRequestIsAdmin(s, r) {
		return event, true
	}
	if event.Type == "desktop_policy" {
		return event, true
	}
	if event.Type != "desktop_changed" {
		return desktop.Event{}, false
	}
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		return desktop.Event{}, false
	}
	operation, _ := payload["operation"].(string)
	switch operation {
	case "write_file", "move_path", "delete_path", "upload_file", "create_directory", "copy_path", "write_document", "write_workbook", "trash_move":
		projected := map[string]interface{}{"operation": operation}
		for _, key := range []string{"path", "old_path", "new_path", "source_path", "dest_path"} {
			if value, ok := payload[key].(string); ok {
				projected[key] = value
			}
		}
		event.Payload = projected
		return event, true
	default:
		return desktop.Event{}, false
	}
}

package agent

import "encoding/json"

// These are hard in-process dependencies already required by the handlers.
// This check performs no network probe and does not infer remote health.
func toolRuntimeUnavailable(dc *DispatchContext, name string) string {
	if dc == nil {
		return ""
	}
	switch name {
	case "manage_notes", "notes", "todo", "manage_journal", "journal", "manage_plan":
		if dc.ShortTermMem == nil {
			return "Memory storage not available."
		}
	case "address_book":
		if dc.ContactsDB == nil {
			return "Contacts database not available."
		}
	case "manage_todos", "manage_appointments":
		if dc.PlannerDB == nil {
			return "Planner database not available."
		}
	case "cheatsheet":
		if dc.CheatsheetDB == nil {
			return "Cheat sheet database is not available."
		}
	case "media_registry":
		if dc.MediaRegistryDB == nil {
			return "Media registry is not enabled or DB not initialized."
		}
	case "homepage_registry":
		if dc.HomepageRegistryDB == nil {
			return "Homepage registry is not enabled or DB not initialized."
		}
	case "sql_query", "manage_sql_connections":
		if dc.SQLConnectionsDB == nil || dc.SQLConnectionPool == nil {
			return "SQL Connections database not available."
		}
	case "manage_missions":
		if dc.MissionManagerV2 == nil {
			return "Mission control storage not available."
		}
	case "manage_daemon":
		if dc.DaemonSupervisor == nil {
			return "Daemon supervisor not initialized."
		}
	}
	return ""
}

func toolSetupRequiredOutput(reason string) string {
	data, _ := json.Marshal(map[string]string{"status": "needs_setup", "code": "runtime_dependency_unavailable", "message": reason})
	return "Tool Output: " + string(data)
}

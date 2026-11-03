package main

import "encoding/json"

// These requests describe the runtime's operation and ownership semantics in
// both corpus languages. Arguments remain the curated sparse fixture values.
func auditTrainingTask(tool, operation, language string, arguments map[string]interface{}) string {
	phrases := map[string][2]string{
		"ecalc/search":              {"Suche passende Elektronikrechner im ECalc-Katalog.", "Search the ECalc catalog for matching electronics calculators."},
		"ecalc/describe":            {"Zeige Variablen, Einheiten und lösbare Eingabekombinationen dieses ECalc-Rechners.", "Describe this ECalc calculator's variables, units and solvable input sets."},
		"ecalc/calculate":           {"Berechne die fehlenden Größen mit ECalc aus diesen Werten mit Einheiten.", "Calculate the missing quantities with ECalc from these values with units."},
		"ecalc/open":                {"Öffne diesen Rechner mit den angegebenen Werten in der ECalc-Desktop-App.", "Open this calculator with the supplied values in the ECalc desktop app."},
		"co_agent/spawn":            {"Starte für diese Sitzung einen Co-Agenten für die angegebene Teilaufgabe.", "Start a co-agent for this session to perform the supplied subtask."},
		"co_agent/spawn_specialist": {"Starte für diese Sitzung den angegebenen Spezialisten für die Teilaufgabe.", "Start the specified specialist for this session to perform the subtask."},
		"co_agent/list":             {"Zeige die Co-Agenten dieser Sitzung und ihren Status.", "List the co-agents owned by this session and their status."},
		"co_agent/get_result":       {"Warte kurz auf den angegebenen Co-Agenten dieser Sitzung und hole sein Ergebnis oder seinen Status.", "Wait briefly for the specified co-agent owned by this session and retrieve its result or status."},
		"co_agent/stop":             {"Ich möchte den angegebenen Co-Agenten dieser Sitzung ausdrücklich abbrechen.", "I explicitly request cancellation of the specified co-agent owned by this session."},
		"co_agent/stop_all":         {"Ich möchte alle laufenden und wartenden Co-Agenten dieser Sitzung ausdrücklich abbrechen.", "I explicitly request cancellation of all running and queued co-agents owned by this session."},
	}
	index := 1
	if language == "de" {
		index = 0
	}
	args, _ := json.Marshal(arguments)
	return phrases[tool+"/"+operation][index] + " " + string(args)
}

package main

import "encoding/json"

func nativeDesktopTrainingTask(op, language string, args map[string]interface{}) string {
	phrases := map[string][2]string{
		"status":        {"Prüfe die native Anwendungssteuerung", "Check native application control"},
		"list_apps":     {"Zeige die laufenden nativen Anwendungen", "List running native applications"},
		"list_windows":  {"Zeige die nativen Fenster mit ihren IDs", "List native windows and their IDs"},
		"list_surfaces": {"Zeige Dialoge und andere Oberflächen des Fensters", "List dialogs and other surfaces of the window"},
		"snapshot":      {"Lies eine kompakte Ansicht der Bedienelemente", "Read a compact accessibility snapshot"},
		"find":          {"Finde das eindeutig benannte Bedienelement", "Find the uniquely named control"},
		"wait":          {"Warte begrenzt auf das bedienbare Element", "Wait within the time limit for the element to be actionable"},
		"screenshot":    {"Erstelle eine private Bildschirmaufnahme für analyze_image", "Capture a private screenshot for analyze_image"},
		"launch":        {"Starte die native Anwendung", "Launch the native application"},
		"close_app":     {"Schließe die Anwendung regulär und beachte Speicherrückfragen", "Close the application normally and observe any save prompt"},
		"focus_window":  {"Aktiviere genau dieses Fenster", "Focus this exact window"},
		"click":         {"Aktiviere das beobachtete Bedienelement einmal", "Activate the observed control once"},
		"double_click":  {"Öffne das beobachtete Element per Doppelklick", "Double-click the observed element"},
		"right_click":   {"Öffne das Kontextmenü des beobachteten Elements", "Open the observed element's context menu"},
		"type":          {"Gib den Text in das beobachtete Textfeld ein", "Type the text into the observed text field"},
		"set_value":     {"Ersetze den Wert des beobachteten Feldes", "Replace the value of the observed field"},
		"select":        {"Wähle die genannte sichtbare Option", "Select the named visible option"},
		"toggle":        {"Schalte das beobachtete Kontrollkästchen einmal um", "Toggle the observed checkbox once"},
		"expand":        {"Klappe das beobachtete Element auf", "Expand the observed element"},
		"collapse":      {"Klappe das beobachtete Element zu", "Collapse the observed element"},
		"scroll_to":     {"Scrolle zum beobachteten Element", "Scroll to the observed element"},
		"press":         {"Sende die Tastenkombination an genau dieses Fenster", "Send the key chord to this exact window"},
		"click_xy":      {"Klicke einmal auf die per Bildanalyse erkannte Position", "Click the position identified by image analysis once"},
		"scroll":        {"Scrolle an der per Bildanalyse erkannten Position", "Scroll at the position identified by image analysis"},
		"drag":          {"Ziehe innerhalb des im Bild analysierten Fensters", "Drag within the window inspected by image analysis"},
	}
	pair := phrases[op]
	raw, _ := json.Marshal(args)
	if language == "de" {
		return pair[0] + " im bestehenden Agent Workspace. Die Referenzen stammen aus der aktuellen Beobachtung desselben Fensters; Bildreferenzen sind jünger als 30 Sekunden. Danach den Zustand prüfen und ungewisse Eingaben nicht wiederholen: " + string(raw)
	}
	return pair[1] + " in the existing Agent Workspace. References come from the current observation of the same window; image references are less than 30 seconds old. Verify the result and never replay uncertain input: " + string(raw)
}

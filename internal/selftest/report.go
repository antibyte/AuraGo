package selftest

import (
	"fmt"
	"sort"
	"strings"
)

func Summarize(run *Run) {
	s := Summary{Cases: len(run.Cases), Verdicts: map[string]int{}, CostStatus: "unknown"}
	enabled, executed, operations := map[string]bool{}, map[string]bool{}, map[string]bool{}
	integrations := map[string]bool{}
	for _, cap := range run.Inventory {
		if !cap.Enabled {
			continue
		}
		key := cap.Name + "@" + cap.Instance
		enabled[key] = true
		if cap.Kind == "integration" {
			integrations[key] = true
		}
		count := 0
		for field, values := range cap.Operations {
			for _, value := range values {
				operations[key+"/"+field+"/"+value] = true
				count++
			}
		}
		if count == 0 {
			operations[key+"/direct"] = true
		}
	}
	verifiedOps := map[string]bool{}
	for _, c := range run.Cases {
		s.Verdicts[c.Verdict]++
		s.Tokens += c.Tokens
		key := c.Tool + "@" + c.Instance
		for _, e := range c.Evidence {
			if e.Tool != c.Tool || !e.Executed {
				continue
			}
			if enabled[key] {
				executed[key] = true
			}
			if c.Verdict == "PASS" && len(c.Assertions) > 0 && e.Status == "success" {
				op := key + "/" + e.Operation
				if e.Operation == "" {
					op = key + "/direct"
				}
				if operations[op] {
					verifiedOps[op] = true
				}
			}
		}
	}
	s.EnabledTools = len(enabled) - len(integrations)
	s.EnabledIntegrations = len(integrations)
	for key := range executed {
		if integrations[key] {
			s.ExecutedIntegrations++
		} else {
			s.ExecutedTools++
		}
	}
	s.EnabledOperations, s.ExecutedOperations = len(operations), len(verifiedOps)
	run.Summary = s
}

func markdownCell(s string) string {
	s = strings.NewReplacer("|", "\\|", "\n", " ", "\r", " ", "<", "&lt;", ">", "&gt;", "[", "\\[", "]", "\\]").Replace(s)
	return s
}

func Markdown(run Run) string {
	Summarize(&run)
	var out strings.Builder
	fmt.Fprintf(&out, "# AuraGo-Selbsttest\n\nLauf: `%s`  \nStatus: %s\n\n", run.ID, run.Status)
	fmt.Fprintf(&out, "Ausgeführte aktivierte Tools: **%d / %d**. Verifizierte Operationsprüfungen: **%d / %d**. Tokens: %d. Kosten: unbekannt; reguläre Providerbuchung bleibt aktiv.\n\n", run.Summary.ExecutedTools, run.Summary.EnabledTools, run.Summary.ExecutedOperations, run.Summary.EnabledOperations, run.Summary.Tokens)
	fmt.Fprintf(&out, "Geprüfte Integrationsverbindungen: **%d / %d**. Operationsabdeckung zählt deklarierte Hauptselektoren; weitere Parameterkombinationen sind damit nicht vollständig geprüft. Tokenangaben betreffen das Testmodell; zusätzliche Dienste behalten ihre eigenen Limits.\n\n", run.Summary.ExecutedIntegrations, run.Summary.EnabledIntegrations)
	var verdicts []string
	for key := range run.Summary.Verdicts {
		verdicts = append(verdicts, key)
	}
	sort.Strings(verdicts)
	for _, key := range verdicts {
		fmt.Fprintf(&out, "- %s: %d\n", key, run.Summary.Verdicts[key])
	}
	out.WriteString("\nPASS bestätigt nur die aufgeführten Assertions. Ein Verbindungstest belegt keine Zustellung, Gerätesteuerung oder vollständige Integration. Blockierte und nicht abgedeckte Fälle bleiben in der Gesamtzahl.\n\n| Test | Tool / Instanz | Ergebnis | Diagnose | Bereinigung |\n| --- | --- | --- | --- | --- |\n")
	for _, c := range run.Cases {
		fmt.Fprintf(&out, "| %s | %s / %s | %s | %s | %s |\n", markdownCell(c.ID), markdownCell(c.Tool), markdownCell(c.Instance), c.Verdict, markdownCell(c.Code), c.Cleanup)
	}
	out.WriteString("\n## Belege\n\n")
	for _, c := range run.Cases {
		if len(c.Evidence) == 0 {
			continue
		}
		fmt.Fprintf(&out, "### %s\n\n", markdownCell(c.ID))
		for _, a := range c.Assertions {
			fmt.Fprintf(&out, "- Assertion: %s\n", markdownCell(a))
		}
		for _, e := range c.Evidence {
			if e.FixtureID != "" {
				fmt.Fprintf(&out, "- Lokales Testobjekt: `%s`.\n", markdownCell(e.FixtureID))
			}
			fmt.Fprintf(&out, "- `%s`: %s / %s; Status %s; %d ms; Argument-Hash `%s`; Ergebnis-Hash `%s`.\n", markdownCell(e.ID), markdownCell(e.Tool), markdownCell(e.Operation), markdownCell(e.Status), e.DurationMS, e.ArgumentsHash, e.OutputHash)
		}
	}
	out.WriteString("\n## Nächste Schritte\n\nAUTH/SETUP: gespeicherte Verbindung und Voraussetzungen prüfen. PERMISSION: vorhandene Grants prüfen, keine automatische Erweiterung. TOOL_EXECUTION/MODEL_BEHAVIOR: den betreffenden Einzeltest anhand seiner Beleg-IDs reproduzieren. BLOCKED_FIXTURE: ein geeignetes isoliertes Testziel bereitstellen. UNKNOWN/CLEANUP: Zustand prüfen, bevor eine Aktion erneut ausgeführt wird. Reparaturen sind ein eigener Auftrag.\n")
	return out.String()
}

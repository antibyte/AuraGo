package main

import (
	"strings"
	"testing"
)

func TestAuditTrainingTasksCoverOperationsAndSessionOwnership(t *testing.T) {
	for tool, operations := range map[string][]string{"ecalc": {"search", "describe", "calculate", "open"}, "co_agent": {"spawn", "spawn_specialist", "list", "get_result", "stop", "stop_all"}} {
		for _, operation := range operations {
			choice := fixtureChoice{Tool: ToolExport{Name: tool}, Arguments: map[string]interface{}{"operation": operation}}
			for _, lang := range []string{"de", "en"} {
				text := naturalTaskClause(choice, lang)
				if len(text) < 55 || !strings.Contains(text, operation) {
					t.Fatalf("missing %s/%s/%s task: %q", tool, operation, lang, text)
				}
				if tool == "co_agent" {
					owner := "this session"
					if lang == "de" {
						owner = "Sitzung"
					}
					if !strings.Contains(text, owner) {
						t.Fatalf("unscoped co-agent request: %s", text)
					}
				}
			}
		}
	}
}

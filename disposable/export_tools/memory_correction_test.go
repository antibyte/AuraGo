package main

import (
	"strings"
	"testing"
)

func TestCorrectionTrainingRequestIncludesActualEvidence(t *testing.T) {
	choice := fixtureChoice{Tool: ToolExport{Name: "correct_memory"}, Arguments: map[string]interface{}{"operation": "preview", "evidence": []interface{}{map[string]interface{}{"reference": "user:current", "quote": "I prefer green tea"}}}}
	for _, language := range []string{"de", "en"} {
		request := naturalTaskClause(choice, language)
		if !strings.Contains(request, "I prefer green tea") || !strings.Contains(request, "user:current") {
			t.Fatalf("%s request omitted the actual evidence", language)
		}
	}
}

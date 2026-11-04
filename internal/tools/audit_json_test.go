package tools

import (
	"aurago/internal/ecalc"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestAnsibleErrorPreservesJSONSpecialCharacters(t *testing.T) {
	message := "failure \"quoted\"\nC:\\path\\ä"
	var result map[string]string
	if err := json.Unmarshal([]byte(ansibleResult(nil, 0, errors.New(message))), &result); err != nil {
		t.Fatal(err)
	}
	if result["message"] != "Cannot reach Ansible sidecar: "+message {
		t.Fatal(result)
	}
}

func TestECalcAllResponsesRespectEffectiveBudget(t *testing.T) {
	long := strings.Repeat("ä\"\n", 10000)
	requests := []ECalcRequest{
		{Operation: "describe", ID: long},
		{Operation: "calculate", ID: "ohms_law", Inputs: []ecalc.NamedValue{{Name: long, Value: "1"}}},
		{Operation: "open", ID: long},
		{Operation: "search", Query: long},
		{Operation: "pinout", ID: long},
		{Operation: "pinout", ID: "x", Pin: long, Variant: long, Filter: long},
	}
	cat, err := ecalc.Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range cat.Calculators {
		requests = append(requests, ECalcRequest{Operation: "describe", ID: item.ID})
	}
	for _, item := range cat.Pinouts {
		requests = append(requests, ECalcRequest{Operation: "describe", ID: item.ID}, ECalcRequest{Operation: "pinout", ID: item.ID})
	}
	for _, req := range requests {
		for _, budget := range []int{0, 100, 1500, 2500} {
			req.OutputBudget = budget
			result := ExecuteECalc(ecalcDesktopConfig(), req)
			if len(result.Output) > ecalcBudget(budget) || !json.Valid([]byte(result.Output)) || !utf8.ValidString(result.Output) {
				t.Fatalf("%s budget=%d bytes=%d", req.Operation, budget, len(result.Output))
			}
		}
	}
}

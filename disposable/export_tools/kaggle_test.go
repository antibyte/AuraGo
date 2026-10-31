package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/kaggle"
)

func TestKaggleCuratedFixturesMatchSDKWireContracts(t *testing.T) {
	var contracts OperationContractManifest
	if err := readJSON(filepath.Join(trainingSource(t), "operation_contracts.json"), &contracts); err != nil {
		t.Fatal(err)
	}
	count := 0
	for name, contract := range contracts.Tools {
		if !strings.HasPrefix(name, "kaggle_") || name == "kaggle_runtime" {
			continue
		}
		for _, fixture := range contract.Operations {
			operation, _ := fixture.Arguments["operation"].(string)
			t.Run(name+"/"+operation, func(t *testing.T) {
				op, err := kaggle.Lookup(strings.TrimPrefix(name, "kaggle_"), operation)
				if err != nil {
					t.Fatal(err)
				}
				raw, err := json.Marshal(fixture.Arguments["parameters"])
				if err != nil {
					t.Fatal(err)
				}
				if _, _, err = kaggle.Decode(op, raw); err != nil {
					t.Fatal(err)
				}
			})
			count++
		}
	}
	if count != len(kaggle.Catalog()) {
		t.Fatalf("curated operations: %d; runtime: %d", count, len(kaggle.Catalog()))
	}
}

func TestKaggleTrainingNeverRetriesUncertainMutation(t *testing.T) {
	for _, tc := range []struct {
		tool, operation string
		mutation        bool
	}{
		{"kaggle_datasets", "create_dataset", true},
		{"kaggle_notebooks", "save_kernel", true},
		{"kaggle_runtime", "execute", true},
		{"kaggle_runtime", "events", false},
		{"kaggle_datasets", "list_datasets", false},
	} {
		t.Run(tc.tool+"/"+tc.operation, func(t *testing.T) {
			choice := fixtureChoice{Tool: ToolExport{Name: tc.tool}, Arguments: map[string]interface{}{"operation": tc.operation}}
			if kaggleMutationFixture(choice) != tc.mutation {
				t.Fatal("incorrect retry classification")
			}
			if tc.mutation {
				if _, err := makeErrorScenario(nil, choice, "en", 0, true); err == nil {
					t.Fatal("mutation retry admitted")
				}
				for _, language := range []string{"de", "en"} {
					s, err := makeErrorScenario([]ToolExport{choice.Tool}, choice, language, 0, false)
					if err != nil || len(s.Expectations.Calls) != 1 || !strings.Contains(s.Messages[3].Content, `"status":"unknown"`) {
						t.Fatal("unknown outcome lost", err)
					}
				}
			}
		})
	}
}

package layerling

import (
	"encoding/json"
	"os"
	"testing"
)

func TestTrainingArgumentsMatchPinnedContract(t *testing.T) {
	data, err := os.ReadFile("../../training/operation_contracts.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Tools map[string]json.RawMessage `json:"tools"`
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Operations []struct {
			Value     string            `json:"value"`
			Arguments map[string]string `json:"arguments"`
		} `json:"operations"`
	}
	if err = json.Unmarshal(manifest.Tools["layerling"], &fixtures); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, fixture := range fixtures.Operations {
		seen[fixture.Value] = true
		if fixture.Value == "list_editors" || fixture.Value == "describe" {
			continue
		}
		if fixture.Arguments["editor_id"] == "" {
			t.Errorf("%s has no editor", fixture.Value)
		}
		if err = Validate(fixture.Value, json.RawMessage(fixture.Arguments["arguments"])); err != nil {
			t.Errorf("%s: %v", fixture.Value, err)
		}
	}
	for _, op := range Operations() {
		if !seen[op] {
			t.Errorf("missing fixture: %s", op)
		}
	}
	if err = Validate("import_file", json.RawMessage(`{"fileName":"mesh.zip","base64":"AA=="}`)); err == nil {
		t.Fatal("raw file transport exposed")
	}
}

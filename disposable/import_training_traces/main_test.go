package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportTracesSanitizesAndRequiresReview(t *testing.T) {
	tmp := t.TempDir()
	input := filepath.Join(tmp, "input.jsonl")
	staging := filepath.Join(tmp, "staging")
	row := map[string]interface{}{
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": `contact admin@private.example at 10.0.0.4 and read C:\Users\Alice\secret.txt with api_key=supersecret123`,
			},
		},
	}
	raw, _ := json.Marshal(row)
	if err := os.WriteFile(input, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := importTraces(cliOptions{input: input, staging: staging}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(staging, "traces_staged.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, secret := range []string{"admin@private.example", "10.0.0.4", `C:\Users\Alice`, "supersecret123"} {
		if strings.Contains(text, secret) {
			t.Fatalf("staged trace retained sensitive value %q: %s", secret, text)
		}
	}
	if !strings.Contains(text, `"review_status":"pending"`) {
		t.Fatalf("staged trace is not pending human review: %s", text)
	}
}

func TestImportTracesRejectsRowsWithoutMessages(t *testing.T) {
	tmp := t.TempDir()
	input := filepath.Join(tmp, "input.jsonl")
	staging := filepath.Join(tmp, "staging")
	if err := os.WriteFile(input, []byte("{\"content\":\"not a conversation\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := importTraces(cliOptions{input: input, staging: staging}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(staging, "traces_staged.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("unexpected staged row: %s", data)
	}
}

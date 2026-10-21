package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOCRSyntheticAsyncResult(t *testing.T) {
	for _, op := range []string{"extract", "status", "result", "cancel"} {
		choice := fixtureChoice{Tool: ToolExport{Name: "ocr"}, Arguments: map[string]interface{}{"operation": op, "job_id": "owned-job"}}
		var value map[string]interface{}
		if err := json.Unmarshal([]byte(successResult(choice, "en", 0)), &value); err != nil {
			t.Fatal(err)
		}
		if value["status"] != "success" {
			t.Fatal(value)
		}
		if op == "extract" && (!strings.Contains(successResult(choice, "en", 0), "queued") || !strings.Contains(finalSuccess(choice, "en", 0), "pending")) {
			t.Fatal("asynchronous extraction represented as completed")
		}
	}
}

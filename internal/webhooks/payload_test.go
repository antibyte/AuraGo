package webhooks

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestPreparePayloadKeepsJSONValidInsideIsolationBoundary(t *testing.T) {
	t.Parallel()

	prepared, err := PreparePayload([]byte(`{"message":"<external_data>&\"quoted\"</external_data>"}`), "application/json; charset=utf-8", nil, 4000)
	if err != nil {
		t.Fatalf("PreparePayload() error = %v", err)
	}
	if strings.Contains(prepared.PromptPayload, "&quot;") {
		t.Fatalf("PromptPayload contains HTML-escaped quotes: %q", prepared.PromptPayload)
	}
	if strings.Count(prepared.PromptPayload, "</external_data>") != 1 {
		t.Fatalf("closing boundary count = %d, want 1: %q", strings.Count(prepared.PromptPayload, "</external_data>"), prepared.PromptPayload)
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(prepared.PromptPayload, "<external_data>\n"), "\n</external_data>")
	if !json.Valid([]byte(inner)) {
		t.Fatalf("isolated JSON is invalid: %q", inner)
	}
	if !strings.Contains(inner, `\u003c`) || !strings.Contains(inner, `\u0026`) {
		t.Fatalf("JSON boundary characters were not escaped: %q", inner)
	}
}

func TestPreparePayloadTruncatesLongJSONToValidEnvelope(t *testing.T) {
	t.Parallel()

	body := []byte(`{"message":"` + strings.Repeat("🙂", 200) + `"}`)
	prepared, err := PreparePayload(body, "application/json", nil, 120)
	if err != nil {
		t.Fatalf("PreparePayload() error = %v", err)
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(prepared.PromptPayload, "<external_data>\n"), "\n</external_data>")
	if !json.Valid([]byte(inner)) {
		t.Fatalf("truncated JSON is invalid: %q", inner)
	}
	if len([]rune(inner)) > 120 {
		t.Fatalf("truncated JSON runes = %d, want <= 120", len([]rune(inner)))
	}
	var envelope map[string]interface{}
	if err := json.Unmarshal([]byte(inner), &envelope); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if envelope["truncated"] != true {
		t.Fatalf("truncated metadata = %#v, want true", envelope["truncated"])
	}
}

func TestPreparePayloadRejectsInvalidJSONAndNonObjectMappings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		body        string
		contentType string
		mappings    []FieldMapping
		status      int
	}{
		{name: "invalid json", body: `{`, contentType: "application/json", status: 400},
		{name: "mapping on text", body: `hello`, contentType: "text/plain", mappings: []FieldMapping{{Source: "hello"}}, status: 415},
		{name: "mapping on array", body: `[]`, contentType: "application/json", mappings: []FieldMapping{{Source: "hello"}}, status: 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := PreparePayload([]byte(tt.body), tt.contentType, tt.mappings, 4000)
			var payloadErr *PayloadError
			if !errors.As(err, &payloadErr) {
				t.Fatalf("error = %v, want PayloadError", err)
			}
			if payloadErr.StatusCode != tt.status {
				t.Fatalf("StatusCode = %d, want %d", payloadErr.StatusCode, tt.status)
			}
		})
	}
}

func TestPreparePayloadExtractsMappedJSONObjectFields(t *testing.T) {
	t.Parallel()

	prepared, err := PreparePayload([]byte(`{"repository":{"full_name":"owner/repo"}}`), "application/problem+json", []FieldMapping{{Source: "repository.full_name", Alias: "repo"}}, 4000)
	if err != nil {
		t.Fatalf("PreparePayload() error = %v", err)
	}
	if got := prepared.Fields["repo"]; got != "owner/repo" {
		t.Fatalf("Fields[repo] = %#v, want owner/repo", got)
	}
}

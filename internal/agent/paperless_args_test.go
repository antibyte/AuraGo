package agent

import (
	"encoding/json"
	"testing"
)

func TestDecodePaperlessArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		wantID   string
		wantQ    string
		wantTags string
		wantName string
		wantCat  string
		wantLim  int
	}{
		{
			name: "native fields win over skill aliases",
			args: map[string]interface{}{
				"correspondent": "Native Corp",
				"name":          "Skill Corp",
				"document_type": "Invoice",
				"category":      "Receipt",
				"query":         "electricity",
				"content":       "not a query",
			},
			wantQ:    "electricity",
			wantName: "Native Corp",
			wantCat:  "Invoice",
		},
		{
			name: "skill aliases when native fields are absent",
			args: map[string]interface{}{
				"name":     "Stadtwerke",
				"category": "Bill",
				"query":    "2025",
			},
			wantQ:    "2025",
			wantName: "Stadtwerke",
			wantCat:  "Bill",
		},
		{
			name: "numeric document id and tag array",
			args: map[string]interface{}{
				"document_id": float64(42),
				"tags":        []interface{}{"invoice", float64(2), " tax "},
			},
			wantID:   "42",
			wantTags: "invoice,2,tax",
		},
		{
			name: "id alias and numeric limit string",
			args: map[string]interface{}{
				"id":      json.Number("7"),
				"limit":   "1000",
				"content": "body only",
			},
			wantID:  "7",
			wantLim: 1000,
		},
		{
			name: "content is not a search query",
			args: map[string]interface{}{
				"content": "meeting notes",
				"limit":   float64(10),
			},
			wantLim: 10,
		},
		{
			name: "fractional document id is ignored",
			args: map[string]interface{}{
				"document_id": float64(4.5),
				"id":          "9",
			},
			wantID: "9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodePaperlessArgs(tt.args)
			if got.DocumentID != tt.wantID || got.Query != tt.wantQ || got.Tags != tt.wantTags || got.Name != tt.wantName || got.Category != tt.wantCat || got.Limit != tt.wantLim {
				t.Fatalf("decode = %+v", got)
			}
		})
	}
}

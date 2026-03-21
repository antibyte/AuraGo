package telnyx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateE164(t *testing.T) {
	tests := []struct {
		number string
		valid  bool
	}{
		{"+14155551234", true},
		{"+491511234567", true},
		{"+1", false},
		{"+123456789012345", true},
		{"14155551234", false},       // missing +
		{"+0155551234", false},       // starts with 0
		{"", false},                  // empty
		{"+", false},                 // only +
		{"+1234567890123456", false}, // too long (16 digits)
		{"abc", false},               // not a number
		{"+1-415-555-1234", false},   // dashes not allowed
	}

	for _, tt := range tests {
		err := ValidateE164(tt.number)
		if tt.valid && err != nil {
			t.Errorf("ValidateE164(%q) returned error %v, expected valid", tt.number, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("ValidateE164(%q) returned nil, expected error", tt.number)
		}
	}
}

func TestClient_SendSMS(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/messages" {
			t.Errorf("expected path /messages, got %s", r.URL.Path)
		}

		var req SendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("could not decode request: %v", err)
		}
		if req.From != "+14155551234" {
			t.Errorf("expected from +14155551234, got %s", req.From)
		}
		if req.To != "+491511234567" {
			t.Errorf("expected to +491511234567, got %s", req.To)
		}
		if req.Text != "Hello, World!" {
			t.Errorf("expected text 'Hello, World!', got %s", req.Text)
		}
		if req.Type != "SMS" {
			t.Errorf("expected type 'SMS', got %s", req.Type)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":     "msg_abc123",
				"status": "queued",
				"type":   "SMS",
			},
		})
	}))
	defer ts.Close()

	c := NewClient("key", nil)
	c.baseURL = ts.URL

	resp, err := c.SendSMS(context.Background(), "+14155551234", "+491511234567", "Hello, World!", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Data.ID != "msg_abc123" {
		t.Errorf("expected ID 'msg_abc123', got %q", resp.Data.ID)
	}
	if resp.Data.Status != "queued" {
		t.Errorf("expected status 'queued', got %q", resp.Data.Status)
	}
}

func TestClient_SendSMS_InvalidNumbers(t *testing.T) {
	c := NewClient("key", nil)

	_, err := c.SendSMS(context.Background(), "not-e164", "+14155551234", "test", "")
	if err == nil {
		t.Error("expected error for invalid from number")
	}

	_, err = c.SendSMS(context.Background(), "+14155551234", "not-e164", "test", "")
	if err == nil {
		t.Error("expected error for invalid to number")
	}

	_, err = c.SendSMS(context.Background(), "+14155551234", "+491511234567", "", "")
	if err == nil {
		t.Error("expected error for empty message")
	}
}

func TestClient_SendMMS(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req SendMessageRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Type != "MMS" {
			t.Errorf("expected type 'MMS', got %s", req.Type)
		}
		if len(req.MediaURLs) != 1 {
			t.Errorf("expected 1 media URL, got %d", len(req.MediaURLs))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":     "msg_mms123",
				"status": "queued",
				"type":   "MMS",
			},
		})
	}))
	defer ts.Close()

	c := NewClient("key", nil)
	c.baseURL = ts.URL

	resp, err := c.SendMMS(context.Background(), "+14155551234", "+491511234567", "Photo", []string{"https://example.com/img.jpg"}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Data.ID != "msg_mms123" {
		t.Errorf("expected ID 'msg_mms123', got %q", resp.Data.ID)
	}
}

func TestClient_SendMMS_Validation(t *testing.T) {
	c := NewClient("key", nil)

	_, err := c.SendMMS(context.Background(), "+14155551234", "+491511234567", "text", nil, "")
	if err == nil {
		t.Error("expected error for empty media URLs")
	}

	tooMany := make([]string, 11)
	for i := range tooMany {
		tooMany[i] = "https://example.com/img.jpg"
	}
	_, err = c.SendMMS(context.Background(), "+14155551234", "+491511234567", "text", tooMany, "")
	if err == nil {
		t.Error("expected error for >10 media URLs")
	}
}

func TestClient_GetMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/messages/msg_abc123" {
			t.Errorf("expected path /messages/msg_abc123, got %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":     "msg_abc123",
				"status": "delivered",
			},
		})
	}))
	defer ts.Close()

	c := NewClient("key", nil)
	c.baseURL = ts.URL

	resp, err := c.GetMessage(context.Background(), "msg_abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Data.Status != "delivered" {
		t.Errorf("expected status 'delivered', got %q", resp.Data.Status)
	}
}

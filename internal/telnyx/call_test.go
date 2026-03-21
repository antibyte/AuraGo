package telnyx

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestInitiateCall(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v2/calls" {
			t.Errorf("expected path /v2/calls, got %s", r.URL.Path)
		}
		var body CreateCallRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.ConnectionID != "conn-123" {
			t.Errorf("expected connection_id 'conn-123', got %q", body.ConnectionID)
		}
		if body.To != "+15551234567" {
			t.Errorf("expected to '+15551234567', got %q", body.To)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(CallResponse{
			Data: struct {
				CallControlID string `json:"call_control_id"`
				CallLegID     string `json:"call_leg_id"`
				CallSessionID string `json:"call_session_id"`
				IsAlive       bool   `json:"is_alive"`
				RecordType    string `json:"record_type"`
				State         string `json:"state"`
			}{
				CallControlID: "v2-ctrl-abc123",
				CallSessionID: "session-xyz",
				State:         "dialing",
				IsAlive:       true,
			},
		})
	}))
	defer ts.Close()

	c := NewClient("key", nil)
	c.baseURL = ts.URL

	resp, err := c.InitiateCall(context.Background(), "conn-123", "+15559999999", "+15551234567", "", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Data.CallControlID != "v2-ctrl-abc123" {
		t.Errorf("expected call_control_id 'v2-ctrl-abc123', got %q", resp.Data.CallControlID)
	}
	if resp.Data.State != "dialing" {
		t.Errorf("expected state 'dialing', got %q", resp.Data.State)
	}
}

func TestInitiateCall_InvalidNumber(t *testing.T) {
	c := NewClient("key", nil)
	_, err := c.InitiateCall(context.Background(), "conn-123", "+15559999999", "invalid", "", 30)
	if err == nil {
		t.Error("expected error for invalid number")
	}
}

func TestInitiateCall_MissingConnectionID(t *testing.T) {
	c := NewClient("key", nil)
	_, err := c.InitiateCall(context.Background(), "", "+15559999999", "+15551234567", "", 30)
	if err == nil {
		t.Error("expected error for missing connection_id")
	}
}

func TestSpeakText(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/calls/ctrl-abc/actions/speak" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body SpeakRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.Payload != "Hello world" {
			t.Errorf("expected payload 'Hello world', got %q", body.Payload)
		}
		if body.Voice != "female" {
			t.Errorf("expected voice 'female', got %q", body.Voice)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClient("key", nil)
	c.baseURL = ts.URL

	err := c.SpeakText(context.Background(), "ctrl-abc", "Hello world", "en-US", "female")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSpeakText_MissingParams(t *testing.T) {
	c := NewClient("key", nil)
	if err := c.SpeakText(context.Background(), "", "text", "en", "female"); err == nil {
		t.Error("expected error for missing call_control_id")
	}
	if err := c.SpeakText(context.Background(), "ctrl-1", "", "en", "female"); err == nil {
		t.Error("expected error for missing text")
	}
}

func TestGatherDTMF(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/calls/ctrl-abc/actions/gather_using_speak" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body GatherSpeakRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.MaximumDigits != 4 {
			t.Errorf("expected max_digits 4, got %d", body.MaximumDigits)
		}
		if body.TimeoutMillis != 15000 {
			t.Errorf("expected timeout 15000ms, got %d", body.TimeoutMillis)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClient("key", nil)
	c.baseURL = ts.URL

	err := c.GatherDTMF(context.Background(), "ctrl-abc", "Enter your code", "en-US", "female", 4, 15)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTransferCall(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/calls/ctrl-abc/actions/transfer" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body TransferRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.To != "+15559876543" {
			t.Errorf("expected to '+15559876543', got %q", body.To)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClient("key", nil)
	c.baseURL = ts.URL

	err := c.TransferCall(context.Background(), "ctrl-abc", "+15559876543", "+15551111111")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTransferCall_InvalidNumber(t *testing.T) {
	c := NewClient("key", nil)
	err := c.TransferCall(context.Background(), "ctrl-abc", "not-e164", "+15551111111")
	if err == nil {
		t.Error("expected error for invalid transfer number")
	}
}

func TestHangUp(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/calls/ctrl-abc/actions/hangup" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClient("key", nil)
	c.baseURL = ts.URL

	err := c.HangUp(context.Background(), "ctrl-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecordStartStop(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := NewClient("key", nil)
	c.baseURL = ts.URL

	if err := c.RecordStart(context.Background(), "ctrl-abc", "mp3", "single"); err != nil {
		t.Fatalf("RecordStart error: %v", err)
	}
	if err := c.RecordStop(context.Background(), "ctrl-abc"); err != nil {
		t.Fatalf("RecordStop error: %v", err)
	}
}

func TestDispatchCall_MissingTo(t *testing.T) {
	cfg := &config.Config{}
	cfg.Telnyx.APIKey = "key"
	cfg.Telnyx.ConnectionID = "conn-abc"

	result := DispatchCall(context.Background(), "initiate", "", "", "", "", 0, 0, cfg, nil)
	var m map[string]interface{}
	json.Unmarshal([]byte(result), &m)
	if m["status"] != "error" {
		t.Errorf("expected error status, got %v", m["status"])
	}
}

func TestDispatchCall_UnknownOperation(t *testing.T) {
	cfg := &config.Config{}
	cfg.Telnyx.APIKey = "key"

	result := DispatchCall(context.Background(), "invalid_op", "", "", "", "", 0, 0, cfg, nil)
	var m map[string]interface{}
	json.Unmarshal([]byte(result), &m)
	if m["status"] != "error" {
		t.Errorf("expected error status, got %v", m["status"])
	}
}

package telnyx

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDestinationPolicyCoversAllOutboundOperationsAndFeedback(t *testing.T) {
	cfg := &config.Config{}
	cfg.Telnyx.Enabled = true
	cfg.Telnyx.PhoneNumber = "+15550000001"
	cfg.Telnyx.ConnectionID = "application"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, allowed := range [][]string{nil, {"+15550000003"}, {"+15550000002"}} {
		cfg.Telnyx.AllowedNumbers = allowed
		for _, readOnly := range []bool{false, true} {
			cfg.Telnyx.ReadOnly = readOnly
			c := NewConfiguredClient(cfg, logger)
			requests := 0
			c.httpClient.Transport = boundaryTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":{}}`)), Header: make(http.Header)}, nil
			})
			operations := []func() error{
				func() error {
					_, err := c.InitiateCall(context.Background(), "application", cfg.Telnyx.PhoneNumber, "+15550000002", "", 30, 300)
					return err
				},
				func() error {
					return c.TransferCall(context.Background(), "call", "+15550000002", cfg.Telnyx.PhoneNumber)
				},
				func() error {
					_, err := c.SendSMS(context.Background(), cfg.Telnyx.PhoneNumber, "+15550000002", "fixture", "")
					return err
				},
				func() error {
					_, err := c.SendMMS(context.Background(), cfg.Telnyx.PhoneNumber, "+15550000002", "fixture", []string{"https://example.com/a.jpg"}, "")
					return err
				},
			}
			permitted := !readOnly && len(allowed) == 1 && allowed[0] == "+15550000002"
			for i, call := range operations {
				if err := call(); (err == nil) != permitted {
					t.Fatalf("operation %d, allowed=%v readOnly=%v error=%v", i, allowed, readOnly, err)
				}
			}
			if !permitted && requests != 0 {
				t.Fatal("blocked destination reached transport")
			}
			broker := NewSMSBroker(cfg, "+15550000002", logger)
			broker.client.httpClient.Transport = c.httpClient.Transport
			before := requests
			broker.Send("final_response", "fixture")
			if (requests > before) != permitted {
				t.Fatal("feedback bypassed destination policy")
			}
		}
	}
	cfg.Telnyx.ReadOnly = false
	cfg.Telnyx.AllowedNumbers = nil
	for _, op := range []string{"initiate", "transfer"} {
		if result := DispatchCall(context.Background(), op, "+15550000002", "call", "", "", 0, 0, cfg, logger); !strings.Contains(result, "allowed_numbers") {
			t.Fatalf("dispatch call %s: %s", op, result)
		}
	}
	for _, op := range []string{"send", "send_mms"} {
		if result := DispatchSMS(context.Background(), op, "+15550000002", "fixture", "", []string{"https://example.com/a.jpg"}, cfg, logger); !strings.Contains(result, "allowed_numbers") {
			t.Fatalf("dispatch SMS %s: %s", op, result)
		}
	}
}

func TestCallDurationIsSeparateFromRingingAndCallbackMustBeAbsolute(t *testing.T) {
	c := newAllowlistedTestClient("fixture-key", "+15550000002")
	requests := 0
	c.httpClient.Transport = boundaryTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["timeout_secs"] != float64(30) || payload["time_limit_secs"] != float64(900) {
			t.Fatalf("limits: %v", payload)
		}
		if _, exists := payload["webhook_url"]; exists {
			t.Fatal("application callback overridden")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":{}}`)), Header: make(http.Header)}, nil
	})
	if _, err := c.InitiateCall(context.Background(), "application", "+15550000001", "+15550000002", "", 0, 900); err != nil {
		t.Fatal(err)
	}
	for _, callback := range []string{"/api/telnyx/webhook", "//example.com/hook", "https://user:pass@example.com/hook"} {
		if _, err := c.InitiateCall(context.Background(), "application", "+15550000001", "+15550000002", callback, 30, 900); err == nil {
			t.Fatal("invalid callback accepted")
		}
	}
	if requests != 1 {
		t.Fatal("invalid callback reached provider")
	}
}

func TestIncomingCallsAreRejectedOrAnsweredOnce(t *testing.T) {
	for _, mode := range []string{"allowed", "denied", "read-only", "disabled", "busy", "uncertain-answer"} {
		t.Run(mode, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Telnyx.Enabled = mode != "disabled"
			cfg.Telnyx.ReadOnly = mode == "read-only"
			cfg.Telnyx.MaxConcurrentCalls = 1
			if mode != "denied" {
				cfg.Telnyx.AllowedNumbers = []string{"+15550000002"}
			}
			h := NewWebhookHandler(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nil, nil)
			if mode == "busy" {
				h.activeCalls["existing"] = &CallSession{}
			}
			var paths []string
			h.client.httpClient.Transport = boundaryTransport(func(r *http.Request) (*http.Response, error) {
				paths = append(paths, r.URL.EscapedPath())
				status := 200
				if mode == "uncertain-answer" {
					status = 503
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
			})
			event := &WebhookEvent{}
			event.Data.Payload.Direction = "incoming"
			event.Data.Payload.From = "+15550000002"
			event.Data.Payload.CallControlID = "call/id"
			h.handleCallInitiated(event)
			accepted := mode == "allowed" || mode == "uncertain-answer"
			want := "/v2/calls/call%2Fid/actions/reject"
			if accepted {
				want = "/v2/calls/call%2Fid/actions/answer"
				h.handleCallInitiated(event)
			}
			if len(paths) != 1 || paths[0] != want {
				t.Fatalf("provider actions: %v", paths)
			}
			if (h.activeCalls["call/id"] != nil) != accepted {
				t.Fatal("call reservation mismatch")
			}
		})
	}
}

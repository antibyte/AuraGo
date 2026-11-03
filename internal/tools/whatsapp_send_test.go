package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"aurago/internal/config"
)

// scriptedWhatsAppSender answers SendOwnerMessage with a fixed result and records the call. It
// is a pointer, so it is comparable as the interface requires.
type scriptedWhatsAppSender struct {
	ready bool
	res   WhatsAppSendResult
	err   error

	calls int
	text  string
	file  string
}

func (f *scriptedWhatsAppSender) Ready() bool { return f.ready }

func (f *scriptedWhatsAppSender) SendOwnerMessage(_ context.Context, text, filePath string) (WhatsAppSendResult, error) {
	f.calls++
	f.text, f.file = text, filePath
	return f.res, f.err
}

func decodeWhatsAppToolResult(t *testing.T, out string) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("result is not JSON: %v\n%s", err, out)
	}
	return v
}

func whatsAppToolConfig(enabled, readonly bool) *config.Config {
	cfg := &config.Config{}
	cfg.WhatsApp.Enabled, cfg.WhatsApp.ReadOnly = enabled, readonly
	return cfg
}

// Not parallel: the owner sender is process-wide state.
func TestSendWhatsAppMessageResult(t *testing.T) {
	t.Cleanup(func() { SetWhatsAppOwnerSender(nil) })

	// A file whose text went out first.
	fake := &scriptedWhatsAppSender{ready: true, res: WhatsAppSendResult{Kind: "document", File: "a.pdf", TextSent: true}}
	SetWhatsAppOwnerSender(fake)
	got := decodeWhatsAppToolResult(t, SendWhatsAppMessage(context.Background(), whatsAppToolConfig(true, false), "hi", "a.pdf"))
	if got["status"] != "success" || got["kind"] != "document" || got["document"] != "a.pdf" || got["text_sent"] != true {
		t.Fatalf("success = %#v", got)
	}
	results, _ := got["results"].([]any)
	if len(results) != 1 || !strings.Contains(toJSON(t, results[0]), `"channel":"whatsapp"`) || !strings.Contains(toJSON(t, results[0]), `"status":"sent"`) {
		t.Fatalf("results = %#v", got["results"])
	}
	if fake.text != "hi" || fake.file != "a.pdf" {
		t.Fatalf("sender got text=%q file=%q", fake.text, fake.file)
	}

	// A plain message names no document and no text_sent.
	fake.res = WhatsAppSendResult{Kind: "text"}
	got = decodeWhatsAppToolResult(t, SendWhatsAppMessage(context.Background(), whatsAppToolConfig(true, false), "hi", ""))
	if _, ok := got["document"]; ok || got["kind"] != "text" || got["text_sent"] != nil {
		t.Fatalf("text result = %#v", got)
	}

	// An error keeps text_sent, so the model knows only the file failed.
	fake.res, fake.err = WhatsAppSendResult{Kind: "document", File: "a.pdf", TextSent: true}, errors.New("upload failed")
	got = decodeWhatsAppToolResult(t, SendWhatsAppMessage(context.Background(), whatsAppToolConfig(true, false), "hi", "a.pdf"))
	if got["status"] != "error" || got["message"] != "upload failed" || got["text_sent"] != true {
		t.Fatalf("error = %#v", got)
	}
	if !strings.Contains(toJSON(t, got["results"]), `"detail":"upload failed"`) {
		t.Fatalf("results = %#v", got["results"])
	}

	// The three refusals send nothing.
	fake.calls, fake.err = 0, nil
	for name, tc := range map[string]struct {
		cfg    *config.Config
		sender WhatsAppOwnerSender
		want   string
	}{
		"disabled":      {whatsAppToolConfig(false, false), fake, "not enabled"},
		"nil config":    {nil, fake, "not enabled"},
		"readonly":      {whatsAppToolConfig(true, true), fake, "read-only mode"},
		"not published": {whatsAppToolConfig(true, false), nil, "not linked or not running"},
		"not ready":     {whatsAppToolConfig(true, false), &scriptedWhatsAppSender{}, "not linked or not running"},
	} {
		SetWhatsAppOwnerSender(tc.sender)
		got := decodeWhatsAppToolResult(t, SendWhatsAppMessage(context.Background(), tc.cfg, "hi", ""))
		if got["status"] != "error" || !strings.Contains(toJSON(t, got), tc.want) {
			t.Errorf("%s: result = %#v", name, got)
		}
	}
	if fake.calls != 0 {
		t.Fatalf("a refused send reached the sender %d times", fake.calls)
	}
}

func toJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

package server

import (
	"net/http"
	"testing"

	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

func homeDashCommandFake() *sourcetest.FakeSource {
	return sourcetest.New("fake",
		sourcetest.Entity("fake:a", homedash.KindSwitch, homedash.CapabilitySpec{Name: homedash.CapToggle, Allowed: true}),
		sourcetest.Entity("fake:s", homedash.KindSensor))
}

func TestHomeDashCommandsAcceptAuditAndRefuse(t *testing.T) {
	fake := homeDashCommandFake()
	env := newHomeDashTestEnv(t, nil, fakeHomeDashSpec(fake))
	cmd := func(body map[string]any) (int, string) {
		w := env.do(t, env.admin, http.MethodPost, "commands", body)
		if w.Code == http.StatusAccepted {
			return w.Code, ""
		}
		return w.Code, hdCode(t, w)
	}
	if code, _ := cmd(map[string]any{"entity": "fake:a", "command": "toggle", "request_id": "r1"}); code != http.StatusAccepted || len(fake.Commands()) != 1 {
		t.Fatalf("toggle = %d, %d source calls", code, len(fake.Commands()))
	}
	if !env.audits.has("fake:a|toggle|ok") {
		t.Fatalf("audit = %+v", env.audits.entries)
	}
	if code, _ := cmd(map[string]any{"entity": "fake:a", "command": "toggle", "request_id": "r1"}); code != http.StatusAccepted || len(fake.Commands()) != 1 {
		t.Fatalf("same request id = %d, %d source calls (dedupe)", code, len(fake.Commands()))
	}
	tests := []struct {
		body       map[string]any
		status     int
		code       string
		auditEntry string
	}{
		{map[string]any{"entity": "fake:a", "command": "toggle"}, 400, "bad_request", ""},
		{map[string]any{"entity": "fake:a", "command": "toggle", "request_id": "r2", "dashboard_id": "nope"}, 400, "bad_request", ""},
		{map[string]any{"entity": "bad id", "command": "toggle", "request_id": "r5"}, 400, "bad_request", ""},
		{map[string]any{"entity": "fake:s", "command": "toggle", "request_id": "r3"}, 400, "unsupported_command", "fake:s|toggle|refused:unsupported_command"},
		{map[string]any{"entity": "fake:zz", "command": "toggle", "request_id": "r4"}, 404, "entity_not_found", "fake:zz|toggle|refused:entity_not_found"},
	}
	for _, tc := range tests {
		if status, code := cmd(tc.body); status != tc.status || code != tc.code {
			t.Errorf("%v = %d %s, want %d %s", tc.body, status, code, tc.status, tc.code)
		}
		if tc.auditEntry != "" && !env.audits.has(tc.auditEntry) {
			t.Errorf("missing audit %q in %+v", tc.auditEntry, env.audits.entries)
		}
	}
}

func TestHomeDashCommandsRefusedOnReadOnly(t *testing.T) {
	fake := homeDashCommandFake()
	env := newHomeDashTestEnv(t, func(c *config.Config) { c.VirtualDesktop.ReadOnly = true }, fakeHomeDashSpec(fake))
	w := env.do(t, env.admin, http.MethodPost, "commands", map[string]any{"entity": "fake:a", "command": "toggle", "request_id": "r1"})
	if w.Code != http.StatusForbidden || hdCode(t, w) != "desktop_readonly" || len(fake.Commands()) != 0 {
		t.Fatalf("read-only desktop = %d %s", w.Code, w.Body.String())
	}

	fake2 := homeDashCommandFake()
	env2 := newHomeDashTestEnv(t, func(c *config.Config) { c.HomeDash.ReadOnly = true }, fakeHomeDashSpec(fake2))
	w = env2.do(t, env2.admin, http.MethodPost, "commands", map[string]any{"entity": "fake:a", "command": "toggle", "request_id": "r1"})
	var body struct {
		Code      string `json:"code"`
		BlockedBy string `json:"blocked_by"`
	}
	hdDecode(t, w, &body)
	if w.Code != http.StatusForbidden || body.Code != "read_only" || body.BlockedBy != "homedash.readonly" || len(fake2.Commands()) != 0 {
		t.Fatalf("homedash.readonly = %d %s", w.Code, w.Body.String())
	}
}

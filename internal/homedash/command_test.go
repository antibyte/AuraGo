package homedash_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

func f64(v float64) *float64 { return &v }

func cmd(id string, c homedash.Capability, args map[string]any, p homedash.Principal) homedash.Command {
	if args == nil {
		args = map[string]any{}
	}
	return homedash.Command{Entity: homedash.EntityID(id), Name: c, Args: args, Principal: p}
}

func gateSource() *sourcetest.FakeSource {
	return sourcetest.New("fa",
		sourcetest.Entity("fa:light", homedash.KindLight, homedash.CapabilitySpec{Name: homedash.CapToggle, Allowed: true},
			homedash.CapabilitySpec{Name: homedash.CapBrightness, Allowed: true, Min: f64(0), Max: f64(100), Step: f64(1)}),
		sourcetest.Entity("fa:lock", homedash.KindLock, homedash.CapabilitySpec{Name: homedash.CapLock, Allowed: true},
			homedash.CapabilitySpec{Name: homedash.CapUnlock, Allowed: true}),
		sourcetest.Entity("fa:cover", homedash.KindCover, homedash.CapabilitySpec{Name: homedash.CapOpen, BlockedBy: "home_assistant.allowed_services"}))
}

func TestCommandPipelineGates(t *testing.T) {
	sess := session("s")
	kiosk := func(viewOnly bool) homedash.Principal {
		return homedash.Principal{Kind: homedash.PrincipalKiosk, Key: "kd", DeviceID: "kd", ViewOnly: viewOnly,
			Allowed: &homedash.AllowedSet{Commands: map[homedash.EntityID]map[homedash.Capability]bool{
				"fa:light": {homedash.CapToggle: true}, "fa:lock": {homedash.CapUnlock: true}}}}
	}
	agent := homedash.Principal{Kind: homedash.PrincipalAgent, Key: "a"}
	unlockConfirmed := cmd("fa:lock", homedash.CapUnlock, nil, kiosk(false))
	unlockConfirmed.Confirmed = true
	for _, tc := range []struct {
		name            string
		mut             func(*homedash.RuntimeConfig)
		c               homedash.Command
		code, blockedBy string
	}{
		{"unknown entity", nil, cmd("fa:nope", homedash.CapToggle, nil, sess), homedash.CodeEntityNotFound, ""},
		{"unknown source", nil, cmd("zz:x", homedash.CapToggle, nil, sess), homedash.CodeEntityNotFound, ""},
		{"missing capability", nil, cmd("fa:light", homedash.CapOpen, nil, sess), homedash.CodeUnsupportedCommand, ""},
		{"bad args", nil, cmd("fa:light", homedash.CapBrightness, map[string]any{"value": 150.0}, sess), homedash.CodeInvalidArgs, ""},
		{"integration blocked", nil, cmd("fa:cover", homedash.CapOpen, nil, sess), homedash.CodeReadOnly, "home_assistant.allowed_services"},
		{"homedash readonly", func(c *homedash.RuntimeConfig) { c.ReadOnly = true }, cmd("fa:light", homedash.CapToggle, nil, sess), homedash.CodeReadOnly, "homedash.readonly"},
		{"desktop readonly", func(c *homedash.RuntimeConfig) { c.DesktopReadOnly = true }, cmd("fa:light", homedash.CapToggle, nil, sess), homedash.CodeReadOnly, "virtual_desktop.readonly"},
		{"control off", func(c *homedash.RuntimeConfig) { s := c.Sources["fa"]; s.Control = false; c.Sources["fa"] = s },
			cmd("fa:light", homedash.CapToggle, nil, sess), homedash.CodeReadOnly, "homedash.sources.fa.control"},
		{"control off without config key", func(c *homedash.RuntimeConfig) {
			c.Sources["fa"] = homedash.SourceSettings{Enabled: true, Control: false}
		}, cmd("fa:light", homedash.CapToggle, nil, sess), homedash.CodeReadOnly, "homedash.sources.fa"},
		// The source stays registered but the snapshot has no entry for it (reconfigured after setup below).
		{"no settings entry", nil,
			cmd("fa:light", homedash.CapToggle, nil, sess), homedash.CodeReadOnly, "homedash.sources.fa"},
		{"kiosk outside set", nil, cmd("fa:lock", homedash.CapLock, nil, kiosk(false)), homedash.CodeForbiddenEntity, ""},
		{"kiosk view only", nil, cmd("fa:light", homedash.CapToggle, nil, kiosk(true)), homedash.CodeViewOnly, ""},
		{"kiosk control off", func(c *homedash.RuntimeConfig) { c.KioskAllowControl = false }, cmd("fa:light", homedash.CapToggle, nil, kiosk(false)), homedash.CodeViewOnly, ""},
		{"kiosk disabled", func(c *homedash.RuntimeConfig) { c.KioskEnabled = false }, cmd("fa:light", homedash.CapToggle, nil, kiosk(false)), homedash.CodeKioskDisabled, ""},
		{"kiosk without allowed set", nil, cmd("fa:light", homedash.CapToggle, nil, homedash.Principal{Kind: homedash.PrincipalKiosk, Key: "kd"}), homedash.CodeForbiddenEntity, ""},
		{"kiosk denied unlock", nil, unlockConfirmed, homedash.CodeForbiddenEntity, "kiosk.denied"},
		{"sensitive unconfirmed", nil, cmd("fa:lock", homedash.CapUnlock, nil, sess), homedash.CodeConfirmationRequired, ""},
		{"agent", nil, cmd("fa:light", homedash.CapToggle, nil, agent), homedash.CodeForbiddenEntity, "agent"},
		{"unknown principal", nil, cmd("fa:light", homedash.CapToggle, nil, homedash.Principal{Kind: "robot", Key: "r"}), homedash.CodeForbiddenEntity, "principal"},
		{"ok", nil, cmd("fa:light", homedash.CapToggle, nil, sess), "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fa := gateSource()
			h := newTestHub(t, tc.mut, fa)
			if tc.name == "no settings entry" {
				// newTestHub cannot wait for a source without a settings entry (it stays disabled).
				h.Reconfigure(homedash.RuntimeConfig{Enabled: true, KioskEnabled: true, KioskAllowControl: true, Sources: map[homedash.SourceID]homedash.SourceSettings{}})
				sourcetest.WaitFor(t, time.Second, func() bool { return h.Sources()[0].State == homedash.StatusDisabled }, "source disabled")
			}
			var outcomes []string
			homedash.SetAuditForTest(h, func(_ context.Context, _ homedash.Command, outcome string) { outcomes = append(outcomes, outcome) })
			_, err := h.Command(context.Background(), tc.c)
			var he *homedash.Error
			got, blocked := "", ""
			if errors.As(err, &he) {
				got, blocked = he.Code, he.BlockedBy
			} else if err != nil {
				got = "untyped"
			}
			if got != tc.code || blocked != tc.blockedBy {
				t.Fatalf("code %q blocked_by %q, want %q %q (%v)", got, blocked, tc.code, tc.blockedBy, err)
			}
			if tc.code == "" && (len(fa.Commands()) != 1 || !slices.Equal(outcomes, []string{"ok"})) {
				t.Fatalf("commands %d outcomes %v", len(fa.Commands()), outcomes)
			}
			if tc.code != "" && len(fa.Commands()) != 0 {
				t.Fatal("a refused command reached the source")
			}
		})
	}
}

func TestCommandRateLimitAndRequestIDDedupe(t *testing.T) {
	fa := gateSource()
	h := newTestHub(t, nil, fa)
	ctx := context.Background()
	first := cmd("fa:light", homedash.CapToggle, nil, session("s"))
	first.RequestID = "req-1"
	for i := 0; i < 2; i++ {
		if _, err := h.Command(ctx, first); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(fa.Commands()); n != 1 {
		t.Fatalf("duplicate request id reached the source %d times", n)
	}
	// Rate limits come before the dedupe lookup (backend section 5, steps 7 and 8), so the duplicate
	// counted too: 2 of 5 per entity and 5 s so far.
	for i := 0; i < 3; i++ {
		if _, err := h.Command(ctx, cmd("fa:light", homedash.CapToggle, nil, session("s"))); err != nil {
			t.Fatal(err)
		}
	}
	_, err := h.Command(ctx, cmd("fa:light", homedash.CapToggle, nil, session("s")))
	var he *homedash.Error
	if !errors.As(err, &he) || he.Code != homedash.CodeRateLimited || he.RetryAfterMS <= 0 {
		t.Fatalf("sixth command: %v", err)
	}
}

func TestCommandLatestWinsSlot(t *testing.T) {
	fa := gateSource()
	fa.CommandGate = make(chan struct{})
	h := newTestHub(t, nil, fa)
	queued := make(chan struct{}, 4)
	homedash.SetQueuedHookForTest(h, func() { queued <- struct{}{} })
	ctx := context.Background()
	send := func(v float64) chan homedash.CommandResult {
		out := make(chan homedash.CommandResult, 1)
		go func() {
			res, err := h.Command(ctx, cmd("fa:light", homedash.CapBrightness, map[string]any{"value": v}, session("s")))
			if err != nil {
				t.Errorf("value %v: %v", v, err)
			}
			out <- res
		}()
		return out
	}
	waitQueued := func() {
		t.Helper()
		select {
		case <-queued:
		case <-time.After(2 * time.Second):
			t.Fatal("command was not queued")
		}
	}
	a := send(10)
	sourcetest.WaitFor(t, time.Second, func() bool { return fa.InCommand() == 1 }, "first command in flight")
	b := send(20)
	waitQueued()
	c := send(30)
	waitQueued()
	if res := <-b; !res.Superseded {
		t.Fatal("queued value 20 was not superseded by 30")
	}
	close(fa.CommandGate)
	<-a
	if res := <-c; res.Superseded {
		t.Fatal("the latest value was reported as superseded")
	}
	sourcetest.WaitFor(t, time.Second, func() bool { return len(fa.Commands()) == 2 }, "two commands sent")
	got := fa.Commands()
	if got[0].Args["value"] != 10.0 || got[1].Args["value"] != 30.0 {
		t.Fatalf("sent %v then %v, want 10 then 30", got[0].Args["value"], got[1].Args["value"])
	}
}

func TestCommandSourcePanicIsCommandFailedWithoutUpstreamText(t *testing.T) {
	fa := gateSource()
	fa.PanicOnCommand = true
	h := newTestHub(t, nil, fa)
	var outcomes []string
	homedash.SetAuditForTest(h, func(_ context.Context, _ homedash.Command, outcome string) { outcomes = append(outcomes, outcome) })
	_, err := h.Command(context.Background(), cmd("fa:light", homedash.CapToggle, nil, session("s")))
	var he *homedash.Error
	if !errors.As(err, &he) || he.Code != homedash.CodeCommandFailed || strings.Contains(err.Error(), "upstream") {
		t.Fatalf("got %v", err)
	}
	if !slices.Equal(outcomes, []string{"failed"}) {
		t.Fatalf("outcomes %v", outcomes)
	}
}

func TestCommandSourceErrorTextIsNeverReported(t *testing.T) {
	fa := gateSource()
	fa.CommandErr = errors.New("secret upstream detail")
	h := newTestHub(t, nil, fa)
	_, err := h.Command(context.Background(), cmd("fa:light", homedash.CapToggle, nil, session("s")))
	var he *homedash.Error
	if !errors.As(err, &he) || he.Code != homedash.CodeCommandFailed || strings.Contains(err.Error(), "secret") {
		t.Fatalf("got %v", err)
	}
}

func TestCommandRequestIDAndHistoryValidation(t *testing.T) {
	fa := gateSource()
	h := newTestHub(t, nil, fa)
	bad := cmd("fa:light", homedash.CapToggle, nil, session("s"))
	bad.RequestID = strings.Repeat("x", 65)
	var he *homedash.Error
	if _, err := h.Command(context.Background(), bad); !errors.As(err, &he) || he.Code != homedash.CodeBadRequest {
		t.Fatalf("overlong request id: %v", err)
	}
	now := time.Now()
	if _, err := h.History(context.Background(), "fa:light", now, now.Add(-time.Hour), 10); !errors.As(err, &he) || he.Code != homedash.CodeBadRequest {
		t.Fatalf("inverted range: %v", err)
	}
	s, err := h.History(context.Background(), "fa:light", now.Add(-time.Hour), now, 10)
	if err != nil || s.Origin != "memory" || s.Points == nil {
		t.Fatalf("memory history: %+v %v", s, err)
	}
}

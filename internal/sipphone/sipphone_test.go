package sipphone

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"

	"github.com/emiago/diago"
	"github.com/emiago/diago/audio"
	"github.com/emiago/diago/diagotest"
	"github.com/emiago/sipgo/sip"
)

func TestNormalizeSIPURIAndDestinationPolicy(t *testing.T) {
	uri, canonical, err := NormalizeSIPURI("sip:+491234@example.COM")
	if err != nil {
		t.Fatal(err)
	}
	if canonical != "sip:+491234@example.com" {
		t.Fatalf("canonical URI = %q", canonical)
	}
	allowed := config.SIPOutboundConfig{
		AllowedDomains: []string{"example.com"}, AllowedE164Prefixes: []string{"+49"},
	}
	if !DestinationAllowed(allowed, uri) {
		t.Fatal("expected E.164 destination to be allowed")
	}
	for _, invalid := range []string{
		"sips:user@example.com", "sip:user:secret@example.com", "sip:user@example.com?Subject=x", "sip:user@example.com\r\nX: injected",
	} {
		if _, _, err := NormalizeSIPURI(invalid); err == nil {
			t.Fatalf("expected %q to be rejected", invalid)
		}
	}
	if DestinationAllowed(config.SIPOutboundConfig{}, uri) {
		t.Fatal("empty destination allowlists must deny")
	}
}

func TestCallerAllowedRequiresPeerAndIdentity(t *testing.T) {
	cfg := config.SIPInboundConfig{
		TrustedPeerCIDRs: []string{"192.168.10.0/24"},
		AllowedCallers:   []string{"sip:alice@example.com"},
	}
	from := sip.Uri{Scheme: "sip", User: "alice", Host: "EXAMPLE.COM"}
	if !CallerAllowed(cfg, "192.168.10.5:5060", from) {
		t.Fatal("trusted peer and allowed caller should pass")
	}
	if CallerAllowed(cfg, "192.168.11.5:5060", from) {
		t.Fatal("untrusted peer must fail even with allowed From")
	}
	if CallerAllowed(cfg, "192.168.10.5:5060", sip.Uri{Scheme: "sip", User: "mallory", Host: "example.com"}) {
		t.Fatal("unlisted caller must fail even from trusted peer")
	}
	cfg.TrustedPeerCIDRs = []string{"192.168.10.5"}
	if !CallerAllowed(cfg, "192.168.10.5:5060", from) {
		t.Fatal("exact trusted peer IP should pass")
	}
}

func TestValidateRequestRejectsHeaderInjection(t *testing.T) {
	raw := strings.Join([]string{
		"INVITE sip:bob@example.com SIP/2.0",
		"Via: SIP/2.0/UDP 192.0.2.1:5060;branch=z9hG4bK-test",
		"From: <sip:alice@example.com>;tag=123",
		"To: <sip:bob@example.com>",
		"Call-ID: safe-call-id",
		"CSeq: 1 INVITE",
		"Content-Length: 0", "", "",
	}, "\r\n")
	message, err := sip.ParseMessage([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	req := message.(*sip.Request)
	if err := ValidateRequest(req); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	req.AppendHeader(sip.NewHeader("X-Test", "safe\r\nInjected: true"))
	if err := ValidateRequest(req); err == nil {
		t.Fatal("expected CRLF header injection to be rejected")
	}
}

func TestStateTransitionsAndRegistrationBackoff(t *testing.T) {
	if !validTransition(StateRegistered, StateConnecting) || !validTransition(StateActive, StateEnding) {
		t.Fatal("expected normal call transitions")
	}
	if validTransition(StateDisabled, StateActive) || validTransition(StateEnded, StateActive) {
		t.Fatal("invalid transition accepted")
	}
	if got := registrationBackoff(1); got != time.Second {
		t.Fatalf("first backoff = %s", got)
	}
	if got := registrationBackoff(99); got != 5*time.Minute {
		t.Fatalf("capped backoff = %s", got)
	}
}

func TestOutboundInviteUsesConfiguredCallerIdentity(t *testing.T) {
	cfg := validTestSIPConfig()
	cfg.DisplayName = "AuraGo Phone"
	options := outboundInviteOptions(cfg, nil)
	if len(options.Headers) != 1 {
		t.Fatalf("outbound headers = %d, want 1", len(options.Headers))
	}
	from, ok := options.Headers[0].(*sip.FromHeader)
	if !ok || from.DisplayName != cfg.DisplayName || from.Address.User != cfg.Username || from.Address.Host != cfg.Domain {
		t.Fatalf("unexpected caller identity: %#v", options.Headers[0])
	}
}

func TestManagerRejectsSecondOutboundCall(t *testing.T) {
	cfg := validTestSIPConfig()
	manager, err := NewManager(cfg, t.TempDir(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		manager.active = nil
		_ = manager.Close()
	}()
	manager.endpoint = &diago.Diago{}
	manager.active = &activeCall{record: CallRecord{ID: "existing"}}
	_, err = manager.Dial(context.Background(), "sip:alice@example.com")
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("second call error = %v", err)
	}
}

func TestManagerRespondsBusyToSecondInboundCall(t *testing.T) {
	cfg := validTestSIPConfig()
	cfg.Permissions.AnswerInbound = true
	cfg.Inbound.TrustedPeerCIDRs = []string{"192.0.2.10"}
	cfg.Inbound.AllowedCallers = []string{"alice"}
	manager, err := NewManager(cfg, t.TempDir(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.active = &activeCall{record: CallRecord{ID: "existing"}}
	req, err := diagotest.NewRequest(sip.INVITE, sip.Uri{Scheme: "sip", User: "aurago", Host: "example.com"})
	if err != nil {
		t.Fatal(err)
	}
	req.SetSource("192.0.2.10:5060")
	req.From().Address.User = "alice"
	dialog, recorder, err := diagotest.NewDialogServerSession(req)
	if err != nil {
		t.Fatal(err)
	}
	terminate := time.AfterFunc(100*time.Millisecond, recorder.Terminate)
	defer terminate.Stop()
	manager.handleIncoming(dialog)
	responses := recorder.Result()
	if len(responses) == 0 || responses[len(responses)-1].StatusCode != sip.StatusBusyHere {
		t.Fatalf("responses = %#v, want 486 Busy Here", responses)
	}
}

func TestG711RoundTrip(t *testing.T) {
	linear := make([]byte, 320)
	for i := 0; i < len(linear); i += 2 {
		linear[i] = byte(i)
		linear[i+1] = byte(i >> 1)
	}
	for _, codec := range []string{"pcma", "pcmu"} {
		encoded := make([]byte, 160)
		decoded := make([]byte, 320)
		var err error
		if codec == "pcma" {
			_, err = audio.EncodeAlawTo(encoded, linear)
			if err == nil {
				_, err = audio.DecodeAlawTo(decoded, encoded)
			}
		} else {
			_, err = audio.EncodeUlawTo(encoded, linear)
			if err == nil {
				_, err = audio.DecodeUlawTo(decoded, encoded)
			}
		}
		if err != nil || len(decoded) != len(linear) {
			t.Fatalf("%s round trip failed: %v", codec, err)
		}
	}
}

func TestStorePersistsPrivacySafeCallRecord(t *testing.T) {
	store, err := OpenStore(t.TempDir() + "/sip_calls.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	record := CallRecord{ID: "call-1", Direction: "outbound", RemoteParty: "sip:alice@example.com", StartedAt: time.Now().UTC(), State: StateConnecting, Backend: "classic"}
	if err := store.Upsert(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	calls, err := store.List(context.Background(), 10)
	if err != nil || len(calls) != 1 || calls[0].RemoteParty != record.RemoteParty {
		t.Fatalf("calls=%v err=%v", calls, err)
	}
}

func validTestSIPConfig() config.SIPConfig {
	var cfg config.SIPConfig
	config.ApplySIPDefaults(&cfg)
	cfg.Enabled = true
	cfg.ReadOnly = false
	cfg.Registrar = "example.com"
	cfg.Domain = "example.com"
	cfg.Username = "aurago"
	cfg.Password = "runtime-secret"
	cfg.Permissions.OriginateOutbound = true
	cfg.Outbound.AllowedDomains = []string{"example.com"}
	cfg.Outbound.AllowedUsers = []string{"alice"}
	return cfg
}

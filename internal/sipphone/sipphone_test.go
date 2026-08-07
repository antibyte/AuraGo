package sipphone

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/voice"

	"github.com/emiago/diago"
	"github.com/emiago/diago/audio"
	"github.com/emiago/diago/diagotest"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

type recordingDialog struct {
	ctx     context.Context
	cancel  context.CancelFunc
	hangups atomic.Int32
	closes  atomic.Int32
}

type recordingMediaPeer struct {
	attaches atomic.Int32
	detaches atomic.Int32
}

type testVoiceBackend struct{}

func (testVoiceBackend) Start(context.Context, voice.CallContext, voice.DuplexAudio) (voice.VoiceSession, error) {
	return nil, errors.New("test backend must not start")
}

func readyTestBackendFactory(context.Context, config.SIPVoiceConfig) (voice.VoiceBackend, error) {
	return testVoiceBackend{}, nil
}

type blockingDTMFWriter struct {
	started chan struct{}
	release chan struct{}
}

func (w *blockingDTMFWriter) WriteDTMF(rune) error {
	close(w.started)
	<-w.release
	return nil
}

func (p *recordingMediaPeer) Attach(context.Context, string, voice.DuplexAudio) error {
	p.attaches.Add(1)
	return nil
}

func (p *recordingMediaPeer) Detach(string) {
	p.detaches.Add(1)
}

func newRecordingDialog() *recordingDialog {
	ctx, cancel := context.WithCancel(context.Background())
	return &recordingDialog{ctx: ctx, cancel: cancel}
}

func (d *recordingDialog) Id() string                { return "recording-dialog" }
func (d *recordingDialog) Context() context.Context  { return d.ctx }
func (d *recordingDialog) Media() *diago.DialogMedia { return nil }
func (d *recordingDialog) DialogSIP() *sipgo.Dialog  { return nil }
func (d *recordingDialog) Do(context.Context, *sip.Request) (*sip.Response, error) {
	return nil, nil
}
func (d *recordingDialog) Hangup(context.Context) error {
	d.hangups.Add(1)
	d.cancel()
	return nil
}
func (d *recordingDialog) Close() error {
	d.closes.Add(1)
	d.cancel()
	return nil
}

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
	if !DestinationAllowed(allowed, "example.com", uri) {
		t.Fatal("expected E.164 destination to be allowed")
	}
	for _, invalid := range []string{
		"sips:user@example.com", "sip:user:secret@example.com", "sip:user@example.com?Subject=x", "sip:user@example.com\r\nX: injected",
	} {
		if _, _, err := NormalizeSIPURI(invalid); err == nil {
			t.Fatalf("expected %q to be rejected", invalid)
		}
	}
	if !DestinationAllowed(config.SIPOutboundConfig{}, "example.com", uri) {
		t.Fatal("empty destination allowlists must allow the account domain")
	}
	foreign, _, err := NormalizeSIPURI("sip:+491234@other.example")
	if err != nil {
		t.Fatal(err)
	}
	if DestinationAllowed(config.SIPOutboundConfig{}, "example.com", foreign) {
		t.Fatal("empty domain allowlist widened access beyond the account domain")
	}
}

func TestDestinationPolicyRequiresExactAllowsAndKeepsDenyPrecedence(t *testing.T) {
	cfg := config.SIPOutboundConfig{
		AllowedDomains: []string{"branch.example.com"},
		DeniedDomains:  []string{"premium.example.com"},
		AllowedUsers:   []string{"sales-123", "desk-ab"},
		DeniedUsers:    []string{"sales-999"},
	}
	for _, test := range []struct {
		raw     string
		allowed bool
	}{
		{"sip:sales-123@branch.example.com", true},
		{"sip:desk-ab@branch.example.com", true},
		{"sip:desk-abc@branch.example.com", false},
		{"sip:sales-999@branch.example.com", false},
		{"sip:sales-123@premium.example.com", false},
	} {
		uri, _, err := NormalizeSIPURI(test.raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := DestinationAllowed(cfg, "example.com", uri); got != test.allowed {
			t.Errorf("DestinationAllowed(%q) = %v, want %v", test.raw, got, test.allowed)
		}
	}
}

func TestDestinationPolicyLegacyWildcardAllowsRequireMigrationAndGrantNothing(t *testing.T) {
	cfg := config.SIPOutboundConfig{AllowedDomains: []string{"*.example.com"}, AllowedUsers: []string{"sales-*"}}
	uri, _, err := NormalizeSIPURI("sip:sales-123@branch.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if DestinationAllowed(cfg, "example.com", uri) {
		t.Fatal("legacy wildcard allow granted outbound access")
	}
	if !OutboundPolicyMigrationRequired(cfg) {
		t.Fatal("legacy wildcard allow did not request migration")
	}
}

func TestDestinationPolicyUniversalWildcardMigratesToProviderScope(t *testing.T) {
	var cfg config.SIPConfig
	config.ApplySIPDefaults(&cfg)
	cfg.Domain = "example.com"
	cfg.Outbound.AllowedDomains = []string{"*"}
	cfg.Outbound.AllowedUsers = []string{"*"}
	config.NormalizeSIPConfig(&cfg)
	if OutboundPolicyMigrationRequired(cfg.Outbound) {
		t.Fatal("universal wildcard still requires manual migration")
	}
	uri, _, err := NormalizeSIPURI("sip:any-extension@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if !DestinationAllowed(cfg.Outbound, cfg.Domain, uri) {
		t.Fatal("migrated universal wildcard did not permit the provider destination")
	}
}

func TestDestinationPolicyDeniedE164PrefixOverridesAllow(t *testing.T) {
	uri, _, err := NormalizeSIPURI("sip:+49900123@example.com")
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.SIPOutboundConfig{
		AllowedDomains:      []string{"example.com"},
		AllowedE164Prefixes: []string{"+49"},
		DeniedE164Prefixes:  []string{"+49900"},
	}
	if DestinationAllowed(cfg, "example.com", uri) {
		t.Fatal("denied E.164 prefix must override the allowed prefix")
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

func TestCallerPolicySupportsWildcardsAndDenyPrecedence(t *testing.T) {
	cfg := config.SIPInboundConfig{
		TrustedPeerCIDRs: []string{"192.168.10.0/24"},
		AllowedCallers:   []string{"sip:+49*@*.example.com"},
		DeniedCallers:    []string{"sip:+49900*@*.example.com"},
	}
	if !CallerAllowed(cfg, "192.168.10.5:5060", sip.Uri{Scheme: "sip", User: "+491701234567", Host: "pbx.example.com"}) {
		t.Fatal("wildcard caller should pass")
	}
	if CallerAllowed(cfg, "192.168.10.5:5060", sip.Uri{Scheme: "sip", User: "+49900123", Host: "pbx.example.com"}) {
		t.Fatal("denied caller wildcard must override the allowlist")
	}
}

func TestCallerPolicyCanonicalizesNumbersAndKeepsFritzInternalLiteral(t *testing.T) {
	base := config.SIPInboundConfig{TrustedPeerCIDRs: []string{"192.168.10.5"}, NumberRegion: "DE"}
	base.AllowedCallers = []string{"01701234567"}
	if !CallerAllowed(base, "192.168.10.5:5060", sip.Uri{Scheme: "sip", User: "+491701234567", Host: "fritz.box"}) {
		t.Fatal("national caller was not canonicalized with the explicit region")
	}
	base.AllowedCallers = []string{"00491701234567"}
	if !CallerAllowed(base, "192.168.10.5:5060", sip.Uri{Scheme: "sip", User: "+491701234567", Host: "fritz.box"}) {
		t.Fatal("00 caller form was not canonicalized")
	}
	base.AllowedCallers = []string{"**610"}
	if !CallerAllowed(base, "192.168.10.5:5060", sip.Uri{Scheme: "sip", User: "**610", Host: "fritz.box"}) {
		t.Fatal("FRITZ!Box internal number was not matched literally")
	}
	if CallerAllowed(base, "192.168.10.5:5060", sip.Uri{Scheme: "sip", User: "9910", Host: "fritz.box"}) {
		t.Fatal("FRITZ!Box internal number was treated as a wildcard")
	}
}

func TestRegisterOptionsUseNegotiatedExpiryRefresh(t *testing.T) {
	var cfg config.SIPConfig
	config.ApplySIPDefaults(&cfg)
	if got := registerOptions(cfg, nil).RetryInterval; got != 0 {
		t.Fatalf("RetryInterval = %v, want zero for negotiated 75%% refresh", got)
	}
	err := &diago.RegisterResponseError{RegisterRes: sip.NewResponse(403, "Forbidden")}
	code, status := classifyRegistrationError(err)
	if code != "registration_failed_403" || status != 403 {
		t.Fatalf("registration classification = %q/%d", code, status)
	}
}

func TestStoreMigratesV1AndTracksTransientSessions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sip_calls.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE sip_calls (
id TEXT PRIMARY KEY, direction TEXT NOT NULL, remote_party TEXT NOT NULL, started_at INTEGER NOT NULL,
answered_at INTEGER, ended_at INTEGER, state TEXT NOT NULL, end_reason TEXT NOT NULL DEFAULT '',
backend TEXT NOT NULL, session_id TEXT NOT NULL DEFAULT ''); PRAGMA user_version=1;`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	var version int
	if err := store.db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil || version != 3 {
		t.Fatalf("schema version = %d, err=%v", version, err)
	}
	call := CallRecord{ID: "call-1", Direction: "inbound", StartedAt: time.Now(), State: StateEnded, Backend: "classic", SessionID: "sip-call-1", persistTranscripts: false}
	if err := store.Upsert(context.Background(), call); err != nil {
		t.Fatal(err)
	}
	var mediaMode string
	if err := store.db.QueryRow(`SELECT media_mode FROM sip_calls WHERE id=?`, call.ID).Scan(&mediaMode); err != nil || mediaMode != MediaModeAgent {
		t.Fatalf("media mode = %q, err=%v", mediaMode, err)
	}
	sessions, err := store.NonPersistentSessionIDs(context.Background())
	if err != nil || len(sessions) != 1 || sessions[0] != call.SessionID {
		t.Fatalf("transient sessions = %v, err=%v", sessions, err)
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
	if !validTransition(StateConnecting, StateRinging) {
		t.Fatal("outbound SIP progress must transition from connecting to ringing")
	}
	if !validTransition(StateRinging, StateActive) {
		t.Fatal("answered inbound and outbound SIP calls must transition from ringing to active")
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

func TestOutboundRingingStatus(t *testing.T) {
	for _, statusCode := range []int{sip.StatusRinging, sip.StatusSessionInProgress} {
		if !isOutboundRingingStatus(statusCode) {
			t.Fatalf("status %d must surface outbound ringing", statusCode)
		}
	}
	for _, statusCode := range []int{sip.StatusTrying, sip.StatusCallIsForwarded, sip.StatusOK} {
		if isOutboundRingingStatus(statusCode) {
			t.Fatalf("status %d must not claim the other phone is ringing", statusCode)
		}
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
	manager, err := NewManager(cfg, t.TempDir(), readyTestBackendFactory, nil, nil)
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

func TestReconfigureReservationBlocksCallsAndIsIdempotent(t *testing.T) {
	cfg := validTestSIPConfig()
	manager, err := NewManager(cfg, t.TempDir(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.endpoint = &diago.Diago{}
	release, err := manager.ReserveReconfigure()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Dial(context.Background(), "sip:alice@example.com"); !errors.Is(err, ErrBusy) {
		t.Fatalf("Dial during reconfigure = %v", err)
	}
	if _, err := manager.ReserveReconfigure(); !errors.Is(err, ErrBusy) {
		t.Fatalf("second reservation = %v", err)
	}
	release()
	release()
	if nextRelease, err := manager.ReserveReconfigure(); err != nil {
		t.Fatalf("reservation after release = %v", err)
	} else {
		nextRelease()
	}
}

func TestRegistrationFailureDoesNotOverwriteActiveCallState(t *testing.T) {
	cfg := validTestSIPConfig()
	manager, err := NewManager(cfg, t.TempDir(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		manager.active = nil
		_ = manager.Close()
	}()
	manager.active = &activeCall{record: CallRecord{ID: "active"}}
	manager.state = StateActive
	manager.registered = true
	manager.registrationFailed(context.Background(), 1, errors.New("registration failed"))
	status := manager.Status()
	if status.State != StateActive || status.Registered || status.RegistrationError != "registration_failed" {
		t.Fatalf("status after registration failure = %+v", status)
	}
}

type failingCallStore struct{ err error }

func (s failingCallStore) Upsert(context.Context, CallRecord) error         { return s.err }
func (s failingCallStore) List(context.Context, int) ([]CallRecord, error)  { return nil, s.err }
func (s failingCallStore) DeleteOlderThan(context.Context, time.Time) error { return s.err }
func (s failingCallStore) DeleteAll(context.Context) error                  { return s.err }
func (s failingCallStore) Close() error                                     { return nil }

func TestCallHistoryFailureReportsIssueWithoutEndingLiveCall(t *testing.T) {
	reported := make(chan string, 1)
	manager := &Manager{
		logger: slog.Default(),
		store:  failingCallStore{err: errors.New("disk unavailable")},
		issueReporter: func(_ context.Context, fingerprint, _ string) {
			reported <- fingerprint
		},
		active: &activeCall{record: CallRecord{ID: "active"}},
		state:  StateActive,
	}
	manager.persistCall(CallRecord{ID: "active"}, "connected")
	select {
	case fingerprint := <-reported:
		if fingerprint != "sip_call_history_persist_failed" {
			t.Fatalf("fingerprint = %q", fingerprint)
		}
	case <-time.After(time.Second):
		t.Fatal("operational issue was not reported")
	}
	if manager.active == nil || manager.state != StateActive {
		t.Fatal("history failure ended or replaced the live call")
	}
}

func TestManagerAnswerBrowserAssignsExclusivePeer(t *testing.T) {
	cfg := validTestSIPConfig()
	cfg.BrowserMedia.Enabled = true
	cfg.Inbound.Route = "manual"
	cfg.Permissions.AnswerInbound = true
	manager, err := NewManager(cfg, t.TempDir(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	peer := &recordingMediaPeer{}
	call := &activeCall{
		record:       CallRecord{ID: "incoming", Backend: cfg.Voice.Backend},
		serverDialog: &diago.DialogServerSession{},
		decision:     make(chan string, 1),
	}
	manager.active = call
	if err := manager.AnswerBrowser("incoming", peer); err != nil {
		t.Fatal(err)
	}
	if call.mediaMode != MediaModeBrowser || call.mediaPeer != peer || call.record.Backend != MediaModeBrowser {
		t.Fatalf("browser peer was not assigned exclusively: %#v", call)
	}
	select {
	case decision := <-call.decision:
		if decision != "answer" {
			t.Fatalf("decision = %q", decision)
		}
	default:
		t.Fatal("answer decision was not queued")
	}
}

func TestManagerBrowserAnswerRequiresManualRouteAndEnabledMedia(t *testing.T) {
	cfg := validTestSIPConfig()
	cfg.Permissions.AnswerInbound = true
	manager, err := NewManager(cfg, t.TempDir(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.active = &activeCall{
		record:       CallRecord{ID: "incoming"},
		serverDialog: &diago.DialogServerSession{},
		decision:     make(chan string, 1),
	}
	if err := manager.AnswerBrowser("incoming", &recordingMediaPeer{}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("browser answer error = %v", err)
	}
}

func TestManagerBrowserMediaFailureCancelsMatchingCall(t *testing.T) {
	cfg := validTestSIPConfig()
	manager, err := NewManager(cfg, t.TempDir(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	callCtx, cancel := context.WithCancel(context.Background())
	call := &activeCall{
		record:    CallRecord{ID: "browser-call"},
		ctx:       callCtx,
		cancel:    cancel,
		mediaMode: MediaModeBrowser,
	}
	manager.active = call
	manager.BrowserMediaFailed("browser-call")
	if call.terminalReason != "browser_media_error" {
		t.Fatalf("terminal reason = %q", call.terminalReason)
	}
	select {
	case <-callCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("browser media failure did not cancel the call")
	}
}

func TestManagerFinishDetachesBrowserPeerExactlyOnce(t *testing.T) {
	cfg := validTestSIPConfig()
	manager, err := NewManager(cfg, t.TempDir(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	dialog := newRecordingDialog()
	peer := &recordingMediaPeer{}
	callCtx, cancel := context.WithCancel(context.Background())
	call := &activeCall{
		record:            CallRecord{ID: "browser-call", StartedAt: time.Now().UTC()},
		dialog:            dialog,
		ctx:               callCtx,
		cancel:            cancel,
		bridge:            voice.NewBridge(2),
		mediaMode:         MediaModeBrowser,
		mediaPeer:         peer,
		dialogEstablished: true,
		done:              make(chan struct{}),
	}
	manager.active = call
	manager.finishCall(call, "local_hangup")
	manager.finishCall(call, "duplicate")
	if got := peer.detaches.Load(); got != 1 {
		t.Fatalf("peer detaches = %d, want 1", got)
	}
	if got := dialog.hangups.Load(); got != 1 {
		t.Fatalf("dialog hangups = %d, want 1", got)
	}
	if got := dialog.closes.Load(); got != 1 {
		t.Fatalf("dialog closes = %d, want 1", got)
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

func TestSpeechLabStackReservationBlocksOutboundAndInboundAdmission(t *testing.T) {
	cfg := validTestSIPConfig()
	cfg.Permissions.AnswerInbound = true
	cfg.Inbound.TrustedPeerCIDRs = []string{"192.0.2.10"}
	cfg.Inbound.AllowedCallers = []string{"alice"}
	manager, err := NewManager(cfg, t.TempDir(), readyTestBackendFactory, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.endpoint = &diago.Diago{}

	release, err := manager.ReserveSpeechLabStackChange()
	if err != nil {
		t.Fatalf("reserve stack change: %v", err)
	}
	if _, err := manager.ReserveSpeechLabStackChange(); !errors.Is(err, ErrBusy) {
		t.Fatalf("second stack reservation error = %v, want ErrBusy", err)
	}
	if _, err := manager.Dial(context.Background(), "sip:alice@example.com"); !errors.Is(err, ErrBusy) {
		t.Fatalf("outbound admission error = %v, want ErrBusy", err)
	}

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
	manager.handleIncoming(dialog)
	responses := recorder.Result()
	if len(responses) == 0 || responses[len(responses)-1].StatusCode != sip.StatusTemporarilyUnavailable {
		t.Fatalf("responses = %#v, want 480 during stack reservation", responses)
	}

	release()
	release() // idempotent release must not make a later reservation unsafe.
	nextRelease, err := manager.ReserveSpeechLabStackChange()
	if err != nil {
		t.Fatalf("reservation was not released: %v", err)
	}
	nextRelease()
}

func TestAutoAnswerStopsImmediatelyOnRemoteCancel(t *testing.T) {
	cfg := validTestSIPConfig()
	cfg.Permissions.AnswerInbound = true
	cfg.Inbound.Route = "agent"
	cfg.Inbound.AutoAnswerDelayMS = 5000
	cfg.Inbound.TrustedPeerCIDRs = []string{"192.0.2.10"}
	cfg.Inbound.AllowedCallers = []string{"alice"}
	manager, err := NewManager(cfg, t.TempDir(), readyTestBackendFactory, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.rootCtx = context.Background()
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
	done := make(chan struct{})
	go func() {
		manager.handleIncoming(dialog)
		close(done)
	}()
	deadline := time.Now().Add(time.Second)
	for manager.Status().State != StateRinging && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if manager.Status().State != StateRinging {
		t.Fatal("incoming call did not enter ringing state")
	}
	cancelReq := sip.NewRequest(sip.CANCEL, req.Recipient)
	cancelReq.AppendHeader(sip.HeaderClone(req.Via()))
	cancelReq.AppendHeader(sip.HeaderClone(req.From()))
	cancelReq.AppendHeader(sip.HeaderClone(req.To()))
	cancelReq.AppendHeader(sip.HeaderClone(req.CallID()))
	if err := recorder.Receive(cancelReq); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("auto-answer delay ignored remote cancellation")
	}
	calls, err := manager.ListCalls(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].EndReason != "remote_cancel" {
		t.Fatalf("calls=%#v, want remote_cancel", calls)
	}
}

func TestHangupRejectsRingingInboundDialog(t *testing.T) {
	cfg := validTestSIPConfig()
	cfg.Permissions.AnswerInbound = true
	cfg.Permissions.AgentHangup = true
	cfg.Inbound.Route = "manual"
	cfg.Inbound.TrustedPeerCIDRs = []string{"192.0.2.10"}
	cfg.Inbound.AllowedCallers = []string{"alice"}
	manager, err := NewManager(cfg, t.TempDir(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.rootCtx = context.Background()
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
	done := make(chan struct{})
	go func() {
		manager.handleIncoming(dialog)
		close(done)
	}()
	deadline := time.Now().Add(time.Second)
	var callID string
	for time.Now().Before(deadline) {
		status := manager.Status()
		if status.ActiveCall != nil && status.State == StateRinging {
			callID = status.ActiveCall.ID
			break
		}
		time.Sleep(time.Millisecond)
	}
	if callID == "" {
		t.Fatal("incoming call did not enter ringing state")
	}
	terminate := time.AfterFunc(20*time.Millisecond, recorder.Terminate)
	defer terminate.Stop()
	if err := manager.Hangup(context.Background(), callID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("ringing inbound call did not end")
	}
	responses := recorder.Result()
	if len(responses) == 0 || responses[len(responses)-1].StatusCode != sip.StatusTemporarilyUnavailable {
		t.Fatalf("responses=%#v, want 480", responses)
	}
	calls, err := manager.ListCalls(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].EndReason != "local_hangup" {
		t.Fatalf("calls=%#v, want local_hangup", calls)
	}
}

func TestFinishCallTerminatesEstablishedDialogExactlyOnce(t *testing.T) {
	for _, tt := range []struct {
		name        string
		reason      string
		wantHangups int32
	}{
		{name: "local backend failure", reason: "voice_backend_error", wantHangups: 1},
		{name: "remote hangup", reason: "remote_hangup", wantHangups: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager(validTestSIPConfig(), t.TempDir(), nil, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer manager.Close()
			callCtx, cancel := context.WithCancel(context.Background())
			dialog := newRecordingDialog()
			call := &activeCall{
				record: CallRecord{ID: "call-1", Direction: "outbound", StartedAt: time.Now().UTC(), State: StateActive},
				dialog: dialog, dialogEstablished: true, ctx: callCtx, cancel: cancel,
				bridge: voice.NewBridge(1), done: make(chan struct{}),
			}
			manager.active = call
			manager.finishCall(call, tt.reason)
			manager.finishCall(call, "duplicate")
			if got := dialog.hangups.Load(); got != tt.wantHangups {
				t.Fatalf("hangups=%d, want %d", got, tt.wantHangups)
			}
			if got := dialog.closes.Load(); got != 1 {
				t.Fatalf("closes=%d, want 1", got)
			}
			if call.record.EndReason != tt.reason {
				t.Fatalf("end reason=%q, want %q", call.record.EndReason, tt.reason)
			}
		})
	}
}

func TestClassifyOutboundCallError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantReason string
		wantStatus int
	}{
		{
			name:       "authentication",
			err:        &sipgo.ErrDialogResponse{Res: sip.NewResponse(sip.StatusUnauthorized, "Unauthorized")},
			wantReason: "authentication_failed",
			wantStatus: sip.StatusUnauthorized,
		},
		{
			name:       "busy",
			err:        &sipgo.ErrDialogResponse{Res: sip.NewResponse(sip.StatusBusyHere, "Busy Here")},
			wantReason: "busy",
			wantStatus: sip.StatusBusyHere,
		},
		{
			name:       "provider unavailable",
			err:        &sipgo.ErrDialogResponse{Res: sip.NewResponse(sip.StatusServiceUnavailable, "Service Unavailable")},
			wantReason: "provider_unavailable",
			wantStatus: sip.StatusServiceUnavailable,
		},
		{
			name:       "timeout",
			err:        context.DeadlineExceeded,
			wantReason: "dial_timeout",
		},
		{
			name:       "generic",
			err:        errors.New("safe test failure"),
			wantReason: "dial_failed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reason, status := classifyOutboundCallError(test.err)
			if reason != test.wantReason || status != test.wantStatus {
				t.Fatalf("classification = (%q, %d), want (%q, %d)", reason, status, test.wantReason, test.wantStatus)
			}
		})
	}
}

func TestOutboundAgentCallPreflightBlocksBeforeInvite(t *testing.T) {
	cfg := validTestSIPConfig()
	ua, err := sipgo.NewUA()
	if err != nil {
		t.Fatal(err)
	}
	defer ua.Close()
	methods := make(chan sip.RequestMethod, 8)
	endpoint := diagotest.NewDiagoClientTest(ua, func(req *sip.Request) *sip.Response {
		methods <- req.Method
		response := sip.NewResponseFromRequest(req, sip.StatusOK, "OK", nil)
		if req.Method == sip.INVITE {
			response.SetBody(append([]byte(nil), req.Body()...))
			response.AppendHeader(sip.NewHeader("Content-Type", "application/sdp"))
			response.AppendHeader(&sip.ContactHeader{Address: sip.Uri{Scheme: "sip", User: "peer", Host: "127.0.0.1", Port: 5060}})
			response.To().Params.Add("tag", "remote")
		}
		return response
	})
	manager, err := NewManager(cfg, t.TempDir(), func(context.Context, config.SIPVoiceConfig) (voice.VoiceBackend, error) {
		return nil, errors.New("pipeline unavailable")
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.endpoint = endpoint
	manager.rootCtx = context.Background()
	if _, err := manager.Dial(context.Background(), "sip:alice@example.com"); err == nil || !strings.Contains(err.Error(), "preflight") {
		t.Fatalf("Dial error = %v, want telephone agent preflight blocker", err)
	}
	select {
	case method := <-methods:
		t.Fatalf("preflight blocker still sent SIP method %s", method)
	case <-time.After(50 * time.Millisecond):
	}
	calls, err := manager.ListCalls(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 || manager.Status().ActiveCall != nil {
		t.Fatalf("blocked preflight created a call: %#v", calls)
	}
}

func TestOutboundAgentCallPreflightDoesNotHoldManagerLockAndRejectsStaleConfig(t *testing.T) {
	cfg := validTestSIPConfig()
	started := make(chan struct{})
	release := make(chan struct{})
	manager, err := NewManager(cfg, t.TempDir(), func(context.Context, config.SIPVoiceConfig) (voice.VoiceBackend, error) {
		close(started)
		<-release
		return testVoiceBackend{}, nil
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.endpoint = &diago.Diago{}
	manager.rootCtx = context.Background()

	dialErr := make(chan error, 1)
	go func() {
		_, err := manager.Dial(context.Background(), "sip:alice@example.com")
		dialErr <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("telephone agent preflight did not start")
	}

	statusDone := make(chan struct{})
	go func() {
		_ = manager.Status()
		close(statusDone)
	}()
	select {
	case <-statusDone:
	case <-time.After(100 * time.Millisecond):
		close(release)
		t.Fatal("telephone agent preflight held the manager lock")
	}

	next := cfg
	next.Voice.AgentProviderID = "new-agent"
	manager.UpdateAgentConfig(next)
	close(release)
	if err := <-dialErr; !errors.Is(err, ErrBusy) {
		t.Fatalf("Dial error after config change = %v, want ErrBusy", err)
	}
	if status := manager.Status(); status.ActiveCall != nil {
		t.Fatalf("stale preflight published active call %#v", status.ActiveCall)
	}
}

func TestInternalEndCallBypassesAgentHangupPermission(t *testing.T) {
	cfg := validTestSIPConfig()
	cfg.Permissions.AgentHangup = false
	manager, err := NewManager(cfg, t.TempDir(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.rootCtx = context.Background()
	manager.mu.Lock()
	call := manager.newActiveCallLocked("inbound", "sip:alice@example.com")
	manager.mu.Unlock()

	if err := manager.Hangup(context.Background(), call.record.ID); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("permission-gated Hangup error = %v, want ErrPermissionDenied", err)
	}
	manager.EndCallInternal(call.record.ID, "inactivity_timeout")
	select {
	case <-call.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("internal call termination did not cancel the call")
	}
	manager.mu.Lock()
	reason := call.terminalReason
	manager.mu.Unlock()
	if reason != "inactivity_timeout" {
		t.Fatalf("internal end reason = %q", reason)
	}
	manager.finishCall(call, reason)
}

func TestSendDTMFDoesNotHoldManagerLock(t *testing.T) {
	cfg := validTestSIPConfig()
	cfg.Permissions.SendDTMF = true
	manager, err := NewManager(cfg, t.TempDir(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.rootCtx = context.Background()
	writer := &blockingDTMFWriter{started: make(chan struct{}), release: make(chan struct{})}
	manager.mu.Lock()
	call := manager.newActiveCallLocked("outbound", "sip:alice@example.com")
	call.media = &mediaPump{dtmfWriter: writer}
	manager.mu.Unlock()

	dtmfErr := make(chan error, 1)
	go func() {
		dtmfErr <- manager.SendDTMF(call.record.ID, '5')
	}()
	select {
	case <-writer.started:
	case <-time.After(time.Second):
		t.Fatal("DTMF send did not start")
	}
	statusDone := make(chan struct{})
	go func() {
		_ = manager.Status()
		close(statusDone)
	}()
	select {
	case <-statusDone:
	case <-time.After(100 * time.Millisecond):
		close(writer.release)
		t.Fatal("DTMF send held the manager lock")
	}
	close(writer.release)
	if err := <-dtmfErr; err != nil {
		t.Fatal(err)
	}
	manager.finishCall(call, "test_complete")
}

func TestUpdateAgentConfigKeepsActiveCallSnapshot(t *testing.T) {
	cfg := validTestSIPConfig()
	cfg.Inbound.Route = MediaModeAgent
	cfg.Inbound.AutoAnswerDelayMS = 1000
	cfg.Voice.AgentProviderID = "agent-old"
	cfg.Voice.AllowedTools = []string{"status"}
	cfg.Voice.PersistTranscripts = false
	manager, err := NewManager(cfg, t.TempDir(), readyTestBackendFactory, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	manager.rootCtx = context.Background()

	manager.mu.Lock()
	call := manager.newActiveCallLocked("inbound", "sip:alice@example.com")
	call.voiceBackend = testVoiceBackend{}
	oldCallContext := call.ctx
	manager.mu.Unlock()

	next := cfg
	next.Inbound.Route = "manual"
	next.Inbound.AutoAnswerDelayMS = 4500
	next.Voice.AgentProviderID = "agent-new"
	next.Voice.AllowedTools = []string{"workspace_search"}
	next.Voice.PersistTranscripts = true
	manager.UpdateAgentConfig(next)

	got := manager.Config()
	if got.Inbound.Route != "manual" || got.Inbound.AutoAnswerDelayMS != 4500 || got.Voice.AgentProviderID != "agent-new" {
		t.Fatalf("future telephone config was not updated: %+v", got)
	}
	if manager.Status().ActiveCall == nil || oldCallContext.Err() != nil {
		t.Fatal("telephone config update interrupted the active call")
	}
	if call.persistTranscripts || call.voiceBackend == nil {
		t.Fatalf("active call snapshot changed: persist=%v backend=%T", call.persistTranscripts, call.voiceBackend)
	}

	manager.finishCall(call, "test_complete")
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

package sipphone

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/voice"

	"github.com/emiago/diago"
	"github.com/emiago/diago/media"
	"github.com/emiago/sipgo"
	"github.com/emiago/sipgo/sip"
)

var rtpPortConfig struct {
	sync.Mutex
	configured bool
	start      int
	end        int
}

type Manager struct {
	mu              sync.Mutex
	cfg             config.SIPConfig
	logger          *slog.Logger
	store           *Store
	backendFactory  BackendFactory
	issueReporter   IssueReporter
	state           State
	registered      bool
	registrationErr string
	rootCtx         context.Context
	lifecycleCtx    context.Context
	cancel          context.CancelFunc
	ua              *sipgo.UserAgent
	endpoint        *diago.Diago
	active          *activeCall
	subscribers     map[uint64]chan Event
	nextSubscriber  uint64
	sequence        uint64
}

type activeCall struct {
	record         CallRecord
	dialog         diago.DialogSession
	serverDialog   *diago.DialogServerSession
	ctx            context.Context
	cancel         context.CancelFunc
	decision       chan string
	bridge         *voice.Bridge
	backend        voice.VoiceSession
	media          *mediaPump
	terminalReason string
}

func NewManager(cfg config.SIPConfig, dataDir string, backendFactory BackendFactory, reporter IssueReporter, logger *slog.Logger) (*Manager, error) {
	store, err := OpenStore(filepath.Join(dataDir, "sip_calls.db"))
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	state := StateDisabled
	if cfg.Enabled {
		state = StateRegistering
	}
	return &Manager{
		cfg: cfg, logger: logger, store: store, backendFactory: backendFactory,
		issueReporter: reporter, state: state, subscribers: make(map[uint64]chan Event),
	}, nil
}

func (m *Manager) Start(parent context.Context) error {
	m.mu.Lock()
	if m.lifecycleCtx == nil {
		m.lifecycleCtx = parent
	}
	if m.cancel != nil {
		m.mu.Unlock()
		return nil
	}
	if !m.cfg.Enabled {
		m.state = StateDisabled
		m.mu.Unlock()
		return nil
	}
	cfg := m.cfg
	m.mu.Unlock()
	if err := config.ValidateSIPRuntimeConfig(cfg); err != nil {
		return err
	}
	if err := configureRTPPorts(cfg.Media.RTPPortStart, cfg.Media.RTPPortEnd); err != nil {
		return err
	}

	tlsConfig, err := buildTLSConfig(cfg)
	if err != nil {
		return err
	}
	hostname := cfg.AdvertisedSignalingHost
	if hostname == "" {
		hostname = cfg.BindHost
	}
	protocolLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ua, err := sipgo.NewUA(
		// Diago also uses the UA name as the Contact URI user. The configured
		// account name is therefore required here for PBX registration routing.
		sipgo.WithUserAgent(cfg.Username),
		sipgo.WithUserAgentHostname(hostname),
		sipgo.WithUserAgentTransactionLayerOptions(sip.WithTransactionLayerLogger(protocolLogger)),
		sipgo.WithUserAgentTransportLayerOptions(sip.WithTransportLayerLogger(protocolLogger)),
	)
	if err != nil {
		return fmt.Errorf("create SIP user agent: %w", err)
	}
	transport := diago.Transport{
		Transport:       cfg.Transport,
		BindHost:        cfg.BindHost,
		BindPort:        cfg.BindPort,
		ExternalHost:    hostname,
		ExternalPort:    cfg.BindPort,
		MediaExternalIP: net.ParseIP(cfg.Media.AdvertisedHost),
		TLSConf:         tlsConfig,
	}
	endpoint := diago.NewDiago(ua,
		// Upstream debug logs may include complete protocol messages. Keep them
		// suppressed and emit only AuraGo's structured, sanitized state events.
		diago.WithLogger(protocolLogger),
		diago.WithTransport(transport),
		diago.WithMediaConfig(diago.MediaConfig{Codecs: configuredCodecs(cfg.Media.Codecs)}),
		diago.WithServerRequestMiddleware(validateRequestMiddleware),
	)
	rootCtx, cancel := context.WithCancel(parent)
	m.mu.Lock()
	m.ua = ua
	m.endpoint = endpoint
	m.rootCtx = rootCtx
	m.cancel = cancel
	m.state = StateRegistering
	m.emitLocked("status", nil, nil)
	m.mu.Unlock()
	if err := endpoint.ServeBackground(rootCtx, m.handleIncoming); err != nil {
		cancel()
		_ = ua.Close()
		m.mu.Lock()
		m.ua = nil
		m.endpoint = nil
		m.cancel = nil
		m.state = StateFailed
		m.registrationErr = "listener_start_failed"
		m.emitLocked("status", nil, nil)
		m.mu.Unlock()
		return fmt.Errorf("start SIP listener: %w", err)
	}
	go m.registrationLoop(rootCtx, endpoint, cfg)
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	cancel := m.cancel
	ua := m.ua
	active := m.active
	m.cancel = nil
	m.ua = nil
	m.endpoint = nil
	m.registered = false
	m.state = StateDisabled
	m.registrationErr = ""
	m.emitLocked("status", nil, nil)
	m.mu.Unlock()
	if active != nil {
		if active.cancel != nil {
			active.cancel()
		}
		if active.dialog != nil {
			_ = active.dialog.Hangup(ctx)
		}
	}
	if cancel != nil {
		cancel()
	}
	if ua != nil {
		return ua.Close()
	}
	return nil
}

func (m *Manager) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = m.Stop(ctx)
	return m.store.Close()
}

func (m *Manager) Reconfigure(ctx context.Context, cfg config.SIPConfig) error {
	m.mu.Lock()
	parent := m.lifecycleCtx
	m.mu.Unlock()
	if parent == nil {
		parent = context.Background()
	}
	if err := m.Stop(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	return m.Start(parent)
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked()
}

func (m *Manager) Config() config.SIPConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

func (m *Manager) ListCalls(ctx context.Context, limit int) ([]CallRecord, error) {
	return m.store.List(ctx, limit)
}

func (m *Manager) PruneHistory(ctx context.Context, cutoff time.Time) error {
	return m.store.DeleteOlderThan(ctx, cutoff)
}

func (m *Manager) Dial(ctx context.Context, target string) (CallRecord, error) {
	m.mu.Lock()
	cfg := m.cfg
	endpoint := m.endpoint
	if !cfg.Enabled || endpoint == nil {
		m.mu.Unlock()
		return CallRecord{}, ErrDisabled
	}
	if cfg.ReadOnly {
		m.mu.Unlock()
		return CallRecord{}, ErrReadOnly
	}
	if !cfg.Permissions.OriginateOutbound {
		m.mu.Unlock()
		return CallRecord{}, ErrPermissionDenied
	}
	if m.active != nil {
		m.mu.Unlock()
		return CallRecord{}, ErrBusy
	}
	uri, canonical, err := NormalizeSIPURI(target)
	if err != nil || !DestinationAllowed(cfg.Outbound, uri) {
		m.mu.Unlock()
		return CallRecord{}, ErrPermissionDenied
	}
	call := m.newActiveCallLocked("outbound", canonical)
	m.state = StateConnecting
	call.record.State = StateConnecting
	_ = m.store.Upsert(context.Background(), call.record)
	m.emitLocked("call", &call.record, nil)
	m.mu.Unlock()
	go m.runOutbound(call, endpoint, uri, cfg)
	return call.record, nil
}

func (m *Manager) Answer(callID string) error { return m.decideInbound(callID, "answer") }
func (m *Manager) Reject(callID string) error { return m.decideInbound(callID, "reject") }

func (m *Manager) decideInbound(callID, decision string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cfg.ReadOnly {
		return ErrReadOnly
	}
	if !m.cfg.Permissions.AnswerInbound {
		return ErrPermissionDenied
	}
	if m.active == nil || m.active.record.ID != callID || m.active.serverDialog == nil {
		return ErrCallNotFound
	}
	select {
	case m.active.decision <- decision:
		return nil
	default:
		return fmt.Errorf("SIP call already has a pending decision")
	}
}

func (m *Manager) Hangup(ctx context.Context, callID string) error {
	m.mu.Lock()
	if m.cfg.ReadOnly {
		m.mu.Unlock()
		return ErrReadOnly
	}
	if !m.cfg.Permissions.AgentHangup {
		m.mu.Unlock()
		return ErrPermissionDenied
	}
	call := m.active
	if call == nil || call.record.ID != callID {
		m.mu.Unlock()
		return ErrCallNotFound
	}
	m.state = StateEnding
	call.record.State = StateEnding
	m.emitLocked("call", &call.record, nil)
	m.mu.Unlock()
	call.cancel()
	if call.dialog != nil {
		return call.dialog.Hangup(ctx)
	}
	return nil
}

func (m *Manager) SendDTMF(callID string, digit rune) error {
	if !strings.ContainsRune("0123456789*#ABCD", digit) {
		return fmt.Errorf("invalid DTMF digit")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cfg.ReadOnly {
		return ErrReadOnly
	}
	if !m.cfg.Permissions.SendDTMF {
		return ErrPermissionDenied
	}
	if m.active == nil || m.active.record.ID != callID || m.active.media == nil {
		return ErrCallNotFound
	}
	return m.active.media.sendDTMF(digit)
}

func (m *Manager) TestConnection(ctx context.Context) error {
	m.mu.Lock()
	endpoint := m.endpoint
	cfg := m.cfg
	registered := m.registered
	m.mu.Unlock()
	if endpoint == nil || !cfg.Enabled {
		return ErrDisabled
	}
	if registered {
		return nil
	}
	uri, err := registrarURI(cfg)
	if err != nil {
		return err
	}
	tx, err := endpoint.RegisterTransaction(ctx, uri, registerOptions(cfg, nil))
	if err != nil {
		return fmt.Errorf("prepare SIP registration test: %w", err)
	}
	if err := tx.Register(ctx); err != nil {
		return fmt.Errorf("SIP registration test failed: %w", err)
	}
	defer func() {
		unregisterCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = tx.Unregister(unregisterCtx)
	}()
	return nil
}

func (m *Manager) Subscribe() (<-chan Event, func()) {
	m.mu.Lock()
	id := m.nextSubscriber
	m.nextSubscriber++
	ch := make(chan Event, 32)
	m.subscribers[id] = ch
	status := m.statusLocked()
	m.sequence++
	ch <- Event{Sequence: m.sequence, Type: "snapshot", Timestamp: time.Now().UTC(), Status: &status}
	m.mu.Unlock()
	return ch, func() {
		m.mu.Lock()
		delete(m.subscribers, id)
		m.mu.Unlock()
	}
}

func (m *Manager) registrationLoop(ctx context.Context, endpoint *diago.Diago, cfg config.SIPConfig) {
	uri, err := registrarURI(cfg)
	if err != nil {
		m.registrationFailed(ctx, 1, err)
		return
	}
	for attempt := 1; ctx.Err() == nil; attempt++ {
		m.mu.Lock()
		if m.active == nil {
			m.state = StateRegistering
		}
		m.emitLocked("status", nil, nil)
		m.mu.Unlock()
		err = endpoint.Register(ctx, uri, registerOptions(cfg, func() {
			m.mu.Lock()
			m.registered = true
			m.registrationErr = ""
			if m.active == nil {
				m.state = StateRegistered
			}
			m.emitLocked("status", nil, nil)
			m.mu.Unlock()
		}))
		if ctx.Err() != nil {
			return
		}
		m.registrationFailed(ctx, attempt, err)
		timer := time.NewTimer(registrationBackoff(attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (m *Manager) registrationFailed(ctx context.Context, attempt int, err error) {
	m.mu.Lock()
	m.registered = false
	m.state = StateFailed
	m.registrationErr = "registration_failed"
	m.emitLocked("registration_error", nil, map[string]any{"attempt": attempt})
	m.mu.Unlock()
	m.logger.Warn("SIP registration failed", "attempt", attempt, "error_type", fmt.Sprintf("%T", err))
	if attempt >= 3 && m.issueReporter != nil {
		m.issueReporter(ctx, "sip_registration_failed", "SIP registration repeatedly failed")
	}
}

func (m *Manager) handleIncoming(dialog *diago.DialogServerSession) {
	req := dialog.InviteRequest
	from := req.From()
	if from == nil {
		_ = dialog.Respond(sip.StatusBadRequest, "Bad Request", nil)
		return
	}
	m.mu.Lock()
	cfg := m.cfg
	if !CallerAllowed(cfg.Inbound, req.Source(), from.Address) {
		m.mu.Unlock()
		_ = dialog.Respond(sip.StatusForbidden, "Forbidden", nil)
		return
	}
	if !cfg.Enabled || cfg.ReadOnly || !cfg.Permissions.AnswerInbound {
		m.mu.Unlock()
		_ = dialog.Respond(sip.StatusTemporarilyUnavailable, "Temporarily Unavailable", nil)
		return
	}
	if m.active != nil {
		m.mu.Unlock()
		_ = dialog.Respond(sip.StatusBusyHere, "Busy Here", nil)
		return
	}
	remote := (&sip.Uri{Scheme: "sip", User: from.Address.User, Host: strings.ToLower(from.Address.Host)}).String()
	call := m.newActiveCallLocked("inbound", remote)
	call.serverDialog = dialog
	call.dialog = dialog
	call.record.State = StateRinging
	m.state = StateRinging
	_ = m.store.Upsert(context.Background(), call.record)
	m.emitLocked("call", &call.record, nil)
	m.mu.Unlock()
	if cfg.Inbound.Route == "reject" {
		_ = dialog.Respond(sip.StatusGlobalDecline, "Decline", nil)
		m.finishCall(call, "rejected")
		return
	}
	_ = dialog.Ringing()
	decision := "answer"
	if cfg.Inbound.Route == "manual" {
		select {
		case decision = <-call.decision:
		case <-call.ctx.Done():
			m.finishCall(call, "cancelled")
			return
		case <-dialog.Context().Done():
			m.finishCall(call, "remote_cancel")
			return
		}
	} else if cfg.Inbound.AutoAnswerDelayMS > 0 {
		select {
		case <-time.After(time.Duration(cfg.Inbound.AutoAnswerDelayMS) * time.Millisecond):
		case decision = <-call.decision:
		case <-call.ctx.Done():
			m.finishCall(call, "cancelled")
			return
		}
	}
	if decision == "reject" {
		_ = dialog.Respond(sip.StatusGlobalDecline, "Decline", nil)
		m.finishCall(call, "rejected")
		return
	}
	rtpNAT := media.RTPNATDisabled
	if cfg.Media.SymmetricRTP {
		rtpNAT = media.RTPNATSymetric
	}
	if err := dialog.AnswerOptions(diago.AnswerOptions{Codecs: configuredCodecs(cfg.Media.Codecs), RTPNAT: rtpNAT}); err != nil {
		m.finishCall(call, "answer_failed")
		return
	}
	m.runEstablished(call, cfg, cfg.Inbound.Route == "agent")
}

func (m *Manager) runOutbound(call *activeCall, endpoint *diago.Diago, uri sip.Uri, cfg config.SIPConfig) {
	if uri.UriParams == nil {
		uri.UriParams = sip.NewParams()
	}
	uri.UriParams.Add("transport", cfg.Transport)
	dialog, err := endpoint.NewDialog(uri, diago.NewDialogOptions{Transport: cfg.Transport})
	if err != nil {
		m.finishCall(call, "dial_failed")
		return
	}
	if cfg.OutboundProxy != "" {
		dialog.InviteRequest.SetDestination(cfg.OutboundProxy)
	}
	m.mu.Lock()
	call.dialog = dialog
	m.mu.Unlock()
	err = dialog.Invite(call.ctx, outboundInviteOptions(cfg, func(response *sip.Response) error {
		if response.StatusCode == sip.StatusRinging {
			m.updateCallState(call, StateRinging)
		}
		return nil
	}))
	if err != nil {
		m.finishCall(call, "dial_failed")
		return
	}
	if err := dialog.Ack(call.ctx); err != nil {
		m.finishCall(call, "ack_failed")
		return
	}
	m.runEstablished(call, cfg, true)
}

func (m *Manager) runEstablished(call *activeCall, cfg config.SIPConfig, useAgent bool) {
	now := time.Now().UTC()
	m.mu.Lock()
	call.record.AnsweredAt = &now
	call.record.State = StateActive
	m.state = StateActive
	_ = m.store.Upsert(context.Background(), call.record)
	m.emitLocked("call", &call.record, nil)
	m.mu.Unlock()
	pump := &mediaPump{dialog: call.dialog, bridge: call.bridge, jitterMS: cfg.Media.JitterBufferMS}
	pump.onDTMF = func(digit rune) { m.emitCallData(call, "dtmf", map[string]any{"digit": string(digit)}) }
	pump.onError = func(error) { m.cancelCallWithReason(call, "media_error") }
	m.mu.Lock()
	call.media = pump
	m.mu.Unlock()
	if err := pump.start(call.ctx); err != nil {
		m.finishCall(call, "media_error")
		return
	}
	if useAgent {
		if m.backendFactory == nil {
			m.finishCall(call, "voice_backend_error")
			return
		}
		backend, err := m.backendFactory(cfg.Voice)
		if err != nil {
			m.finishCall(call, "voice_backend_error")
			return
		}
		session, err := backend.Start(call.ctx, voice.CallContext{
			CallID: call.record.ID, Direction: call.record.Direction, RemoteParty: call.record.RemoteParty,
			Language: cfg.Voice.Language, SessionID: call.record.SessionID, AllowedTools: append([]string{}, cfg.Voice.AllowedTools...),
		}, call.bridge)
		if err != nil {
			m.finishCall(call, "voice_backend_error")
			return
		}
		m.mu.Lock()
		call.backend = session
		m.mu.Unlock()
		go m.forwardVoiceEvents(call, session)
	}
	select {
	case <-call.ctx.Done():
		m.finishCall(call, m.callCancellationReason(call))
	case <-call.dialog.Context().Done():
		m.finishCall(call, "remote_hangup")
	}
}

func (m *Manager) forwardVoiceEvents(call *activeCall, session voice.VoiceSession) {
	for event := range session.Events() {
		if event.Type == "voice_backend_error" {
			m.cancelCallWithReason(call, "voice_backend_error")
		}
		data := map[string]any{"voice_event": event.Type}
		if event.Data != nil {
			data["details"] = event.Data
		}
		m.emitCallData(call, "voice", data)
	}
}

func (m *Manager) cancelCallWithReason(call *activeCall, reason string) {
	m.mu.Lock()
	if call != nil && call.record.EndedAt == nil && call.terminalReason == "" {
		call.terminalReason = reason
	}
	m.mu.Unlock()
	if call != nil {
		call.cancel()
	}
}

func (m *Manager) callCancellationReason(call *activeCall) string {
	m.mu.Lock()
	reason := call.terminalReason
	m.mu.Unlock()
	if reason != "" {
		return reason
	}
	if call.ctx.Err() == context.DeadlineExceeded {
		return "max_duration"
	}
	return "local_hangup"
}

func (m *Manager) forwardBridgeEvents(call *activeCall) {
	for {
		select {
		case <-call.ctx.Done():
			return
		case event := <-call.bridge.Events():
			m.emitCallData(call, "media_queue", map[string]any{"voice_event": event.Type})
		}
	}
}

func (m *Manager) newActiveCallLocked(direction, remote string) *activeCall {
	ctx, cancel := context.WithTimeout(m.rootCtx, time.Duration(m.cfg.Voice.MaxCallDurationSeconds)*time.Second)
	id := randomCallID()
	call := &activeCall{
		record: CallRecord{ID: id, Direction: direction, RemoteParty: remote, StartedAt: time.Now().UTC(), State: StateConnecting, Backend: m.cfg.Voice.Backend, SessionID: "sip-" + id},
		ctx:    ctx, cancel: cancel, decision: make(chan string, 1), bridge: voice.NewBridge(25),
	}
	m.active = call
	go m.forwardBridgeEvents(call)
	return call
}

func (m *Manager) updateCallState(call *activeCall, state State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active != call || !validTransition(call.record.State, state) {
		return
	}
	call.record.State = state
	m.state = state
	_ = m.store.Upsert(context.Background(), call.record)
	m.emitLocked("call", &call.record, nil)
}

func (m *Manager) finishCall(call *activeCall, reason string) {
	call.cancel()
	call.bridge.Close()
	now := time.Now().UTC()
	m.mu.Lock()
	if call.record.EndedAt != nil {
		m.mu.Unlock()
		return
	}
	backend := call.backend
	call.record.EndedAt = &now
	call.record.State = StateEnded
	call.record.EndReason = reason
	_ = m.store.Upsert(context.Background(), call.record)
	m.emitLocked("call", &call.record, nil)
	if m.active == call {
		m.active = nil
	}
	if m.registered {
		m.state = StateRegistered
	} else if m.cfg.Enabled {
		m.state = StateRegistering
	} else {
		m.state = StateDisabled
	}
	m.emitLocked("status", nil, nil)
	m.mu.Unlock()
	if backend != nil {
		_ = backend.Close()
	}
}

func (m *Manager) emitCallData(call *activeCall, eventType string, data any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if call != nil {
		m.emitLocked(eventType, &call.record, data)
	}
}

func (m *Manager) emitLocked(eventType string, call *CallRecord, data any) {
	m.sequence++
	status := m.statusLocked()
	event := Event{Sequence: m.sequence, Type: eventType, Timestamp: time.Now().UTC(), Status: &status, Call: cloneCall(call), Data: data}
	for _, subscriber := range m.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (m *Manager) statusLocked() Status {
	status := Status{
		Enabled: m.cfg.Enabled, ReadOnly: m.cfg.ReadOnly, State: m.state, Registered: m.registered,
		RegistrationError: m.registrationErr, Transport: m.cfg.Transport,
		BindAddress: net.JoinHostPort(m.cfg.BindHost, fmt.Sprint(m.cfg.BindPort)), PasswordSet: m.cfg.Password != "",
	}
	if m.active != nil {
		status.ActiveCall = cloneCall(&m.active.record)
	}
	return status
}

func configuredCodecs(names []string) []media.Codec {
	codecs := make([]media.Codec, 0, len(names)+1)
	for _, name := range names {
		switch strings.ToLower(name) {
		case "pcma":
			codecs = append(codecs, media.CodecAudioAlaw)
		case "pcmu":
			codecs = append(codecs, media.CodecAudioUlaw)
		}
	}
	return append(codecs, media.CodecTelephoneEvent8000)
}

func configureRTPPorts(start, end int) error {
	rtpPortConfig.Lock()
	defer rtpPortConfig.Unlock()
	if rtpPortConfig.configured {
		if rtpPortConfig.start != start || rtpPortConfig.end != end {
			return fmt.Errorf("Diago RTP range is process-global and already set to %d-%d", rtpPortConfig.start, rtpPortConfig.end)
		}
		return nil
	}
	media.RTPPortStart = start
	media.RTPPortEnd = end
	rtpPortConfig.configured = true
	rtpPortConfig.start = start
	rtpPortConfig.end = end
	return nil
}

func buildTLSConfig(cfg config.SIPConfig) (*tls.Config, error) {
	if cfg.Transport != "tls" {
		return nil, nil
	}
	certificate, err := tls.LoadX509KeyPair(cfg.TLS.CertFile, cfg.TLS.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load SIP TLS certificate: %w", err)
	}
	serverName := cfg.TLS.ServerName
	if serverName == "" {
		serverName = hostWithoutPort(cfg.Registrar)
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName, Certificates: []tls.Certificate{certificate}}, nil
}

func validateRequestMiddleware(next sipgo.RequestHandler) sipgo.RequestHandler {
	return func(req *sip.Request, tx sip.ServerTransaction) {
		if err := ValidateRequest(req); err != nil {
			_ = tx.Respond(sip.NewResponseFromRequest(req, sip.StatusBadRequest, "Bad Request", nil))
			return
		}
		next(req, tx)
	}
}

func registrarURI(cfg config.SIPConfig) (sip.Uri, error) {
	raw := strings.TrimSpace(cfg.Registrar)
	if !strings.Contains(raw, ":") || (!strings.HasPrefix(strings.ToLower(raw), "sip:") && !strings.HasPrefix(strings.ToLower(raw), "sips:")) {
		raw = "sip:" + cfg.Username + "@" + raw
	}
	var uri sip.Uri
	if err := sip.ParseUri(raw, &uri); err != nil {
		return sip.Uri{}, fmt.Errorf("parse SIP registrar: %w", err)
	}
	if uri.User == "" {
		uri.User = cfg.Username
	}
	if uri.UriParams == nil {
		uri.UriParams = sip.NewParams()
	}
	uri.UriParams.Add("transport", cfg.Transport)
	return uri, nil
}

func registerOptions(cfg config.SIPConfig, onRegistered func()) diago.RegisterOptions {
	return diago.RegisterOptions{
		Username: authUsername(cfg), Password: cfg.Password, ProxyHost: cfg.OutboundProxy,
		Expiry: time.Duration(cfg.RegisterExpiresSeconds) * time.Second, RetryInterval: 5 * time.Second,
		OnRegistered: onRegistered,
	}
}

func authUsername(cfg config.SIPConfig) string {
	if cfg.AuthUsername != "" {
		return cfg.AuthUsername
	}
	return cfg.Username
}

func outboundInviteOptions(cfg config.SIPConfig, onResponse func(*sip.Response) error) diago.InviteClientOptions {
	options := diago.InviteClientOptions{
		Username:   authUsername(cfg),
		Password:   cfg.Password,
		OnResponse: onResponse,
	}
	options.WithCaller(cfg.DisplayName, cfg.Username, cfg.Domain)
	return options
}

func registrationBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Second << min(attempt-1, 9)
	return min(delay, 5*time.Minute)
}

func hostWithoutPort(raw string) string {
	raw = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(raw, "sip:"), "sips:"))
	if at := strings.LastIndex(raw, "@"); at >= 0 {
		raw = raw[at+1:]
	}
	if semi := strings.Index(raw, ";"); semi >= 0 {
		raw = raw[:semi]
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(raw, "[]")
}

func randomCallID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(data[:])
}

func cloneCall(call *CallRecord) *CallRecord {
	if call == nil {
		return nil
	}
	copy := *call
	return &copy
}

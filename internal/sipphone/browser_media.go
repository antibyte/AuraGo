package sipphone

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/voice"

	"github.com/emiago/diago/audio"
	"github.com/pion/ice/v4"
	"github.com/pion/interceptor"
	"github.com/pion/logging"
	"github.com/pion/webrtc/v4"
	pionmedia "github.com/pion/webrtc/v4/pkg/media"
)

const (
	browserMediaSessionTTL = 30 * time.Second
	browserDisconnectGrace = 10 * time.Second
	browserPCMFrameSamples = 160
)

var (
	ErrBrowserMediaDisabled  = errors.New("SIP browser media is disabled")
	ErrBrowserSessionInvalid = errors.New("SIP browser media session is invalid")
	ErrBrowserSessionOwner   = errors.New("SIP browser media session belongs to another client")
)

// BrowserMediaSession is the one-shot SDP answer returned to an authenticated
// browser. The session remains memory-only and must be claimed by one SIP call.
type BrowserMediaSession struct {
	ID        string    `json:"session_id"`
	AnswerSDP string    `json:"answer_sdp"`
	ExpiresAt time.Time `json:"expires_at"`
}

// BrowserMediaService owns one ICE UDP multiplexer and at most one WebRTC peer.
// It deliberately has no SIP credentials and only exchanges 8 kHz PCMU audio
// with the existing voice.DuplexAudio boundary.
type BrowserMediaService struct {
	mu              sync.Mutex
	enabled         bool
	creating        bool
	api             *webrtc.API
	mux             ice.UDPMux
	packetConn      net.PacketConn
	session         *browserMediaPeer
	sessionTTL      time.Duration
	disconnectGrace time.Duration
	onPeerFailed    func(string)
	bindHost        string
	udpPort         int
	advertisedIP    string
}

// NewBrowserMediaService creates the optional browser media boundary.
func NewBrowserMediaService(cfg config.SIPConfig, onPeerFailed func(string)) (*BrowserMediaService, error) {
	service := &BrowserMediaService{
		enabled:         cfg.Enabled && cfg.BrowserMedia.Enabled,
		sessionTTL:      browserMediaSessionTTL,
		disconnectGrace: browserDisconnectGrace,
		onPeerFailed:    onPeerFailed,
		udpPort:         cfg.BrowserMedia.UDPPort,
		advertisedIP:    strings.TrimSpace(cfg.BrowserMedia.AdvertisedIP),
	}
	if !service.enabled {
		return service, nil
	}

	bindHost := cfg.BrowserMedia.BindHost
	if bindHost == "" {
		bindHost = cfg.BindHost
	}
	service.bindHost = strings.TrimSpace(bindHost)
	bindIP := net.ParseIP(bindHost)
	if bindIP == nil {
		return nil, fmt.Errorf("create SIP browser media listener: bind host is not a concrete IP address")
	}
	network := "udp6"
	if bindIP.To4() != nil {
		network = "udp4"
	}
	packetConn, err := net.ListenPacket(network, net.JoinHostPort(bindHost, fmt.Sprintf("%d", cfg.BrowserMedia.UDPPort)))
	if err != nil {
		return nil, fmt.Errorf("listen for SIP browser media: %w", err)
	}

	loggerFactory := logging.NewDefaultLoggerFactory()
	loggerFactory.DefaultLogLevel = logging.LogLevelDisabled
	udpMux := webrtc.NewICEUDPMux(loggerFactory.NewLogger("sip-browser-media"), packetConn)
	settingEngine := webrtc.SettingEngine{}
	settingEngine.SetICEUDPMux(udpMux)
	settingEngine.SetIncludeLoopbackCandidate(true)
	if cfg.BrowserMedia.AdvertisedIP != "" {
		settingEngine.SetNAT1To1IPs([]string{cfg.BrowserMedia.AdvertisedIP}, webrtc.ICECandidateTypeHost)
	}

	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypePCMU,
			ClockRate: 8000,
			Channels:  1,
		},
		PayloadType: 0,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		_ = udpMux.Close()
		return nil, fmt.Errorf("register SIP browser PCMU codec: %w", err)
	}
	registry := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(mediaEngine, registry); err != nil {
		_ = udpMux.Close()
		return nil, fmt.Errorf("register SIP browser media interceptors: %w", err)
	}
	service.api = webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithInterceptorRegistry(registry),
		webrtc.WithSettingEngine(settingEngine),
	)
	service.mux = udpMux
	service.packetConn = packetConn
	return service, nil
}

// Enabled reports whether browser media was enabled and initialized.
func (s *BrowserMediaService) Enabled() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled && s.api != nil
}

// MatchesConfig reports whether the running UDP listener still represents the
// persisted browser-media configuration. Changes require a process restart.
func (s *BrowserMediaService) MatchesConfig(cfg config.SIPConfig) bool {
	if s == nil {
		return false
	}
	desiredEnabled := cfg.Enabled && cfg.BrowserMedia.Enabled
	bindHost := strings.TrimSpace(cfg.BrowserMedia.BindHost)
	if bindHost == "" {
		bindHost = strings.TrimSpace(cfg.BindHost)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !desiredEnabled {
		return !s.enabled
	}
	return s.enabled &&
		s.api != nil &&
		s.bindHost == bindHost &&
		s.udpPort == cfg.BrowserMedia.UDPPort &&
		s.advertisedIP == strings.TrimSpace(cfg.BrowserMedia.AdvertisedIP)
}

// CreateSession accepts a bounded SDP offer and returns a non-trickle answer.
func (s *BrowserMediaService) CreateSession(ctx context.Context, owner, clientID, offerSDP string) (BrowserMediaSession, error) {
	owner = strings.TrimSpace(owner)
	clientID = strings.TrimSpace(clientID)
	if owner == "" || clientID == "" || offerSDP == "" {
		return BrowserMediaSession{}, ErrBrowserSessionInvalid
	}

	s.mu.Lock()
	if !s.enabled || s.api == nil {
		s.mu.Unlock()
		return BrowserMediaSession{}, ErrBrowserMediaDisabled
	}
	if s.session != nil || s.creating {
		s.mu.Unlock()
		return BrowserMediaSession{}, ErrBusy
	}
	s.creating = true
	api := s.api
	sessionTTL := s.sessionTTL
	disconnectGrace := s.disconnectGrace
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.creating = false
		s.mu.Unlock()
	}()

	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return BrowserMediaSession{}, fmt.Errorf("create browser media peer: %w", err)
	}
	localTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMU, ClockRate: 8000, Channels: 1},
		"sip-audio",
		"aurago",
	)
	if err != nil {
		_ = peerConnection.Close()
		return BrowserMediaSession{}, fmt.Errorf("create browser media audio track: %w", err)
	}
	remoteTrack := make(chan *webrtc.TrackRemote, 1)
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		if !strings.EqualFold(track.Codec().MimeType, webrtc.MimeTypePCMU) || track.Codec().ClockRate != 8000 {
			return
		}
		select {
		case remoteTrack <- track:
		default:
		}
	})
	if err := peerConnection.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offerSDP,
	}); err != nil {
		_ = peerConnection.Close()
		return BrowserMediaSession{}, fmt.Errorf("apply browser media offer: %w", err)
	}
	sender, err := peerConnection.AddTrack(localTrack)
	if err != nil {
		_ = peerConnection.Close()
		return BrowserMediaSession{}, fmt.Errorf("add browser media audio track: %w", err)
	}
	go drainBrowserRTCP(sender)

	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		_ = peerConnection.Close()
		return BrowserMediaSession{}, fmt.Errorf("create browser media answer: %w", err)
	}
	if err := peerConnection.SetLocalDescription(answer); err != nil {
		_ = peerConnection.Close()
		return BrowserMediaSession{}, fmt.Errorf("apply browser media answer: %w", err)
	}
	select {
	case <-gatherComplete:
	case <-ctx.Done():
		_ = peerConnection.Close()
		return BrowserMediaSession{}, fmt.Errorf("gather browser media candidates: %w", ctx.Err())
	}
	localDescription := peerConnection.LocalDescription()
	if localDescription == nil || strings.TrimSpace(localDescription.SDP) == "" {
		_ = peerConnection.Close()
		return BrowserMediaSession{}, fmt.Errorf("browser media answer is unavailable")
	}

	id, err := randomBrowserSessionID()
	if err != nil {
		_ = peerConnection.Close()
		return BrowserMediaSession{}, err
	}
	expiresAt := time.Now().UTC().Add(sessionTTL)
	peer := &browserMediaPeer{
		id:              id,
		owner:           owner,
		clientID:        clientID,
		expiresAt:       expiresAt,
		pc:              peerConnection,
		localTrack:      localTrack,
		remoteTrack:     remoteTrack,
		done:            make(chan struct{}),
		onClosed:        s.peerClosed,
		disconnectGrace: disconnectGrace,
	}
	peerConnection.OnConnectionStateChange(peer.connectionStateChanged)
	go peer.receiveBrowserAudio()

	s.mu.Lock()
	if !s.enabled || s.api != api {
		s.mu.Unlock()
		peer.close(true)
		return BrowserMediaSession{}, ErrBrowserMediaDisabled
	}
	if s.session != nil {
		s.mu.Unlock()
		peer.close(true)
		return BrowserMediaSession{}, ErrBusy
	}
	peer.mu.Lock()
	if peer.closed {
		peer.mu.Unlock()
		s.mu.Unlock()
		return BrowserMediaSession{}, ErrBrowserSessionInvalid
	}
	s.session = peer
	peer.expiryTimer = time.AfterFunc(sessionTTL, func() {
		s.expireSession(peer)
	})
	peer.mu.Unlock()
	s.mu.Unlock()

	return BrowserMediaSession{
		ID:        id,
		AnswerSDP: localDescription.SDP,
		ExpiresAt: expiresAt,
	}, nil
}

// ClaimSession binds the authenticated offer to exactly one SIP call.
func (s *BrowserMediaService) ClaimSession(owner, clientID, sessionID string) (MediaPeer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.enabled || s.api == nil {
		return nil, ErrBrowserMediaDisabled
	}
	peer := s.session
	if peer == nil {
		return nil, ErrBrowserSessionInvalid
	}
	peer.mu.Lock()
	defer peer.mu.Unlock()
	if peer.closed || peer.id != strings.TrimSpace(sessionID) || time.Now().After(peer.expiresAt) {
		return nil, ErrBrowserSessionInvalid
	}
	if peer.owner != strings.TrimSpace(owner) || peer.clientID != strings.TrimSpace(clientID) {
		return nil, ErrBrowserSessionOwner
	}
	if peer.claimed {
		return nil, ErrBusy
	}
	peer.claimed = true
	if peer.expiryTimer != nil {
		peer.expiryTimer.Stop()
	}
	return peer, nil
}

// DeleteSession closes a session owned by the current authenticated tab.
func (s *BrowserMediaService) DeleteSession(owner, clientID, sessionID string) error {
	s.mu.Lock()
	if !s.enabled || s.api == nil {
		s.mu.Unlock()
		return ErrBrowserMediaDisabled
	}
	peer := s.session
	if peer == nil {
		s.mu.Unlock()
		return ErrBrowserSessionInvalid
	}
	peer.mu.Lock()
	if peer.closed || peer.id != strings.TrimSpace(sessionID) {
		peer.mu.Unlock()
		s.mu.Unlock()
		return ErrBrowserSessionInvalid
	}
	if peer.owner != strings.TrimSpace(owner) || peer.clientID != strings.TrimSpace(clientID) {
		peer.mu.Unlock()
		s.mu.Unlock()
		return ErrBrowserSessionOwner
	}
	s.session = nil
	claimed := peer.claimed
	peer.mu.Unlock()
	s.mu.Unlock()
	peer.close(!claimed)
	return nil
}

// Close releases the browser peer and the single UDP listener.
func (s *BrowserMediaService) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	peer := s.session
	s.session = nil
	mux := s.mux
	s.mux = nil
	packetConn := s.packetConn
	s.packetConn = nil
	s.api = nil
	s.enabled = false
	s.mu.Unlock()
	if peer != nil {
		peer.close(true)
	}
	if mux != nil {
		return mux.Close()
	}
	if packetConn != nil {
		return packetConn.Close()
	}
	return nil
}

func (s *BrowserMediaService) expireSession(peer *browserMediaPeer) {
	s.mu.Lock()
	if s.session != peer {
		s.mu.Unlock()
		return
	}
	peer.mu.Lock()
	if peer.closed || peer.claimed {
		peer.mu.Unlock()
		s.mu.Unlock()
		return
	}
	s.session = nil
	peer.mu.Unlock()
	s.mu.Unlock()
	peer.close(true)
}

func (s *BrowserMediaService) peerClosed(peer *browserMediaPeer, unexpected bool, callID string) {
	s.mu.Lock()
	if s.session == peer {
		s.session = nil
	}
	callback := s.onPeerFailed
	s.mu.Unlock()
	if unexpected && callID != "" && callback != nil {
		callback(callID)
	}
}

type browserMediaPeer struct {
	mu              sync.Mutex
	id              string
	owner           string
	clientID        string
	expiresAt       time.Time
	claimed         bool
	closed          bool
	callID          string
	pc              *webrtc.PeerConnection
	localTrack      *webrtc.TrackLocalStaticSample
	remoteTrack     chan *webrtc.TrackRemote
	done            chan struct{}
	ctx             context.Context
	cancel          context.CancelFunc
	media           voice.DuplexAudio
	expiryTimer     *time.Timer
	disconnectTimer *time.Timer
	disconnectGrace time.Duration
	onClosed        func(*browserMediaPeer, bool, string)
	closeOnce       sync.Once
}

func (p *browserMediaPeer) Attach(ctx context.Context, callID string, media voice.DuplexAudio) error {
	if p == nil || media == nil || strings.TrimSpace(callID) == "" {
		return ErrBrowserSessionInvalid
	}
	p.mu.Lock()
	if p.callID != "" {
		p.mu.Unlock()
		return ErrBusy
	}
	if p.pc == nil || p.pc.ConnectionState() == webrtc.PeerConnectionStateClosed || p.pc.ConnectionState() == webrtc.PeerConnectionStateFailed {
		p.mu.Unlock()
		return ErrBrowserSessionInvalid
	}
	peerCtx, cancel := context.WithCancel(ctx)
	p.callID = callID
	p.ctx = peerCtx
	p.cancel = cancel
	p.media = media
	p.mu.Unlock()

	go p.sendSIPAudioToBrowser(peerCtx, media)
	return nil
}

func (p *browserMediaPeer) Detach(callID string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	if p.callID != "" && p.callID != callID {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()
	p.close(true)
}

func (p *browserMediaPeer) sendSIPAudioToBrowser(ctx context.Context, media voice.DuplexAudio) {
	pending := make([]int16, 0, browserPCMFrameSamples*2)
	for {
		var frame voice.PCMFrame
		select {
		case <-ctx.Done():
			return
		case frame = <-media.Receive():
		}
		if frame.SampleRate != 8000 {
			p.close(false)
			return
		}
		pending = append(pending, frame.Samples...)
		for len(pending) >= browserPCMFrameSamples {
			linear := make([]byte, browserPCMFrameSamples*2)
			for i, sample := range pending[:browserPCMFrameSamples] {
				binary.LittleEndian.PutUint16(linear[i*2:i*2+2], uint16(sample))
			}
			encoded := make([]byte, browserPCMFrameSamples)
			if _, err := audio.EncodeUlawTo(encoded, linear); err != nil {
				p.close(false)
				return
			}
			if err := p.localTrack.WriteSample(pionmedia.Sample{Data: encoded, Duration: 20 * time.Millisecond}); err != nil {
				if ctx.Err() == nil {
					p.close(false)
				}
				return
			}
			pending = pending[browserPCMFrameSamples:]
		}
	}
}

func (p *browserMediaPeer) receiveBrowserAudio() {
	var track *webrtc.TrackRemote
	select {
	case track = <-p.remoteTrack:
	case <-p.done:
		return
	}
	for {
		packet, _, err := track.ReadRTP()
		if err != nil {
			p.mu.Lock()
			closed := p.closed
			p.mu.Unlock()
			if !closed && !errors.Is(err, io.EOF) {
				p.close(false)
			}
			return
		}
		if len(packet.Payload) == 0 {
			continue
		}
		p.mu.Lock()
		ctx := p.ctx
		media := p.media
		closed := p.closed
		p.mu.Unlock()
		if closed {
			return
		}
		// RTP is drained as soon as the peer exists. Until Attach registers the
		// active SIP bridge, microphone packets are intentionally discarded.
		if ctx == nil || media == nil {
			continue
		}
		linear := make([]byte, len(packet.Payload)*2)
		decoded, err := audio.DecodeUlawTo(linear, packet.Payload)
		if err != nil {
			p.close(false)
			return
		}
		samples := make([]int16, decoded/2)
		for i := range samples {
			samples[i] = int16(binary.LittleEndian.Uint16(linear[i*2 : i*2+2]))
		}
		if err := media.Send(ctx, voice.PCMFrame{Samples: samples, SampleRate: 8000}); err != nil {
			return
		}
	}
}

func (p *browserMediaPeer) connectionStateChanged(state webrtc.PeerConnectionState) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	switch state {
	case webrtc.PeerConnectionStateConnected:
		if p.disconnectTimer != nil {
			p.disconnectTimer.Stop()
			p.disconnectTimer = nil
		}
		p.mu.Unlock()
	case webrtc.PeerConnectionStateDisconnected:
		if p.disconnectTimer == nil {
			p.disconnectTimer = time.AfterFunc(p.disconnectGrace, func() {
				p.close(false)
			})
		}
		p.mu.Unlock()
	case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
		p.mu.Unlock()
		p.close(false)
	default:
		p.mu.Unlock()
	}
}

func (p *browserMediaPeer) close(expected bool) {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		cancel := p.cancel
		pc := p.pc
		callID := p.callID
		timer := p.disconnectTimer
		p.disconnectTimer = nil
		expiry := p.expiryTimer
		p.expiryTimer = nil
		onClosed := p.onClosed
		done := p.done
		p.mu.Unlock()
		if done != nil {
			close(done)
		}
		if timer != nil {
			timer.Stop()
		}
		if expiry != nil {
			expiry.Stop()
		}
		if cancel != nil {
			cancel()
		}
		if pc != nil {
			_ = pc.Close()
		}
		if onClosed != nil {
			onClosed(p, !expected, callID)
		}
	})
}

func drainBrowserRTCP(sender *webrtc.RTPSender) {
	buffer := make([]byte, 1500)
	for {
		if _, _, err := sender.Read(buffer); err != nil {
			return
		}
	}
}

func randomBrowserSessionID() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate browser media session ID: %w", err)
	}
	return hex.EncodeToString(raw), nil
}
